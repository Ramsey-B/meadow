package processor

import (
	"context"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/internal/repositories/mergedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/mergedrelationship"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedrelationship"
	"github.com/Ramsey-B/ivy/pkg/kafka"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// DeletionProcessor processes CDC events for deleted staged entities and relationships
// and cascades deletions to merged entities/relationships when all sources are gone
//
// This processor watches for soft-deleted staged entities/relationships (deleted_at set) and:
// 1. Removes them from their cluster
// 2. If no more sources remain in the cluster, soft-deletes the merged entity/relationship
type DeletionProcessor struct {
	logger           ectologger.Logger
	stagedEntityRepo *stagedentity.Repository
	stagedRelRepo    *stagedrelationship.Repository
	mergedEntityRepo *mergedentity.Repository
	mergedRelRepo    *mergedrelationship.Repository
}

// NewDeletionProcessor creates a new deletion processor
func NewDeletionProcessor(
	logger ectologger.Logger,
	stagedEntityRepo *stagedentity.Repository,
	stagedRelRepo *stagedrelationship.Repository,
	mergedEntityRepo *mergedentity.Repository,
	mergedRelRepo *mergedrelationship.Repository,
) *DeletionProcessor {
	return &DeletionProcessor{
		logger:           logger,
		stagedEntityRepo: stagedEntityRepo,
		stagedRelRepo:    stagedRelRepo,
		mergedEntityRepo: mergedEntityRepo,
		mergedRelRepo:    mergedRelRepo,
	}
}

// ProcessMessage processes a Debezium CDC event for deleted staged entities and relationships
func (p *DeletionProcessor) ProcessMessage(ctx context.Context, msg *kafka.IncomingMessage) error {
	ctx, span := tracing.StartSpan(ctx, "DeletionProcessor.ProcessMessage")
	defer span.End()

	// Parse the Debezium envelope
	envelope, err := kafka.ParseDebeziumMessage(msg.Value)
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to parse Debezium message")
		return err
	}

	// We only care about UPDATE events where deleted_at was set
	// (soft deletes appear as UPDATEs in Debezium)
	if !envelope.Payload.IsUpdate() {
		return nil
	}

	// Check if this is a staged entity or relationship table
	table := envelope.Payload.Source.Table

	switch table {
	case "staged_entities":
		return p.processStagedEntityDeletion(ctx, envelope)
	case "staged_relationships":
		return p.processStagedRelationshipDeletion(ctx, envelope)
	}

	return nil
}

// processStagedEntityDeletion handles when a staged entity is soft-deleted
func (p *DeletionProcessor) processStagedEntityDeletion(ctx context.Context, envelope *kafka.DebeziumEnvelope) error {
	ctx, span := tracing.StartSpan(ctx, "DeletionProcessor.processStagedEntityDeletion")
	defer span.End()

	// Parse the staged entity row
	row, err := envelope.Payload.ParseStagedEntityRow()
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to parse staged entity row")
		return err
	}
	if row == nil {
		return nil
	}

	// Only process if this entity is now deleted
	if !row.IsDeleted() {
		return nil
	}

	log := p.logger.WithContext(ctx).WithFields(map[string]any{
		"staged_entity_id": row.ID,
		"entity_type":      row.EntityType,
		"tenant_id":        row.TenantID,
	})

	log.Debug("Processing deleted staged entity")

	// Find the merged entity this staged entity belongs to
	mergedEntities, err := p.mergedEntityRepo.GetMergedEntitiesByStagedIDs(ctx, row.TenantID, []string{row.ID})
	if err != nil {
		log.WithError(err).Error("Failed to get merged entity")
		return err
	}

	if len(mergedEntities) == 0 {
		log.Debug("No merged entity found for deleted staged entity")
		return nil
	}

	mergedEntity := mergedEntities[0]

	// Remove from cluster
	if err := p.mergedEntityRepo.RemoveFromCluster(ctx, row.TenantID, mergedEntity.ID, row.ID); err != nil {
		log.WithError(err).Error("Failed to remove from cluster")
		return err
	}

	log.WithFields(map[string]any{
		"merged_entity_id": mergedEntity.ID,
	}).Info("Removed staged entity from cluster")

	// Check if any active cluster members remain
	members, err := p.mergedEntityRepo.GetClusterMembers(ctx, row.TenantID, mergedEntity.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get cluster members")
		return err
	}

	if len(members) == 0 {
		// No more sources - soft delete the merged entity
		if err := p.mergedEntityRepo.SoftDelete(ctx, row.TenantID, mergedEntity.ID); err != nil {
			log.WithError(err).Error("Failed to soft delete merged entity")
			return err
		}

		// Cascade: soft-delete any merged relationships touching this merged entity.
		// Without this, merged_relationships can keep pointing at deleted merged entities.
		if p.mergedRelRepo != nil {
			if _, err := p.mergedRelRepo.SoftDeleteByMergedEntityID(ctx, row.TenantID, mergedEntity.ID); err != nil {
				log.WithError(err).Error("Failed to cascade delete merged relationships for merged entity")
				return err
			}
		}

		log.WithFields(map[string]any{
			"merged_entity_id": mergedEntity.ID,
		}).Info("Soft deleted merged entity (all sources removed)")
	}

	return nil
}

