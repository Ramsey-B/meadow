package processor

import (
	"context"
	"encoding/json"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/internal/repositories/mergedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/mergedrelationship"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedrelationship"
	"github.com/Ramsey-B/ivy/pkg/kafka"
	"github.com/Ramsey-B/ivy/pkg/matching"
	"github.com/Ramsey-B/ivy/pkg/merging"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// MergeProcessor processes staged entity CDC events and performs merging
type MergeProcessor struct {
	logger           ectologger.Logger
	entityRepo       *stagedentity.Repository
	relationshipRepo *stagedrelationship.Repository
	mergedRepo       *mergedentity.Repository
	mergedRelRepo    *mergedrelationship.Repository
	matchingService  *matching.Service
	mergeEngine      *merging.Engine
}

// NewMergeProcessor creates a new merge processor
func NewMergeProcessor(
	logger ectologger.Logger,
	entityRepo *stagedentity.Repository,
	relationshipRepo *stagedrelationship.Repository,
	mergedRepo *mergedentity.Repository,
	mergedRelRepo *mergedrelationship.Repository,
	matchingService *matching.Service,
	mergeEngine *merging.Engine,
) *MergeProcessor {
	return &MergeProcessor{
		logger:           logger,
		entityRepo:       entityRepo,
		relationshipRepo: relationshipRepo,
		mergedRepo:       mergedRepo,
		mergedRelRepo:    mergedRelRepo,
		matchingService:  matchingService,
		mergeEngine:      mergeEngine,
	}
}

// ProcessMessage processes a Debezium CDC event for staged entities
func (p *MergeProcessor) ProcessMessage(ctx context.Context, msg *kafka.IncomingMessage) error {
	ctx, span := tracing.StartSpan(ctx, "MergeProcessor.ProcessMessage")
	defer span.End()

	// Parse the Debezium envelope
	envelope, err := kafka.ParseDebeziumMessage(msg.Value)
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to parse Debezium message")
		return err
	}

	// Skip deletes - we don't need to merge deleted entities
	if envelope.Payload.IsDelete() {
		p.logger.WithContext(ctx).WithFields(map[string]any{
			"entity_type": envelope.Payload.Source.Table,
			"tenant_id":   envelope.Payload.Source.Db,
			"id":          envelope.Payload.Source.TxId,
		}).Debug("Skipping delete event")
		return nil
	}

	// Parse the staged entity row
	row, err := envelope.Payload.ParseStagedEntityRow()
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to parse staged entity row")
		return err
	}
	if row == nil {
		return nil
	}

	// Skip if soft-deleted
	if row.IsDeleted() {
		log := p.logger.WithContext(ctx).WithFields(map[string]any{
			"entity_id":   row.ID,
			"entity_type": row.EntityType,
			"tenant_id":   row.TenantID,
		})
		log.Debug("Processing soft-deleted staged entity (cascade cleanup)")
		_ = p.handleDeletedStagedEntity(ctx, row, log)
		return nil
	}

	// if the fingerprint is the same as the previous fingerprint, skip
	if row.Fingerprint == row.PreviousFingerprint {
		p.logger.WithContext(ctx).WithFields(map[string]any{
			"entity_id":   row.ID,
			"entity_type": row.EntityType,
			"tenant_id":   row.TenantID,
		}).Debug("Skipping entity with same fingerprint")
		return nil
	}

	log := p.logger.WithContext(ctx).WithFields(map[string]any{
		"entity_id":   row.ID,
		"entity_type": row.EntityType,
		"tenant_id":   row.TenantID,
		"op":          envelope.Payload.Op,
	})

	log.Debug("Processing staged entity CDC event")

	entity := row.ToStagedEntity()
	if entity == nil {
		log.Error("Failed to convert row to staged entity")
		return nil
	}

	// Sync entity matching fields
	if err := p.matchingService.IndexEntity(ctx, entity); err != nil {
		log.WithError(err).WithFields(map[string]any{
			"entity_id":   entity.ID,
			"entity_type": entity.EntityType,
			"tenant_id":   entity.TenantID,
		}).Error("Failed to index entity")
		return err
	}

	// Get matches for the entity
	matchResults, err := p.matchingService.FindMatches(ctx, entity.TenantID, entity)
	if err != nil {
		log.WithError(err).WithFields(map[string]any{
			"entity_id":   entity.ID,
			"entity_type": entity.EntityType,
			"tenant_id":   entity.TenantID,
		}).Error("Failed to find matches")
		return err
	}

	// Merge the entity with its matches (or create standalone if no matches)
	result, err := p.mergeEngine.MergeWithMatches(ctx, entity, matchResults)
	if err != nil {
		log.WithError(err).WithFields(map[string]any{
			"entity_id":   entity.ID,
			"entity_type": entity.EntityType,
			"tenant_id":   entity.TenantID,
			"match_count": len(matchResults.Matches),
		}).Error("Failed to merge entity")
		return err
	}

	log.WithFields(map[string]any{
		"entity_id":    entity.ID,
		"merged_id":    result.MergedEntity.ID,
		"is_new":       result.IsNew,
		"source_count": len(result.SourceEntities),
		"conflicts":    len(result.Conflicts),
	}).Info("Entity merged successfully")

	// Process relationship rules that match this entity
	if err := p.processMatchingRelationshipRules(ctx, entity, result.MergedEntity, log); err != nil {
		log.WithError(err).Warn("Failed to process relationship rules")
		// Don't fail the entire operation
	}

	return nil
}

