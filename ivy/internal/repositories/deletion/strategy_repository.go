package deletion

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"
	"github.com/huandu/go-sqlbuilder"

	"github.com/Ramsey-B/ivy/pkg/models"
)

// StrategyRepository handles deletion strategy persistence
type StrategyRepository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewStrategyRepository creates a new deletion strategy repository
func NewStrategyRepository(db database.DB, logger ectologger.Logger) *StrategyRepository {
	return &StrategyRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new deletion strategy
func (r *StrategyRepository) Create(ctx context.Context, strategy *models.DeletionStrategy) (*models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.StrategyRepository.Create")
	defer span.End()

	if strategy.ID == uuid.Nil {
		strategy.ID = uuid.New()
	}
	strategy.CreatedAt = time.Now().UTC()
	strategy.UpdatedAt = strategy.CreatedAt

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("deletion_strategies")
	sb.Cols("id", "tenant_id", "source_type", "entity_type", "strategy", "grace_period_hours", "retention_days", "enabled", "created_at", "updated_at")
	sb.Values(strategy.ID, strategy.TenantID, strategy.SourceType, strategy.EntityType, strategy.Strategy, strategy.GracePeriodHours, strategy.RetentionDays, strategy.Enabled, strategy.CreatedAt, strategy.UpdatedAt)

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to create deletion strategy")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to create deletion strategy")
	}

	return strategy, nil
}

// GetBySource gets the deletion strategy for a source type and entity type
func (r *StrategyRepository) GetBySource(ctx context.Context, tenantID, sourceType, entityType string) (*models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.StrategyRepository.GetBySource")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "source_type", "entity_type", "strategy", "grace_period_hours", "retention_days", "enabled", "created_at", "updated_at")
	sb.From("deletion_strategies")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.Equal("source_type", sourceType),
		sb.Equal("entity_type", entityType),
	)

	query, args := sb.Build()
	var strategy models.DeletionStrategy
	if err := r.db.GetContext(ctx, &strategy, query, args...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // No strategy defined
		}
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get deletion strategy")
	}

	return &strategy, nil
}

// ListByTenant lists all deletion strategies for a tenant
func (r *StrategyRepository) ListByTenant(ctx context.Context, tenantID string) ([]models.DeletionStrategy, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.StrategyRepository.ListByTenant")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "source_type", "entity_type", "strategy", "grace_period_hours", "retention_days", "enabled", "created_at", "updated_at")
	sb.From("deletion_strategies")
	sb.Where(sb.Equal("tenant_id", tenantID))
	sb.OrderBy("source_type", "entity_type")

	query, args := sb.Build()
	var strategies []models.DeletionStrategy
	if err := r.db.SelectContext(ctx, &strategies, query, args...); err != nil {
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list deletion strategies")
	}

	return strategies, nil
}

// Delete deletes a deletion strategy
func (r *StrategyRepository) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.StrategyRepository.Delete")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewDeleteBuilder()
	sb.DeleteFrom("deletion_strategies")
	sb.Where(
		sb.Equal("id", id),
		sb.Equal("tenant_id", tenantID),
	)

	query, args := sb.Build()
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete strategy")
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return httperror.NewHTTPError(http.StatusNotFound, fmt.Sprintf("deletion strategy %s not found", id))
	}

	return nil
}

