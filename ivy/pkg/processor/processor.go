// Package processor handles incoming entity messages and manages the staging pipeline.
// This is the ingestion layer - it writes to staged tables. Merging is handled by
// separate Debezium CDC consumers that react to changes in the staged tables.
package processor

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/Gobusters/ectologger"

	deletionstrategyrepo "github.com/Ramsey-B/ivy/internal/repositories/deletionstrategy"
	relationshiptyperepo "github.com/Ramsey-B/ivy/internal/repositories/relationshiptype"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedrelationship"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedrelationshipcriteria"
	"github.com/Ramsey-B/ivy/pkg/criteria"
	"github.com/Ramsey-B/ivy/pkg/deletion"
	"github.com/Ramsey-B/ivy/pkg/kafka"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/ivy/pkg/schema"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// Processor handles message processing for the staging layer
type Processor struct {
	logger               ectologger.Logger
	entityRepo           *stagedentity.Repository
	relationshipRepo     *stagedrelationship.Repository
	criteriaRepo         *stagedrelationshipcriteria.Repository
	relationshipTypeRepo *relationshiptyperepo.Repository
	deletionStrategyRepo *deletionstrategyrepo.Repository
	deletionEngine       *deletion.Engine
	schemaService        *schema.ValidationService
	criteriaEvaluator    *CriteriaEvaluator

	relationshipSchemaCache sync.Map // key: tenantID|relationshipType -> relationshipSchemaCacheEntry
}

type relationshipSchemaCacheEntry struct {
	schema   *models.EntityTypeSchema
	needsOld bool
}

// NewProcessor creates a new message processor for ingestion.
// It only writes to staged tables - merging is handled by separate Debezium CDC consumers.
func NewProcessor(
	logger ectologger.Logger,
	entityRepo *stagedentity.Repository,
	relationshipRepo *stagedrelationship.Repository,
	criteriaRepo *stagedrelationshipcriteria.Repository,
	relationshipTypeRepo *relationshiptyperepo.Repository,
	deletionStrategyRepo *deletionstrategyrepo.Repository,
	schemaService *schema.ValidationService,
) *Processor {
	deletionEngine := deletion.NewEngine(logger, entityRepo, relationshipRepo)
	criteriaEvaluator := NewCriteriaEvaluator(logger, entityRepo, relationshipRepo, criteriaRepo)
	return &Processor{
		logger:               logger,
		entityRepo:           entityRepo,
		relationshipRepo:     relationshipRepo,
		criteriaRepo:         criteriaRepo,
		relationshipTypeRepo: relationshipTypeRepo,
		deletionStrategyRepo: deletionStrategyRepo,
		deletionEngine:       deletionEngine,
		schemaService:        schemaService,
		criteriaEvaluator:    criteriaEvaluator,
	}
}

// CriteriaEvaluator handles criteria-based relationship evaluation
type CriteriaEvaluator struct {
	logger           ectologger.Logger
	entityRepo       *stagedentity.Repository
	relationshipRepo *stagedrelationship.Repository
	criteriaRepo     *stagedrelationshipcriteria.Repository
}

// NewCriteriaEvaluator creates a new criteria evaluator
func NewCriteriaEvaluator(
	logger ectologger.Logger,
	entityRepo *stagedentity.Repository,
	relationshipRepo *stagedrelationship.Repository,
	criteriaRepo *stagedrelationshipcriteria.Repository,
) *CriteriaEvaluator {
	return &CriteriaEvaluator{
		logger:           logger,
		entityRepo:       entityRepo,
		relationshipRepo: relationshipRepo,
		criteriaRepo:     criteriaRepo,
	}
}

