// Package merging implements entity merge logic and golden record generation
package merging

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/internal/repositories/entitytype"
	"github.com/Ramsey-B/ivy/internal/repositories/mergedrelationship"
	"github.com/Ramsey-B/stem/pkg/tracing"

	"github.com/Ramsey-B/ivy/internal/repositories/mergedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/pkg/matching"
	"github.com/Ramsey-B/ivy/pkg/models"
)

// Engine handles entity merging
type Engine struct {
	logger         ectologger.Logger
	entityRepo     *stagedentity.Repository
	mergedRepo     *mergedentity.Repository
	mergedRelRepo  *mergedrelationship.Repository
	entityTypeRepo *entitytype.Repository
	fieldMerger    *FieldMerger
}

// NewEngine creates a new merge engine
func NewEngine(
	logger ectologger.Logger,
	entityRepo *stagedentity.Repository,
	mergedRepo *mergedentity.Repository,
	mergedRelRepo *mergedrelationship.Repository,
	entityTypeRepo *entitytype.Repository,
) *Engine {
	return &Engine{
		logger:         logger,
		entityRepo:     entityRepo,
		mergedRepo:     mergedRepo,
		mergedRelRepo:  mergedRelRepo,
		entityTypeRepo: entityTypeRepo,
		fieldMerger:    NewFieldMerger(),
	}
}

// MergeWithMatches processes a source entity and its matches to produce a merged entity.
//
// Behavior:
//   - If no matches (or all matches are blocked): upsert the source entity as its own merged_entity
//   - If there are matches: merge the source entity with matched entities into an existing
//     merged_entity (if one exists) or create a new cluster
//
// Returns the merge result with the merged entity, conflicts, and metadata.
func (e *Engine) MergeWithMatches(
	ctx context.Context,
	sourceEntity *models.StagedEntity,
	matchResults *matching.MatchResults,
) (*models.MergeResult, error) {
	ctx, span := tracing.StartSpan(ctx, "merging.Engine.MergeWithMatches")
	defer span.End()

	log := e.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":   sourceEntity.TenantID,
		"entity_type": sourceEntity.EntityType,
		"entity_id":   sourceEntity.ID,
		"match_count": len(matchResults.Matches),
	})

	// Load entity type schema for merge strategies
	schema, err := e.loadSchema(ctx, sourceEntity.TenantID, sourceEntity.EntityType)
	if err != nil {
		return nil, err
	}

	fieldStrategies := deriveFieldStrategiesFromSchema(schema)
	sourcePriorities := schema.SourcePriorities
	if len(sourcePriorities) > 0 {
		log.WithFields(map[string]any{
			"source_priorities_count": len(sourcePriorities),
		}).Debug("Loaded source priorities from entity type schema")
	}

	// Filter out blocked matches (no-merge rules)
	validMatches := make([]matching.MatchOutcome, 0, len(matchResults.Matches))
	for _, m := range matchResults.Matches {
		if !m.Blocked {
			validMatches = append(validMatches, m)
		} else {
			log.WithFields(map[string]any{
				"blocked_entity_id": m.EntityID,
				"rule":              m.RuleMatched,
			}).Debug("Match blocked by no-merge rule")
		}
	}

	// Case 1: No valid matches - upsert source entity as its own merged entity
	if len(validMatches) == 0 {
		return e.upsertSingleEntity(ctx, sourceEntity, fieldStrategies, sourcePriorities, log)
	}

	// Case 2: Has matches - merge source with matched entities
	return e.mergeEntityWithMatches(ctx, sourceEntity, matchResults, fieldStrategies, sourcePriorities, log)
}