// processStagedRelationshipDeletion handles when a staged relationship is soft-deleted
func (p *DeletionProcessor) processStagedRelationshipDeletion(ctx context.Context, envelope *kafka.DebeziumEnvelope) error {
	ctx, span := tracing.StartSpan(ctx, "DeletionProcessor.processStagedRelationshipDeletion")
	defer span.End()

	// Parse the staged relationship row
	row, err := envelope.Payload.ParseStagedRelationshipRow()
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to parse staged relationship row")
		return err
	}
	if row == nil {
		return nil
	}

	// Only process if this relationship is now deleted
	if !row.IsDeleted() {
		return nil
	}

	log := p.logger.WithContext(ctx).WithFields(map[string]any{
		"staged_relationship_id": row.ID,
		"relationship_type":      row.RelationshipType,
		"tenant_id":              row.TenantID,
	})

	log.Debug("Processing deleted staged relationship")

	// Find the merged relationship this staged relationship belongs to
	mergedRelationships, err := p.mergedRelRepo.GetMergedRelationshipsByStagedIDs(ctx, row.TenantID, []string{row.ID})
	if err != nil {
		log.WithError(err).Error("Failed to get merged relationship")
		return err
	}

	if len(mergedRelationships) == 0 {
		log.Debug("No merged relationship found for deleted staged relationship")
		return nil
	}

	mergedRel := mergedRelationships[0]

	// Remove from cluster
	if err := p.mergedRelRepo.RemoveFromCluster(ctx, row.TenantID, mergedRel.ID, row.ID); err != nil {
		log.WithError(err).Error("Failed to remove from cluster")
		return err
	}

	log.WithFields(map[string]any{
		"merged_relationship_id": mergedRel.ID,
	}).Info("Removed staged relationship from cluster")

	// Check if any active cluster members remain
	members, err := p.mergedRelRepo.GetClusterMembers(ctx, row.TenantID, mergedRel.ID)
	if err != nil {
		log.WithError(err).Error("Failed to get cluster members")
		return err
	}

	if len(members) == 0 {
		// No more sources - soft delete the merged relationship
		if err := p.mergedRelRepo.SoftDelete(ctx, row.TenantID, mergedRel.ID); err != nil {
			log.WithError(err).Error("Failed to soft delete merged relationship")
			return err
		}

		log.WithFields(map[string]any{
			"merged_relationship_id": mergedRel.ID,
		}).Info("Soft deleted merged relationship (all sources removed)")
	}

	return nil
}