// EvaluateCriteriaForNewEntity checks if a newly created/updated entity matches any criteria definitions
// and creates staged_relationships + matches accordingly. This is the backfill path.
func (e *CriteriaEvaluator) EvaluateCriteriaForNewEntity(ctx context.Context, entity *models.StagedEntity) (int, error) {
	ctx, span := tracing.StartSpan(ctx, "CriteriaEvaluator.EvaluateCriteriaForNewEntity")
	defer span.End()

	log := e.logger.WithContext(ctx).WithFields(map[string]any{
		"entity_id":   entity.ID,
		"entity_type": entity.EntityType,
		"integration": entity.Integration,
	})

	// Find all criteria that target this entity type + integration
	criteriaList, err := e.criteriaRepo.FindByTarget(ctx, entity.TenantID, entity.EntityType, entity.Integration)
	if err != nil {
		log.WithError(err).Error("Failed to find criteria for entity")
		return 0, err
	}

	if len(criteriaList) == 0 {
		log.Debug("No criteria definitions found for entity type")
		return 0, nil
	}

	matchCount := 0
	for i := range criteriaList {
		crit := &criteriaList[i]
		
		// Parse the criteria
		var criteriaMap map[string]any
		if err := json.Unmarshal(crit.Criteria, &criteriaMap); err != nil {
			log.WithError(err).WithFields(map[string]any{
				"criteria_id": crit.ID,
			}).Warn("Failed to parse criteria JSON")
			continue
		}

		// Evaluate if entity matches
		if criteria.MatchesCriteria(entity.Data, criteriaMap) {
			// Create a staged_relationship for this match
			if err := e.createMatchWithRelationship(ctx, entity.TenantID, crit, entity, log); err != nil {
				log.WithError(err).WithFields(map[string]any{
					"criteria_id": crit.ID,
				}).Warn("Failed to create criteria match")
				continue
			}
			matchCount++
			log.WithFields(map[string]any{
				"criteria_id":       crit.ID,
				"relationship_type": crit.RelationshipType,
			}).Debug("Entity matched criteria")
		}
	}

	if matchCount > 0 {
		log.WithFields(map[string]any{"match_count": matchCount}).Info("Entity matched criteria definitions")
	}

	return matchCount, nil
}

// createMatchWithRelationship creates a staged_relationship for a criteria match
// and records the match. The staged_relationship flows through the normal merge pipeline.
func (e *CriteriaEvaluator) createMatchWithRelationship(
	ctx context.Context,
	tenantID string,
	crit *models.StagedRelationshipCriteria,
	toEntity *models.StagedEntity,
	log ectologger.Logger,
) error {
	ctx, span := tracing.StartSpan(ctx, "CriteriaEvaluator.createMatchWithRelationship")
	defer span.End()

	// Create a staged_relationship for this match
	critID := crit.ID
	stagedRel, err := e.relationshipRepo.Create(ctx, tenantID, models.CreateStagedRelationshipRequest{
		RelationshipType: crit.RelationshipType,

		FromEntityType:  crit.FromEntityType,
		FromSourceID:    crit.FromSourceID,
		FromIntegration: crit.FromIntegration,

		ToEntityType:  toEntity.EntityType,
		ToSourceID:    toEntity.SourceID,
		ToIntegration: toEntity.Integration,

		CriteriaID: &critID,

		Integration:       crit.Integration,
		SourceKey:         crit.SourceKey,
		ConfigID:          crit.ConfigID,
		SourceExecutionID: crit.SourceExecutionID,
		Data:              crit.Data,
	})
	if err != nil {
		return err
	}

	// Record the match with reference to the staged_relationship
	_, err = e.criteriaRepo.AddMatch(ctx, tenantID, crit, toEntity, stagedRel.ID)
	if err != nil {
		// Log but don't fail - the staged_relationship was created
		log.WithError(err).Warn("Failed to record criteria match (staged_relationship created)")
	}

	log.WithFields(map[string]any{
		"staged_rel_id": stagedRel.ID,
		"criteria_id":   crit.ID,
		"to_entity_id":  toEntity.ID,
	}).Debug("Created staged_relationship from criteria match")

	return nil
}