// handleDeletedStagedEntity removes a deleted staged entity from its cluster and cascades deletion to the merged entity
// (and its merged relationships) if no sources remain.
func (p *MergeProcessor) handleDeletedStagedEntity(ctx context.Context, row *kafka.StagedEntityRow, log ectologger.Logger) error {
	// Find the merged entity this staged entity belongs to
	mergedEntities, err := p.mergedRepo.GetMergedEntitiesByStagedIDs(ctx, row.TenantID, []string{row.ID})
	if err != nil {
		return err
	}
	if len(mergedEntities) == 0 {
		return nil
	}
	mergedEntity := mergedEntities[0]

	// Remove from cluster
	if err := p.mergedRepo.RemoveFromCluster(ctx, row.TenantID, mergedEntity.ID, row.ID); err != nil {
		return err
	}

	// Check if any active cluster members remain
	members, err := p.mergedRepo.GetClusterMembers(ctx, row.TenantID, mergedEntity.ID)
	if err != nil {
		return err
	}
	if len(members) > 0 {
		return nil
	}

	// No sources left: soft delete merged entity
	if err := p.mergedRepo.SoftDelete(ctx, row.TenantID, mergedEntity.ID); err != nil {
		return err
	}

	// Cascade: soft-delete merged relationships touching this merged entity
	if p.mergedRelRepo != nil {
		_, _ = p.mergedRelRepo.SoftDeleteByMergedEntityID(ctx, row.TenantID, mergedEntity.ID)
	}

	log.WithFields(map[string]any{
		"merged_entity_id": mergedEntity.ID,
	}).Info("Soft deleted merged entity (all sources removed)")

	return nil
}

// processMatchingRelationshipRules handles incomplete relationships that are waiting for this entity.
// This only processes source_id-based lookups (not field-based) since field-based rules are
// evaluated when the relationship rule is created in the relationship CDC consumer.
//
// Performance: This is much faster than evaluating all rules because we only check relationships
// that are explicitly waiting for this entity (incomplete relationships with NULL entity IDs).
func (p *MergeProcessor) processMatchingRelationshipRules(ctx context.Context, stagedEntity *models.StagedEntity, mergedEntity *models.MergedEntity, log ectologger.Logger) error {
	ctx, span := tracing.StartSpan(ctx, "MergeProcessor.processMatchingRelationshipRules")
	defer span.End()

	// Find incomplete relationships where this entity is the "from" side (from_staged_entity_id IS NULL)
	fromRels, err := p.relationshipRepo.GetIncompleteByFromSource(ctx, stagedEntity.TenantID, stagedEntity.EntityType, stagedEntity.SourceID, stagedEntity.Integration)
	if err != nil {
		return err
	}

	// Find incomplete relationships where this entity is the "to" side (to_staged_entity_id IS NULL)
	toRels, err := p.relationshipRepo.GetIncompleteByToSource(ctx, stagedEntity.TenantID, stagedEntity.EntityType, stagedEntity.SourceID, stagedEntity.Integration)
	if err != nil {
		return err
	}

	if len(fromRels) == 0 && len(toRels) == 0 {
		return nil
	}

	log.WithFields(map[string]any{
		"incomplete_from": len(fromRels),
		"incomplete_to":   len(toRels),
	}).Info("Found incomplete relationships waiting for this entity")

	createdCount := 0

	// Complete relationships where this entity is the "from" side
	for _, rule := range fromRels {
		count, err := p.completeRelationshipAsFrom(ctx, &rule, mergedEntity, log)
		if err != nil {
			log.WithError(err).WithFields(map[string]any{"rule_id": rule.ID}).Warn("Failed to complete relationship as from")
			continue
		}
		createdCount += count
	}

	// Complete relationships where this entity is the "to" side
	for _, rule := range toRels {
		count, err := p.completeRelationshipAsTo(ctx, &rule, stagedEntity, mergedEntity, log)
		if err != nil {
			log.WithError(err).WithFields(map[string]any{"rule_id": rule.ID}).Warn("Failed to complete relationship as to")
			continue
		}
		createdCount += count
	}

	if createdCount > 0 {
		log.WithFields(map[string]any{
			"created_count": createdCount,
		}).Info("Completed incomplete relationships")
	}

	return nil
}

