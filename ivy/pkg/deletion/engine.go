// Package deletion implements deletion strategy execution for entities and relationships
package deletion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedrelationship"
	"github.com/Ramsey-B/ivy/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// Engine executes deletion strategies for entities and relationships
type Engine struct {
	logger           ectologger.Logger
	entityRepo       *stagedentity.Repository
	relationshipRepo *stagedrelationship.Repository
}

// NewEngine creates a new deletion strategy engine
func NewEngine(
	logger ectologger.Logger,
	entityRepo *stagedentity.Repository,
	relationshipRepo *stagedrelationship.Repository,
) *Engine {
	return &Engine{
		logger:           logger,
		entityRepo:       entityRepo,
		relationshipRepo: relationshipRepo,
	}
}

// ExecutionContext contains the context for deletion execution
type ExecutionContext struct {
	TenantID    string
	SourceKey   string
	ExecutionID string
	Status      string // execution status (success, partial, failed)
}

// ExecuteStrategy executes a deletion strategy for the given context
func (e *Engine) ExecuteStrategy(ctx context.Context, strategy *models.DeletionStrategy, execCtx ExecutionContext) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.Engine.ExecuteStrategy")
	defer span.End()

	log := e.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":     execCtx.TenantID,
		"source_key":    execCtx.SourceKey,
		"execution_id":  execCtx.ExecutionID,
		"strategy_id":   strategy.ID,
		"strategy_type": strategy.StrategyType,
	})

	log.Debug("Executing deletion strategy")

	switch strategy.StrategyType {
	case models.DeletionStrategyExecutionBased:
		return e.executeExecutionBased(ctx, strategy, execCtx)
	case models.DeletionStrategyExplicit:
		// Explicit strategy doesn't auto-delete, so we do nothing
		log.Debug("Explicit strategy - no automatic deletion")
		return 0, nil
	case models.DeletionStrategyStaleness:
		return e.executeStaleness(ctx, strategy, execCtx)
	case models.DeletionStrategyRetention:
		return e.executeRetention(ctx, strategy, execCtx)
	case models.DeletionStrategyComposite:
		return e.executeComposite(ctx, strategy, execCtx)
	default:
		log.WithFields(map[string]any{"strategy_type": strategy.StrategyType}).Warn("Unknown strategy type")
		return 0, nil
	}
}

// executeExecutionBased marks entities/relationships not in the execution as deleted
func (e *Engine) executeExecutionBased(ctx context.Context, strategy *models.DeletionStrategy, execCtx ExecutionContext) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.Engine.executeExecutionBased")
	defer span.End()

	log := e.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":    execCtx.TenantID,
		"source_key":   execCtx.SourceKey,
		"execution_id": execCtx.ExecutionID,
	})

	var totalDeleted int64

	// Handle entities
	if strategy.EntityType != nil {
		count, err := e.entityRepo.MarkDeletedExceptExecution(
			ctx,
			execCtx.TenantID,
			execCtx.SourceKey,
			execCtx.ExecutionID,
			strategy.EntityType,
		)
		if err != nil {
			log.WithError(err).Error("Failed to mark entities deleted")
			return totalDeleted, err
		}
		totalDeleted += count
		log.WithFields(map[string]any{
			"entity_type": *strategy.EntityType,
			"count":       count,
		}).Info("Marked entities deleted (execution-based)")
	}

	// Handle relationships
	if strategy.RelationshipType != nil {
		count, err := e.relationshipRepo.MarkDeletedExceptExecution(
			ctx,
			execCtx.TenantID,
			execCtx.SourceKey,
			execCtx.ExecutionID,
			strategy.RelationshipType,
		)
		if err != nil {
			log.WithError(err).Error("Failed to mark relationships deleted")
			return totalDeleted, err
		}
		totalDeleted += count
		log.WithFields(map[string]any{
			"relationship_type": *strategy.RelationshipType,
			"count":             count,
		}).Info("Marked relationships deleted (execution-based)")
	}

	return totalDeleted, nil
}