// EvaluateCriteriaForNewCriteria evaluates a new criteria definition against existing entities
// and creates staged_relationships + matches for all matching entities
func (e *CriteriaEvaluator) EvaluateCriteriaForNewCriteria(ctx context.Context, crit *models.StagedRelationshipCriteria) (int, error) {
	ctx, span := tracing.StartSpan(ctx, "CriteriaEvaluator.EvaluateCriteriaForNewCriteria")
	defer span.End()

	log := e.logger.WithContext(ctx).WithFields(map[string]any{
		"criteria_id":       crit.ID,
		"relationship_type": crit.RelationshipType,
		"to_entity_type":    crit.ToEntityType,
		"to_integration":    crit.ToIntegration,
	})

	// Parse the criteria
	var criteriaMap map[string]any
	if err := json.Unmarshal(crit.Criteria, &criteriaMap); err != nil {
		log.WithError(err).Error("Failed to parse criteria JSON")
		return 0, err
	}

	// Find all entities that match the target type + integration
	entities, err := e.entityRepo.FindByTypeAndIntegration(ctx, crit.TenantID, crit.ToEntityType, crit.ToIntegration)
	if err != nil {
		log.WithError(err).Error("Failed to find target entities")
		return 0, err
	}

	matchCount := 0
	for i := range entities {
		entity := &entities[i]
		
		// Evaluate if entity matches criteria
		if criteria.MatchesCriteria(entity.Data, criteriaMap) {
			// Create a staged_relationship for this match
			if err := e.createMatchWithRelationship(ctx, entity.TenantID, crit, entity, log); err != nil {
				log.WithError(err).WithFields(map[string]any{
					"entity_id": entity.ID,
				}).Warn("Failed to create criteria match")
				continue
			}
			matchCount++
		}
	}

	if matchCount > 0 {
		log.WithFields(map[string]any{"match_count": matchCount}).Info("Criteria matched existing entities")
	}

	return matchCount, nil
}

// ProcessMessage handles an incoming Kafka message
func (p *Processor) ProcessMessage(ctx context.Context, msg *kafka.IncomingMessage) error {
	ctx, span := tracing.StartSpan(ctx, "processor.ProcessMessage")
	defer span.End()

	log := p.logger.WithContext(ctx).WithFields(map[string]any{
		"key":    msg.Key,
		"topic":  msg.Topic,
		"offset": msg.Offset,
	})

	// Check if this is an execution completed event
	if msg.IsExecutionCompleted() {
		return p.handleExecutionCompleted(ctx, msg)
	}

	// Explicit delete messages (e.g. delta/tombstone events)
	if msg.DeleteMessage != nil && msg.DeleteMessage.Action == "delete" {
		return p.handleDelete(ctx, msg.DeleteMessage)
	}

	// Parse the Lotus message if not already parsed
	if msg.LotusMessage == nil {
		if err := msg.ParseLotusMessage(); err != nil {
			log.WithError(err).Error("Failed to parse Lotus message")
			return err
		}
	}

	tenantID := msg.GetTenantID()
	if tenantID == "" {
		log.Error("Missing tenant_id in message")
		return nil // Skip message, don't retry
	}

	log = log.WithFields(map[string]any{"tenant_id": tenantID})

	// Handle relationship or entity - check relationship FIRST because IsEntity() defaults to true
	if msg.IsRelationship() {
		return p.processRelationship(ctx, msg, log)
	} else if msg.IsEntity() {
		return p.processEntity(ctx, msg, log)
	}

	log.Warn("Unknown message type, skipping")
	return nil
}