// upsertSingleEntity creates or updates a merged entity for a single source entity with no matches.
func (e *Engine) upsertSingleEntity(
	ctx context.Context,
	sourceEntity *models.StagedEntity,
	strategies []models.FieldMergeStrategy,
	priorities []models.SourcePriority,
	log ectologger.Logger,
) (*models.MergeResult, error) {
	ctx, span := tracing.StartSpan(ctx, "merging.Engine.upsertSingleEntity")
	defer span.End()

	// Check if this entity is already part of a merged entity
	existingMerged, err := e.mergedRepo.GetMergedEntityByStagedID(ctx, sourceEntity.TenantID, sourceEntity.ID)
	if err != nil {
		return nil, err
	}

	// Build merged data from just this entity
	entities := []models.StagedEntity{*sourceEntity}
	mergedData, conflicts := e.mergeFields(ctx, entities, strategies, priorities)
	mergedDataJSON, err := json.Marshal(mergedData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged data: %w", err)
	}

	if existingMerged != nil {
		// Update existing merged entity
		log.WithFields(map[string]any{"merged_id": existingMerged.ID}).Debug("Updating existing merged entity (single source)")

		updated, err := e.mergedRepo.Update(ctx, existingMerged.ID, sourceEntity.TenantID, mergedDataJSON, 1)
		if err != nil {
			return nil, err
		}

		return &models.MergeResult{
			MergedEntity:   updated,
			SourceEntities: []string{sourceEntity.ID},
			Conflicts:      conflicts,
			IsNew:          false,
			Version:        updated.Version,
		}, nil
	}

	// Create new merged entity
	log.Debug("Creating new merged entity (single source)")

	ctxTx, tx, err := e.mergedRepo.DB().GetTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctxTx)

	merged := &models.MergedEntity{
		TenantID:        sourceEntity.TenantID,
		EntityType:      sourceEntity.EntityType,
		Data:            mergedDataJSON,
		SourceCount:     1,
		PrimarySourceID: &sourceEntity.ID,
	}

	created, err := e.mergedRepo.Create(ctxTx, merged)
	if err != nil {
		return nil, err
	}

	if err := e.mergedRepo.AddToCluster(ctxTx, sourceEntity.TenantID, created.ID, sourceEntity.ID, true); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctxTx); err != nil {
		return nil, err
	}

	log.WithFields(map[string]any{"merged_id": created.ID}).Info("Created merged entity (single source)")

	return &models.MergeResult{
		MergedEntity:   created,
		SourceEntities: []string{sourceEntity.ID},
		Conflicts:      conflicts,
		IsNew:          true,
		Version:        1,
	}, nil
}

// mergeEntityWithMatches merges a source entity with its matched entities.
// Handles three scenarios:
// 1. No existing merged entities - create new cluster
// 2. One existing merged entity - add to existing cluster
// 3. Multiple existing merged entities - consolidate clusters into one
func (e *Engine) mergeEntityWithMatches(
	ctx context.Context,
	sourceEntity *models.StagedEntity,
	matchResults *matching.MatchResults,
	strategies []models.FieldMergeStrategy,
	priorities []models.SourcePriority,
	log ectologger.Logger,
) (*models.MergeResult, error) {
	ctx, span := tracing.StartSpan(ctx, "merging.Engine.mergeEntityWithMatches")
	defer span.End()

	// Collect all entity IDs (source + matches)
	allEntityIDs := make([]string, 0, len(matchResults.Matches)+1)
	allEntityIDs = append(allEntityIDs, sourceEntity.ID)
	for _, m := range matchResults.Matches {
		allEntityIDs = append(allEntityIDs, m.EntityID)
	}

	// Load all matched entities
	allEntities, err := e.entityRepo.GetByIDs(ctx, sourceEntity.TenantID, allEntityIDs)
	if err != nil {
		return nil, err
	}

	// Ensure the source entity is included (repo should return it, but be defensive).
	foundSource := false
	for i := range allEntities {
		if allEntities[i].ID == sourceEntity.ID {
			foundSource = true
			break
		}
	}
	if !foundSource {
		allEntities = append(allEntities, *sourceEntity)
	}

	if len(allEntities) == 1 {
		// Only source entity loaded (all matches failed to load) - treat as single entity
		return e.upsertSingleEntity(ctx, sourceEntity, strategies, priorities, log)
	}

	// Find ALL existing merged entities that contain any of the staged entities
	// This detects the cluster consolidation scenario (e.g., Device C matches A and B, which are in different clusters)
	existingMergedEntities, err := e.mergedRepo.GetMergedEntitiesByStagedIDs(ctx, sourceEntity.TenantID, allEntityIDs)
	if err != nil {
		return nil, err
	}

	// Merge all entity data
	mergedData, conflicts := e.mergeFields(ctx, allEntities, strategies, priorities)
	mergedDataJSON, err := json.Marshal(mergedData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged data: %w", err)
	}

	// Start transaction
	ctxTx, tx, err := e.mergedRepo.DB().GetTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctxTx)

	var result *models.MergeResult

	switch len(existingMergedEntities) {
	case 0:
		// No existing merged entities - create new cluster
		result, err = e.createNewCluster(ctxTx, sourceEntity, allEntities, allEntityIDs, mergedDataJSON, conflicts, log)

	case 1:
		// One existing merged entity - add to existing cluster
		result, err = e.updateExistingCluster(ctxTx, sourceEntity, &existingMergedEntities[0], allEntities, allEntityIDs, mergedDataJSON, conflicts, log)

	default:
		// Multiple existing merged entities - consolidate clusters
		result, err = e.consolidateClusters(ctxTx, sourceEntity, existingMergedEntities, allEntities, allEntityIDs, mergedDataJSON, conflicts, log)
	}

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctxTx); err != nil {
		return nil, err
	}

	return result, nil
}