// executeStaleness marks entities/relationships not updated within max_age_days as deleted
func (e *Engine) executeStaleness(ctx context.Context, strategy *models.DeletionStrategy, execCtx ExecutionContext) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.Engine.executeStaleness")
	defer span.End()

	// Parse config
	var config models.StalenessConfig
	if err := json.Unmarshal(strategy.Config, &config); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to parse staleness config")
		return 0, err
	}

	checkField := config.CheckField
	if checkField == "" {
		checkField = "updated_at"
	}

	cutoffTime := time.Now().UTC().AddDate(0, 0, -config.MaxAgeDays)

	log := e.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":    execCtx.TenantID,
		"max_age_days": config.MaxAgeDays,
		"cutoff_time":  cutoffTime,
		"check_field":  checkField,
	})

	var totalDeleted int64

	// Handle entities
	if strategy.EntityType != nil {
		query := fmt.Sprintf(`
			UPDATE staged_entities
			SET deleted_at = NOW()
			WHERE tenant_id = $1
			  AND entity_type = $2
			  AND %s < $3
			  AND deleted_at IS NULL
		`, checkField)

		if strategy.Integration != nil {
			query = fmt.Sprintf(`
				UPDATE staged_entities
				SET deleted_at = NOW()
				WHERE tenant_id = $1
				  AND entity_type = $2
				  AND integration = $3
				  AND %s < $4
				  AND deleted_at IS NULL
			`, checkField)
			result, err := e.entityRepo.ExecRaw(ctx, query, execCtx.TenantID, *strategy.EntityType, *strategy.Integration, cutoffTime)
			if err != nil {
				log.WithError(err).Error("Failed to mark stale entities deleted")
				return totalDeleted, err
			}
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				count, _ := sqlResult.RowsAffected()
				totalDeleted += count
				log.WithFields(map[string]any{
					"entity_type": *strategy.EntityType,
					"integration": *strategy.Integration,
					"count":       count,
				}).Info("Marked stale entities deleted")
			}
		} else {
			result, err := e.entityRepo.ExecRaw(ctx, query, execCtx.TenantID, *strategy.EntityType, cutoffTime)
			if err != nil {
				log.WithError(err).Error("Failed to mark stale entities deleted")
				return totalDeleted, err
			}
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				count, _ := sqlResult.RowsAffected()
				totalDeleted += count
				log.WithFields(map[string]any{
					"entity_type": *strategy.EntityType,
					"count":       count,
				}).Info("Marked stale entities deleted")
			}
		}
	}

	// Handle relationships
	if strategy.RelationshipType != nil {
		query := fmt.Sprintf(`
			UPDATE staged_relationships
			SET deleted_at = NOW()
			WHERE tenant_id = $1
			  AND relationship_type = $2
			  AND %s < $3
			  AND deleted_at IS NULL
		`, checkField)

		if strategy.Integration != nil {
			query = fmt.Sprintf(`
				UPDATE staged_relationships
				SET deleted_at = NOW()
				WHERE tenant_id = $1
				  AND relationship_type = $2
				  AND integration = $3
				  AND %s < $4
				  AND deleted_at IS NULL
			`, checkField)
			result, err := e.relationshipRepo.ExecRaw(ctx, query, execCtx.TenantID, *strategy.RelationshipType, *strategy.Integration, cutoffTime)
			if err != nil {
				log.WithError(err).Error("Failed to mark stale relationships deleted")
				return totalDeleted, err
			}
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				count, _ := sqlResult.RowsAffected()
				totalDeleted += count
				log.WithFields(map[string]any{
					"relationship_type": *strategy.RelationshipType,
					"integration":       *strategy.Integration,
					"count":             count,
				}).Info("Marked stale relationships deleted")
			}
		} else {
			result, err := e.relationshipRepo.ExecRaw(ctx, query, execCtx.TenantID, *strategy.RelationshipType, cutoffTime)
			if err != nil {
				log.WithError(err).Error("Failed to mark stale relationships deleted")
				return totalDeleted, err
			}
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				count, _ := sqlResult.RowsAffected()
				totalDeleted += count
				log.WithFields(map[string]any{
					"relationship_type": *strategy.RelationshipType,
					"count":             count,
				}).Info("Marked stale relationships deleted")
			}
		}
	}

	return totalDeleted, nil
}