func (p *Processor) handleDelete(ctx context.Context, del *models.LotusDeleteMessage) error {
	ctx, span := tracing.StartSpan(ctx, "processor.handleDelete")
	defer span.End()

	if del == nil {
		return nil
	}

	tenantID := del.Source.TenantID
	sourceKey := del.Source.Key

	log := p.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":   tenantID,
		"source_key":  sourceKey,
		"entity_type": del.EntityType,
		"entity_id":   del.EntityID,
	})

	// Resolve staged entity (may be ambiguous across integration; prefer plan match if present)
	candidates, err := p.entityRepo.FindBySourceID(ctx, tenantID, del.EntityType, del.EntityID)
	if err != nil {
		log.WithError(err).Warn("Failed to find staged entity for delete")
		return nil
	}
	if len(candidates) == 0 {
		log.Debug("Entity not found, skipping delete")
		return nil
	}

	var entity *models.StagedEntity
	if sourceKey != "" {
		for i := range candidates {
			if candidates[i].SourceKey == sourceKey {
				entity = &candidates[i]
				break
			}
		}
	}
	if entity == nil {
		entity = &candidates[0]
	}

	// Soft-delete staged entity
	if err := p.entityRepo.Delete(ctx, tenantID, entity.ID); err != nil {
		log.WithError(err).Warn("Failed to delete staged entity")
		return nil
	}

	// Soft-delete staged relationships for that entity (scope by plan if available)
	if p.relationshipRepo != nil {
		if sourceKey != "" {
			_, _ = p.relationshipRepo.SoftDeleteByEntityIDAndSourceKey(ctx, tenantID, entity.ID, sourceKey)
		} else {
			_, _ = p.relationshipRepo.SoftDeleteByEntityID(ctx, tenantID, entity.ID)
		}
	}

	// The CDC consumers will handle cascading deletes to merged entities/relationships
	log.Info("Staged entity deleted - CDC consumers will handle merged entity cleanup")
	return nil
}

// processEntity processes an entity message
func (p *Processor) processEntity(ctx context.Context, msg *kafka.IncomingMessage, log ectologger.Logger) error {
	ctx, span := tracing.StartSpan(ctx, "processor.processEntity")
	defer span.End()

	tenantID := msg.GetTenantID()
	entityType := msg.GetEntityType()
	sourceID := msg.GetSourceID()
	integration := msg.GetIntegration()
	configID := msg.GetConfigID()
	data := msg.GetData()

	log = log.WithFields(map[string]any{
		"entity_type": entityType,
		"source_id":   sourceID,
		"integration": integration,
		"config_id":   configID,
	})
	log.Debug("Processing entity")

	if entityType == "" || sourceID == "" || integration == "" {
		log.WithFields(map[string]any{
			"integration": integration,
			"config_id":   configID,
			"entity_type": entityType,
			"source_id":   sourceID,
		}).Warn("Skipping entity: missing required identity fields (_entity_type, _source_id, _integration)")
		return nil
	}

	// Validate against schema if available
	if p.schemaService != nil && entityType != "" {
		var dataMap map[string]any
		if err := json.Unmarshal(data, &dataMap); err == nil {
			result, err := p.schemaService.ValidateEntityData(ctx, tenantID, entityType, dataMap)
			if err != nil {
				log.WithError(err).Warn("Schema validation error")
			} else if !result.Valid {
				log.WithFields(map[string]any{"errors": result.Errors}).Warn("Schema validation failed")
				// Continue processing but log warning
			}
		}
	}

	// Get source info
	sourceKey := msg.GetSourceKey()
	execID := msg.GetExecutionID()

	// Build upsert request
	req := models.CreateStagedEntityRequest{
		EntityType:        entityType,
		SourceID:          sourceID,
		Integration:       integration,
		SourceKey:         sourceKey,
		ConfigID:          configID,
		SourceExecutionID: nilIfEmpty(execID),
		Data:              data,
	}

	// Get fingerprint exclusions from schema (if defined)
	var upsertOpts *stagedentity.UpsertOptions
	if p.schemaService != nil && entityType != "" {
		exclusions, err := p.schemaService.GetFingerprintExclusions(ctx, tenantID, entityType)
		if err != nil {
			log.WithError(err).Warn("Failed to get fingerprint exclusions from schema")
			// Continue without exclusions
		} else if len(exclusions) > 0 {
			upsertOpts = &stagedentity.UpsertOptions{
				ExcludeFieldsFromFingerprint: exclusions,
			}
		}
	}

	// Upsert the entity
	result, err := p.entityRepo.UpsertWithOptions(ctx, tenantID, req, upsertOpts)
	if err != nil {
		log.WithError(err).Error("Failed to upsert entity")
		return err
	}

	log.WithFields(map[string]any{
		"entity_id":  result.Entity.ID,
		"is_new":     result.IsNew,
		"is_changed": result.IsChanged,
	}).Info("Entity staged - Debezium CDC will trigger merge consumer")

	// Process embedded relationships (also just stages them for CDC)
	for _, rel := range msg.GetRelationships() {
		if err := p.processEmbeddedRelationship(ctx, tenantID, result.Entity, rel, msg, log); err != nil {
			log.WithError(err).Error("Failed to process embedded relationship")
			// Continue with other relationships
		}
	}

	// Evaluate criteria-based relationships for this entity (backfill path)
	// This checks if this entity matches any existing criteria definitions
	if p.criteriaEvaluator != nil {
		matchCount, err := p.criteriaEvaluator.EvaluateCriteriaForNewEntity(ctx, result.Entity)
		if err != nil {
			log.WithError(err).Warn("Failed to evaluate criteria for entity")
			// Don't fail the whole operation
		} else if matchCount > 0 {
			log.WithFields(map[string]any{"criteria_match_count": matchCount}).Info("Entity matched criteria definitions")
		}
	}

	return nil
}