// createNewCluster creates a new merged entity and adds all sources to its cluster.
func (e *Engine) createNewCluster(
	ctx context.Context,
	sourceEntity *models.StagedEntity,
	allEntities []models.StagedEntity,
	allEntityIDs []string,
	mergedDataJSON json.RawMessage,
	conflicts []models.MergeConflict,
	log ectologger.Logger,
) (*models.MergeResult, error) {
	log.WithFields(map[string]any{"source_count": len(allEntities)}).Debug("Creating new merged entity with matches")

	merged := &models.MergedEntity{
		TenantID:        sourceEntity.TenantID,
		EntityType:      sourceEntity.EntityType,
		Data:            mergedDataJSON,
		SourceCount:     len(allEntities),
		PrimarySourceID: &sourceEntity.ID,
	}

	created, err := e.mergedRepo.Create(ctx, merged)
	if err != nil {
		return nil, err
	}

	// Add all entities to cluster
	for i, entity := range allEntities {
		isPrimary := i == 0
		if err := e.mergedRepo.AddToCluster(ctx, sourceEntity.TenantID, created.ID, entity.ID, isPrimary); err != nil {
			return nil, err
		}
	}

	log.WithFields(map[string]any{
		"merged_id":    created.ID,
		"source_count": len(allEntities),
	}).Info("Created new merged entity")

	return &models.MergeResult{
		MergedEntity:   created,
		SourceEntities: allEntityIDs,
		Conflicts:      conflicts,
		IsNew:          true,
		Version:        1,
	}, nil
}

// updateExistingCluster adds entities to an existing merged entity's cluster.
func (e *Engine) updateExistingCluster(
	ctx context.Context,
	sourceEntity *models.StagedEntity,
	existingMerged *models.MergedEntity,
	allEntities []models.StagedEntity,
	allEntityIDs []string,
	mergedDataJSON json.RawMessage,
	conflicts []models.MergeConflict,
	log ectologger.Logger,
) (*models.MergeResult, error) {
	log.WithFields(map[string]any{
		"merged_id":    existingMerged.ID,
		"source_count": len(allEntities),
	}).Debug("Updating existing merged entity with new matches")

	updated, err := e.mergedRepo.Update(ctx, existingMerged.ID, sourceEntity.TenantID, mergedDataJSON, len(allEntities))
	if err != nil {
		return nil, err
	}

	// Add all entities to cluster (AddToCluster is idempotent via ON CONFLICT)
	for i, entity := range allEntities {
		isPrimary := i == 0 && existingMerged.PrimarySourceID == nil
		if err := e.mergedRepo.AddToCluster(ctx, sourceEntity.TenantID, existingMerged.ID, entity.ID, isPrimary); err != nil {
			return nil, err
		}
	}

	log.WithFields(map[string]any{
		"merged_id":    existingMerged.ID,
		"source_count": len(allEntities),
	}).Info("Updated existing merged entity")

	return &models.MergeResult{
		MergedEntity:   updated,
		SourceEntities: allEntityIDs,
		Conflicts:      conflicts,
		IsNew:          false,
		Version:        updated.Version,
	}, nil
}

// consolidateClusters merges multiple existing clusters into one.
// This happens when a new entity matches entities that were previously in separate clusters.
// Example: Device A (Cluster 1), Device B (Cluster 2), Device C matches both A and B.
func (e *Engine) consolidateClusters(
	ctx context.Context,
	sourceEntity *models.StagedEntity,
	existingMergedEntities []models.MergedEntity,
	allEntities []models.StagedEntity,
	allEntityIDs []string,
	mergedDataJSON json.RawMessage,
	conflicts []models.MergeConflict,
	log ectologger.Logger,
) (*models.MergeResult, error) {
	log.WithFields(map[string]any{
		"cluster_count": len(existingMergedEntities),
		"source_count":  len(allEntities),
	}).Info("Consolidating multiple clusters into one")

	// Use the oldest merged entity as the "surviving" cluster (first in list, ordered by created_at)
	survivingCluster := &existingMergedEntities[0]
	clustersToMerge := existingMergedEntities[1:]

	// Move all members from other clusters to the surviving cluster
	for _, cluster := range clustersToMerge {
		log.WithFields(map[string]any{
			"from_cluster": cluster.ID,
			"to_cluster":   survivingCluster.ID,
		}).Debug("Moving cluster members")

		if err := e.mergedRepo.MoveClusterMembers(ctx, sourceEntity.TenantID, cluster.ID, survivingCluster.ID); err != nil {
			return nil, err
		}

		// Rewire merged relationships that referenced the old merged entity ID.
		// This ensures golden edges follow the surviving merged entity after consolidation.
		if e.mergedRelRepo != nil {
			if _, err := e.mergedRelRepo.RewireMergedEntity(ctx, sourceEntity.TenantID, cluster.ID, survivingCluster.ID); err != nil {
				return nil, err
			}
		}

		// Soft-delete the now-empty cluster
		if err := e.mergedRepo.SoftDelete(ctx, sourceEntity.TenantID, cluster.ID); err != nil {
			return nil, err
		}
	}

	// Add the source entity and any other new entities to the surviving cluster
	for i, entity := range allEntities {
		isPrimary := i == 0 && survivingCluster.PrimarySourceID == nil
		if err := e.mergedRepo.AddToCluster(ctx, sourceEntity.TenantID, survivingCluster.ID, entity.ID, isPrimary); err != nil {
			return nil, err
		}
	}

	// Update the surviving cluster with merged data
	updated, err := e.mergedRepo.Update(ctx, survivingCluster.ID, sourceEntity.TenantID, mergedDataJSON, len(allEntities))
	if err != nil {
		return nil, err
	}

	log.WithFields(map[string]any{
		"merged_id":          survivingCluster.ID,
		"consolidated_count": len(clustersToMerge),
		"final_source_count": len(allEntities),
	}).Info("Cluster consolidation complete")

	return &models.MergeResult{
		MergedEntity:   updated,
		SourceEntities: allEntityIDs,
		Conflicts:      conflicts,
		IsNew:          false,
		Version:        updated.Version,
	}, nil
}

