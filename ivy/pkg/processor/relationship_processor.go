package processor

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/internal/repositories/mergedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/mergedrelationship"
	relationshiptyperepo "github.com/Ramsey-B/ivy/internal/repositories/relationshiptype"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedrelationship"
	"github.com/Ramsey-B/ivy/pkg/kafka"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// RelationshipProcessor processes staged relationship CDC events
// Staged relationships act as "rules" that match entities and create merged relationships
type RelationshipProcessor struct {
	logger               ectologger.Logger
	entityRepo           *stagedentity.Repository
	relationshipRepo     *stagedrelationship.Repository
	mergedRepo           *mergedentity.Repository
	mergedRelRepo        *mergedrelationship.Repository
	relationshipTypeRepo *relationshiptyperepo.Repository

	schemaCache sync.Map // key: tenantID|relationshipType -> *models.EntityTypeSchema
}

// NewRelationshipProcessor creates a new relationship processor
func NewRelationshipProcessor(
	logger ectologger.Logger,
	entityRepo *stagedentity.Repository,
	relationshipRepo *stagedrelationship.Repository,
	mergedRepo *mergedentity.Repository,
	mergedRelRepo *mergedrelationship.Repository,
	relationshipTypeRepo *relationshiptyperepo.Repository,
) *RelationshipProcessor {
	return &RelationshipProcessor{
		logger:               logger,
		entityRepo:           entityRepo,
		relationshipRepo:     relationshipRepo,
		mergedRepo:           mergedRepo,
		mergedRelRepo:        mergedRelRepo,
		relationshipTypeRepo: relationshipTypeRepo,
	}
}

// ProcessMessage processes a Debezium CDC event for staged relationships
// Each staged relationship is a rule that can match multiple entity pairs
func (p *RelationshipProcessor) ProcessMessage(ctx context.Context, msg *kafka.IncomingMessage) error {
	ctx, span := tracing.StartSpan(ctx, "RelationshipProcessor.ProcessMessage")
	defer span.End()

	// Parse the Debezium envelope
	envelope, err := kafka.ParseDebeziumMessage(msg.Value)
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to parse Debezium message")
		return err
	}

	// Skip deletes
	if envelope.Payload.IsDelete() {
		p.logger.WithContext(ctx).Debug("Skipping delete event")
		return nil
	}

	// Parse the staged relationship row
	row, err := envelope.Payload.ParseStagedRelationshipRow()
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to parse staged relationship row")
		return err
	}
	if row == nil {
		return nil
	}

	// Skip if soft-deleted
	if row.IsDeleted() {
		p.logger.WithContext(ctx).Debug("Skipping soft-deleted relationship")
		return nil
	}

	log := p.logger.WithContext(ctx).WithFields(map[string]any{
		"relationship_id":   row.ID,
		"relationship_type": row.RelationshipType,
		"tenant_id":         row.TenantID,
		"from_entity_type":  row.FromEntityType,
		"from_source_id":    row.FromSourceID,
		"from_source_field": row.FromSourceField,
		"to_entity_type":    row.ToEntityType,
		"to_source_id":      row.ToSourceID,
		"to_source_field":   row.ToSourceField,
		"op":                envelope.Payload.Op,
	})

	log.Debug("Processing staged relationship CDC event - will match all entity pairs")

	// Find all matching "from" entities using the source field criteria
	fromEntities, err := p.entityRepo.FindBySourceIDAndFields(ctx, row.TenantID, row.FromEntityType, row.FromSourceID, row.FromSourceField)
	if err != nil {
		log.WithError(err).Error("Failed to lookup from entities")
		return err
	}

	// Find all matching "to" entities using the source field criteria
	toEntities, err := p.entityRepo.FindBySourceIDAndFields(ctx, row.TenantID, row.ToEntityType, row.ToSourceID, row.ToSourceField)
	if err != nil {
		log.WithError(err).Error("Failed to lookup to entities")
		return err
	}

	if len(fromEntities) == 0 || len(toEntities) == 0 {
		log.WithFields(map[string]any{
			"from_entities_found": len(fromEntities),
			"to_entities_found":   len(toEntities),
		}).Debug("Waiting for entities to arrive")
		return nil
	}

	log.WithFields(map[string]any{
		"from_entities_found": len(fromEntities),
		"to_entities_found":   len(toEntities),
		"potential_pairs":     len(fromEntities) * len(toEntities),
	}).Info("Found matching entities - creating merged relationships")

	// Create merged relationships for the cartesian product of all matches
	// Use bulk lookups to map staged entities to merged entities
	fromStagedIDs := make([]string, len(fromEntities))
	for i, e := range fromEntities {
		fromStagedIDs[i] = e.ID
	}
	toStagedIDs := make([]string, len(toEntities))
	for i, e := range toEntities {
		toStagedIDs[i] = e.ID
	}

	// Build staged -> merged ID maps by checking each staged entity
	fromStagedToMerged := make(map[string]string)
	for _, stagedID := range fromStagedIDs {
		merged, err := p.mergedRepo.GetMergedEntityByStagedID(ctx, row.TenantID, stagedID)
		if err == nil && merged != nil {
			fromStagedToMerged[stagedID] = merged.ID
		}
	}

	toStagedToMerged := make(map[string]string)
	for _, stagedID := range toStagedIDs {
		merged, err := p.mergedRepo.GetMergedEntityByStagedID(ctx, row.TenantID, stagedID)
		if err == nil && merged != nil {
			toStagedToMerged[stagedID] = merged.ID
		}
	}

	createdCount := 0
	for _, fromEntity := range fromEntities {
		fromMergedID, fromOk := fromStagedToMerged[fromEntity.ID]
		if !fromOk {
			log.WithFields(map[string]any{"from_staged_id": fromEntity.ID}).Debug("From entity not yet merged, skipping")
			continue
		}

		for _, toEntity := range toEntities {
			toMergedID, toOk := toStagedToMerged[toEntity.ID]
			if !toOk {
				log.WithFields(map[string]any{"to_staged_id": toEntity.ID}).Debug("To entity not yet merged, skipping")
				continue
			}

			if err := p.createMergedRelationshipByIDs(ctx, row, fromMergedID, toMergedID, log); err != nil {
				log.WithError(err).WithFields(map[string]any{
					"from_merged_id": fromMergedID,
					"to_merged_id":   toMergedID,
				}).Warn("Failed to create merged relationship for pair")
				continue
			}
			createdCount++
		}
	}

	log.WithFields(map[string]any{
		"created_count": createdCount,
	}).Info("Created merged relationships from rule")

	return nil
}

// createMergedRelationshipByIDs creates a merged relationship using pre-fetched merged entity IDs
func (p *RelationshipProcessor) createMergedRelationshipByIDs(
	ctx context.Context,
	rule *kafka.StagedRelationshipRow,
	fromMergedID, toMergedID string,
	log ectologger.Logger,
) error {
	ctx, span := tracing.StartSpan(ctx, "RelationshipProcessor.createMergedRelationshipByIDs")
	defer span.End()

	// Parse relationship data from the rule
	var relDataBytes json.RawMessage
	if len(rule.Data) > 0 {
		relDataBytes = rule.Data
	}

	// Create or update the merged relationship (upsert handles duplicates)
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
	}).Debug("Created merged relationship for entity pair")

	return nil
}