// processRelationship processes a standalone relationship message
// Supports both direct (source_id to source_id) and criteria-based relationships
func (p *Processor) processRelationship(ctx context.Context, msg *kafka.IncomingMessage, log ectologger.Logger) error {
	ctx, span := tracing.StartSpan(ctx, "processor.processRelationship")
	defer span.End()

	if msg.LotusMessage == nil || msg.LotusMessage.Data == nil {
		return nil
	}

	// Parse relationship envelope
	b, _ := json.Marshal(msg.LotusMessage.Data)
	var rel models.RelationshipRecord
	if err := json.Unmarshal(b, &rel); err != nil {
		log.WithError(err).Warn("Failed to parse relationship record")
		return nil
	}

	// Normalize with message integration
	msgIntegration := msg.LotusMessage.Source.Integration
	rel.Normalize(msgIntegration)

	// Validate required fields
	if !rel.IsValid() {
		log.WithFields(map[string]any{
			"relationship_type": rel.RelationshipType,
			"from_entity_type":  rel.FromEntityType,
			"to_entity_type":    rel.ToEntityType,
			"is_criteria":       rel.IsCriteriaBased(),
		}).Warn("Invalid relationship record (missing required fields)")
		return nil
	}

	tenantID := msg.GetTenantID()
	sourceKey := msg.GetSourceKey()
	configID := msg.GetConfigID()
	execID := msg.GetExecutionID()

	// Relationship properties = all non-underscore keys
	props := make(map[string]any)
	for k, v := range msg.LotusMessage.Data {
		if len(k) > 0 && k[0] == '_' {
			continue
		}
		props[k] = v
	}

	var relData json.RawMessage
	if len(props) > 0 {
		relData, _ = json.Marshal(props)
	}

	// Handle criteria-based vs direct relationships
	if rel.IsCriteriaBased() {
		return p.processCriteriaRelationship(ctx, tenantID, rel, msgIntegration, sourceKey, configID, execID, relData, log)
	}
	return p.processDirectRelationship(ctx, tenantID, rel, msgIntegration, sourceKey, configID, execID, relData, log)
}