// loadSchema loads the entity type schema, returning an empty schema if not found.
func (e *Engine) loadSchema(ctx context.Context, tenantID, entityType string) (models.EntityTypeSchema, error) {
	et, err := e.entityTypeRepo.GetByKey(ctx, tenantID, entityType)
	if err != nil {
		return models.EntityTypeSchema{}, fmt.Errorf("failed to get entity type %q: %w", entityType, err)
	}
	if et == nil {
		e.logger.WithContext(ctx).WithFields(map[string]any{"entity_type": entityType}).Warn("Entity type schema not found; using default merge strategies")
		return models.EntityTypeSchema{}, nil
	}

	var schema models.EntityTypeSchema
	if err := json.Unmarshal(et.Schema, &schema); err != nil {
		return models.EntityTypeSchema{}, fmt.Errorf("failed to parse entity type schema for %q: %w", entityType, err)
	}
	return schema, nil
}

// mergeFields merges fields from multiple entities using the configured strategies
func (e *Engine) mergeFields(
	ctx context.Context,
	entities []models.StagedEntity,
	strategies []models.FieldMergeStrategy,
	priorities []models.SourcePriority,
) (map[string]any, []models.MergeConflict) {
	ctx, span := tracing.StartSpan(ctx, "merging.Engine.mergeFields")
	defer span.End()

	// Parse all entity data
	parsedData := make([]entityDataWithMeta, len(entities))
	for i, entity := range entities {
		var data map[string]any
		json.Unmarshal(entity.Data, &data)
		parsedData[i] = entityDataWithMeta{
			Data:        data,
			UpdatedAt:   entity.UpdatedAt,
			Integration: entity.Integration,
			EntityID:    entity.ID,
		}
	}

	result := make(map[string]any)
	var conflicts []models.MergeConflict

	// Create strategy map for quick lookup
	strategyMap := make(map[string]models.FieldMergeStrategy)
	for _, s := range strategies {
		strategyMap[s.Field] = s
	}

	// Create priority map
	priorityMap := make(map[string]int)
	for _, p := range priorities {
		priorityMap[p.Integration] = p.Priority
	}

	// Collect all fields across all entities
	allFields := make(map[string]bool)
	for _, pd := range parsedData {
		for field := range pd.Data {
			allFields[field] = true
		}
	}

	// Merge each field
	for field := range allFields {
		strategy, hasStrategy := strategyMap[field]
		if !hasStrategy {
			// Use default strategy: prefer_non_empty
			strategy = models.FieldMergeStrategy{
				Field:    field,
				Strategy: models.MergeStrategyPreferNonEmpty,
			}
		}

		mergedValue, conflict := e.fieldMerger.MergeField(field, parsedData, strategy, priorityMap)
		if mergedValue != nil {
			result[field] = mergedValue
		}
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
		}
	}

	return result, conflicts
}

func deriveFieldStrategiesFromSchema(schema models.EntityTypeSchema) []models.FieldMergeStrategy {
	strategies := make([]models.FieldMergeStrategy, 0)
	for field, def := range schema.Properties {
		if def.MergeStrategy != "" {
			strategies = append(strategies, models.FieldMergeStrategy{
				Field:    field,
				Strategy: def.MergeStrategy,
			})
		}
	}
	return strategies
}

// entityDataWithMeta holds entity data with metadata for merge decisions
type entityDataWithMeta struct {
	Data        map[string]any
	UpdatedAt   interface{} // time.Time
	Integration string
	EntityID    string
}