// completeRelationshipAsFrom completes an incomplete relationship where this entity is the "from" side
func (p *MergeProcessor) completeRelationshipAsFrom(ctx context.Context, rule *models.StagedRelationship, fromMerged *models.MergedEntity, log ectologger.Logger) (int, error) {
	// The "to" side should already be resolved (otherwise this relationship wouldn't be in the incomplete list)
	// Use efficient bulk query to get the merged entity for the "to" side
	toMappings, err := p.mergedRepo.GetMergedEntityIDsForRuleCriteria(ctx, rule.TenantID, rule.ToEntityType, rule.ToSourceID, rule.ToIntegration)
	if err != nil {
		return 0, err
	}

	if len(toMappings) == 0 {
		log.Debug("To entity not yet merged, relationship still incomplete")
		return 0, nil
	}

	createdCount := 0
	for _, mapping := range toMappings {
		// Create the merged relationship
		if err := p.createMergedRelationshipByID(ctx, rule, fromMerged.ID, mapping.MergedEntityID, log); err != nil {
			log.WithError(err).Warn("Failed to create merged relationship")
			continue
		}
		createdCount++
	}

	return createdCount, nil
}

// completeRelationshipAsTo completes an incomplete relationship where this entity is the "to" side
func (p *MergeProcessor) completeRelationshipAsTo(ctx context.Context, rule *models.StagedRelationship, toStaged *models.StagedEntity, toMerged *models.MergedEntity, log ectologger.Logger) (int, error) {
	// The "from" side should already be resolved (otherwise this relationship wouldn't be in the incomplete list)
	// Use efficient bulk query to get the merged entity for the "from" side
	fromMappings, err := p.mergedRepo.GetMergedEntityIDsForRuleCriteria(ctx, rule.TenantID, rule.FromEntityType, rule.FromSourceID, rule.FromIntegration)
	if err != nil {
		return 0, err
	}

	if len(fromMappings) == 0 {
		log.Debug("From entity not yet merged, relationship still incomplete")
		return 0, nil
	}

	createdCount := 0
	for _, mapping := range fromMappings {
		// Create the merged relationship
		if err := p.createMergedRelationshipByID(ctx, rule, mapping.MergedEntityID, toMerged.ID, log); err != nil {
			log.WithError(err).Warn("Failed to create merged relationship")
			continue
		}
		createdCount++
	}

	return createdCount, nil
}

// createMergedRelationshipByID creates a merged relationship from a rule using merged entity IDs
func (p *MergeProcessor) createMergedRelationshipByID(ctx context.Context, rule *models.StagedRelationship, fromMergedID, toMergedID string, log ectologger.Logger) error {
	var relDataBytes json.RawMessage
	if len(rule.Data) > 0 {
		relDataBytes = rule.Data
	}

	mergedRel, err := p.mergedRelRepo.Upsert(ctx, rule.TenantID, models.CreateMergedRelationshipRequest{
		RelationshipType:   rule.RelationshipType,
		FromEntityType:     rule.FromEntityType,
		FromMergedEntityID: fromMergedID,
		ToEntityType:       rule.ToEntityType,
		ToMergedEntityID:   toMergedID,
		Data:               relDataBytes,
	})
	if err != nil {
		return err
	}

	log.WithFields(map[string]any{
		"merged_relationship_id": mergedRel.ID,
		"from_merged_id":         fromMergedID,
		"to_merged_id":           toMergedID,
		"rule_id":                rule.ID,
	}).Debug("Created merged relationship from rule")

	return nil
}