// processDirectRelationship handles direct source_id to source_id relationships
func (p *Processor) processDirectRelationship(
	ctx context.Context,
	tenantID string,
	rel models.RelationshipRecord,
	msgIntegration, sourceKey, configID, execID string,
	relData json.RawMessage,
	log ectologger.Logger,
) error {
	ctx, span := tracing.StartSpan(ctx, "processor.processDirectRelationship")
	defer span.End()

	stagedRel, err := p.relationshipRepo.Create(ctx, tenantID, models.CreateStagedRelationshipRequest{
		RelationshipType: rel.RelationshipType,

		FromEntityType:  rel.FromEntityType,
		FromSourceID:    rel.FromSourceID,
		FromIntegration: rel.FromIntegration,

		ToEntityType:  rel.ToEntityType,
		ToSourceID:    rel.ToSourceID,
		ToIntegration: rel.ToIntegration,

		Integration:       msgIntegration,
		SourceKey:         sourceKey,
		ConfigID:          configID,
		SourceExecutionID: nilIfEmpty(execID),
		Data:              relData,
	})
	if err != nil {
		log.WithError(err).Error("Failed to create staged relationship")
		return err
	}

	log.WithFields(map[string]any{
		"relationship_id":  stagedRel.ID,
		"from_integration": rel.FromIntegration,
		"to_integration":   rel.ToIntegration,
	}).Info("Direct relationship staged - Debezium CDC will resolve entities")

	return nil
}

// processCriteriaRelationship handles criteria-based relationships
func (p *Processor) processCriteriaRelationship(
	ctx context.Context,
	tenantID string,
	rel models.RelationshipRecord,
	msgIntegration, sourceKey, configID, execID string,
	relData json.RawMessage,
	log ectologger.Logger,
) error {
	ctx, span := tracing.StartSpan(ctx, "processor.processCriteriaRelationship")
	defer span.End()

	log = log.WithFields(map[string]any{
		"relationship_type": rel.RelationshipType,
		"from_source_id":    rel.FromSourceID,
		"to_entity_type":    rel.ToEntityType,
		"to_integration":    rel.ToIntegration,
		"criteria":          rel.ToCriteria,
	})

	// ToIntegration is required for criteria-based relationships
	if rel.ToIntegration == "" {
		log.Warn("Criteria-based relationship missing _to_integration")
		return nil
	}

	// Create criteria definition
	result, err := p.criteriaRepo.Upsert(ctx, tenantID, models.CreateStagedRelationshipCriteriaRequest{
		RelationshipType: rel.RelationshipType,

		FromEntityType:  rel.FromEntityType,
		FromSourceID:    rel.FromSourceID,
		FromIntegration: rel.FromIntegration,

		ToEntityType:  rel.ToEntityType,
		ToIntegration: rel.ToIntegration,
		Criteria:      rel.ToCriteria,

		Integration:       msgIntegration,
		SourceKey:         sourceKey,
		ConfigID:          configID,
		SourceExecutionID: nilIfEmpty(execID),
		Data:              relData,
	})
	if err != nil {
		log.WithError(err).Error("Failed to create criteria relationship")
		return err
	}

	log.WithFields(map[string]any{
		"criteria_id": result.Criteria.ID,
		"is_new":      result.IsNew,
	}).Info("Criteria relationship staged")

	// If this is a new criteria, evaluate it against existing entities
	if result.IsNew && p.criteriaEvaluator != nil {
		matchCount, err := p.criteriaEvaluator.EvaluateCriteriaForNewCriteria(ctx, result.Criteria)
		if err != nil {
			log.WithError(err).Warn("Failed to evaluate criteria against existing entities")
			// Don't fail the whole operation
		} else if matchCount > 0 {
			log.WithFields(map[string]any{"match_count": matchCount}).Info("Criteria matched existing entities")
		}
	}

	return nil
}