// executeRetention marks entities/relationships older than retention_days as deleted
func (e *Engine) executeRetention(ctx context.Context, strategy *models.DeletionStrategy, execCtx ExecutionContext) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.Engine.executeRetention")
	defer span.End()

	// Parse config
	var config models.RetentionConfig
	if err := json.Unmarshal(strategy.Config, &config); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to parse retention config")
		return 0, err
	}

	checkField := config.CheckField
	if checkField == "" {
		checkField = "created_at"
	}

	cutoffTime := time.Now().UTC().AddDate(0, 0, -config.RetentionDays)

	log := e.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":      execCtx.TenantID,
		"retention_days": config.RetentionDays,
		"cutoff_time":    cutoffTime,
		"check_field":    checkField,
	})

	var totalDeleted int64

	// Handle entities
	if strategy.EntityType != nil {
		query := fmt.Sprintf(`
			UPDATE staged_entities
			SET deleted_at = NOW()
			WHERE tenant_id = $1
			  AND entity_type = $2
			  AND %s < $3
			  AND deleted_at IS NULL
		`, checkField)

		if strategy.Integration != nil {
			query = fmt.Sprintf(`
				UPDATE staged_entities
				SET deleted_at = NOW()
				WHERE tenant_id = $1
				  AND entity_type = $2
				  AND integration = $3
				  AND %s < $4
				  AND deleted_at IS NULL
			`, checkField)
			result, err := e.entityRepo.ExecRaw(ctx, query, execCtx.TenantID, *strategy.EntityType, *strategy.Integration, cutoffTime)
			if err != nil {
				log.WithError(err).Error("Failed to mark old entities deleted")
				return totalDeleted, err
			}
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				count, _ := sqlResult.RowsAffected()
				totalDeleted += count
				log.WithFields(map[string]any{
					"entity_type": *strategy.EntityType,
					"integration": *strategy.Integration,
					"count":       count,
				}).Info("Marked old entities deleted (retention)")
			}
		} else {
			result, err := e.entityRepo.ExecRaw(ctx, query, execCtx.TenantID, *strategy.EntityType, cutoffTime)
			if err != nil {
				log.WithError(err).Error("Failed to mark old entities deleted")
				return totalDeleted, err
			}
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				count, _ := sqlResult.RowsAffected()
				totalDeleted += count
				log.WithFields(map[string]any{
					"entity_type": *strategy.EntityType,
					"count":       count,
				}).Info("Marked old entities deleted (retention)")
			}
		}
	}

	// Handle relationships
	if strategy.RelationshipType != nil {
		query := fmt.Sprintf(`
			UPDATE staged_relationships
			SET deleted_at = NOW()
			WHERE tenant_id = $1
			  AND relationship_type = $2
			  AND %s < $3
			  AND deleted_at IS NULL
		`, checkField)

		if strategy.Integration != nil {
			query = fmt.Sprintf(`
				UPDATE staged_relationships
				SET deleted_at = NOW()
				WHERE tenant_id = $1
				  AND relationship_type = $2
				  AND integration = $3
				  AND %s < $4
				  AND deleted_at IS NULL
			`, checkField)
			result, err := e.relationshipRepo.ExecRaw(ctx, query, execCtx.TenantID, *strategy.RelationshipType, *strategy.Integration, cutoffTime)
			if err != nil {
				log.WithError(err).Error("Failed to mark old relationships deleted")
				return totalDeleted, err
			}
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				count, _ := sqlResult.RowsAffected()
				totalDeleted += count
				log.WithFields(map[string]any{
					"relationship_type": *strategy.RelationshipType,
					"integration":       *strategy.Integration,
					"count":             count,
				}).Info("Marked old relationships deleted (retention)")
			}
		} else {
			result, err := e.relationshipRepo.ExecRaw(ctx, query, execCtx.TenantID, *strategy.RelationshipType, cutoffTime)
			if err != nil {
				log.WithError(err).Error("Failed to mark old relationships deleted")
				return totalDeleted, err
			}
			if sqlResult, ok := result.(interface{ RowsAffected() (int64, error) }); ok {
				count, _ := sqlResult.RowsAffected()
				totalDeleted += count
				log.WithFields(map[string]any{
					"relationship_type": *strategy.RelationshipType,
					"count":             count,
				}).Info("Marked old relationships deleted (retention)")
			}
		}
	}

	return totalDeleted, nil
}

// executeComposite executes a composite strategy (combination of multiple strategies)
func (e *Engine) executeComposite(ctx context.Context, strategy *models.DeletionStrategy, execCtx ExecutionContext) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.Engine.executeComposite")
	defer span.End()

	// Parse config
	var config models.CompositeConfig
	if err := json.Unmarshal(strategy.Config, &config); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to parse composite config")
		return 0, err
	}

	// For composite strategies, we'll execute each sub-strategy
	// This is a simplified implementation - you could make it more sophisticated
	// by tracking which records match each strategy and applying AND/OR logic

	var totalDeleted int64
	for _, subStrategy := range config.Strategies {
		// Create a temporary strategy object for the sub-strategy
		tempStrategy := &models.DeletionStrategy{
			ID:               strategy.ID,
			TenantID:         strategy.TenantID,
			EntityType:       strategy.EntityType,
			RelationshipType: strategy.RelationshipType,
			Integration:      strategy.Integration,
			StrategyType:     subStrategy.Type,
			Enabled:          strategy.Enabled,
		}

		// Build config for sub-strategy
		var subConfig []byte
		switch subStrategy.Type {
		case models.DeletionStrategyStaleness:
			if subStrategy.MaxAgeDays != nil {
				c := models.StalenessConfig{
					MaxAgeDays: *subStrategy.MaxAgeDays,
					CheckField: subStrategy.CheckField,
				}
				subConfig, _ = json.Marshal(c)
			}
		case models.DeletionStrategyRetention:
			if subStrategy.RetentionDays != nil {
				c := models.RetentionConfig{
					RetentionDays: *subStrategy.RetentionDays,
					CheckField:    subStrategy.CheckField,
				}
				subConfig, _ = json.Marshal(c)
			}
		}

		if subConfig != nil {
			tempStrategy.Config = subConfig
			count, err := e.ExecuteStrategy(ctx, tempStrategy, execCtx)
			if err != nil {
				e.logger.WithContext(ctx).WithError(err).Error("Failed to execute sub-strategy")
				// Continue with other strategies
				continue
			}
			totalDeleted += count
		}
	}

	return totalDeleted, nil
}