// processEmbeddedRelationship processes a relationship embedded in an entity message
func (p *Processor) processEmbeddedRelationship(
	ctx context.Context,
	tenantID string,
	fromEntity *models.StagedEntity,
	rel models.LotusRelationship,
	msg *kafka.IncomingMessage,
	log ectologger.Logger,
) error {
	ctx, span := tracing.StartSpan(ctx, "processor.processEmbeddedRelationship")
	defer span.End()

	log = log.WithFields(map[string]any{
		"relationship_type": rel.Type,
		"to_source_id":      rel.ToSourceID,
	})

	// Build relationship data
	var relData json.RawMessage
	if rel.Data != nil {
		relData, _ = json.Marshal(rel.Data)
	}

	sourceKey := msg.GetSourceKey()
	configID := msg.GetConfigID()
	execID := msg.GetExecutionID()
	integration := msg.GetIntegration()

	// For embedded relationships, we know the from entity (it's the parent entity)
	// Embedded relationships are always direct (not criteria-based)
	req := models.CreateStagedRelationshipRequest{
		RelationshipType: rel.Type,

		FromEntityType:  fromEntity.EntityType,
		FromSourceID:    fromEntity.SourceID,
		FromIntegration: fromEntity.Integration,

		ToEntityType:  rel.ToEntityType,
		ToSourceID:    rel.ToSourceID,
		ToIntegration: integration, // Embedded relationships use message integration for to-side

		Integration:       integration,
		SourceKey:         sourceKey,
		ConfigID:          configID,
		SourceExecutionID: nilIfEmpty(execID),
		Data:              relData,
	}

	relationship, err := p.relationshipRepo.Create(ctx, tenantID, req)
	if err != nil {
		return err
	}

	log.WithFields(map[string]any{
		"relationship_id": relationship.ID,
	}).Info("Created embedded relationship - CDC will resolve to entity")
	return nil
}

// handleExecutionCompleted handles execution.completed events from Orchid
func (p *Processor) handleExecutionCompleted(ctx context.Context, msg *kafka.IncomingMessage) error {
	ctx, span := tracing.StartSpan(ctx, "processor.handleExecutionCompleted")
	defer span.End()

	evt, err := msg.ParseExecutionCompleted()
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to parse execution.completed event")
		return err
	}

	log := p.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":    evt.TenantID,
		"source_key":   evt.SourceKey,
		"execution_id": evt.ExecutionID,
		"status":       evt.Status,
	})

	log.Info("Received execution.completed event")

	// Only process successful or partial executions for deletion
	if evt.Status == "failed" {
		log.Debug("Skipping deletion for failed execution")
		return nil
	}

	// Execute deletion strategies
	if p.deletionStrategyRepo != nil && p.deletionEngine != nil {
		if err := p.executeDeletionStrategies(ctx, evt); err != nil {
			log.WithError(err).Error("Failed to execute deletion strategies")
			// Don't fail the entire operation if deletion strategies fail
			// Just log and continue
		}
	}

	return nil
}

// executeDeletionStrategies applies all applicable deletion strategies for the execution
func (p *Processor) executeDeletionStrategies(ctx context.Context, evt *kafka.ExecutionCompletedMessage) error {
	ctx, span := tracing.StartSpan(ctx, "processor.executeDeletionStrategies")
	defer span.End()

	log := p.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":    evt.TenantID,
		"source_key":   evt.SourceKey,
		"execution_id": evt.ExecutionID,
	})

	// Get all applicable deletion strategies for this tenant
	strategies, err := p.deletionStrategyRepo.GetApplicableStrategies(ctx, evt.TenantID)
	if err != nil {
		log.WithError(err).Error("Failed to get deletion strategies")
		return err
	}

	if len(strategies) == 0 {
		log.Debug("No deletion strategies configured for tenant")
		return nil
	}

	log.WithFields(map[string]any{"strategy_count": len(strategies)}).Info("Executing deletion strategies")

	// Build execution context
	execCtx := deletion.ExecutionContext{
		TenantID:    evt.TenantID,
		SourceKey:   evt.SourceKey,
		ExecutionID: evt.ExecutionID,
		Status:      evt.Status,
	}

	// Execute each strategy
	var totalDeleted int64
	for i := range strategies {
		count, err := p.deletionEngine.ExecuteStrategy(ctx, &strategies[i], execCtx)
		if err != nil {
			log.WithError(err).WithFields(map[string]any{
				"strategy_id": strategies[i].ID,
			}).Error("Failed to execute deletion strategy")
			// Continue with other strategies
			continue
		}
		totalDeleted += count
	}

	log.WithFields(map[string]any{
		"total_deleted": totalDeleted,
	}).Info("Deletion strategies executed")

	return nil
}

// Helper functions

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
