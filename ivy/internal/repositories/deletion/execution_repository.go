package deletion

import (
	"context"
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

// ExecutionRepository handles execution tracking persistence
type ExecutionRepository struct {
	db     database.DB
	logger ectologger.Logger
}

// NewExecutionRepository creates a new execution tracking repository
func NewExecutionRepository(db database.DB, logger ectologger.Logger) *ExecutionRepository {
	return &ExecutionRepository{
		db:     db,
		logger: logger,
	}
}

// RecordExecution records an execution completion
func (r *ExecutionRepository) RecordExecution(ctx context.Context, tracking *models.ExecutionTracking) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.ExecutionRepository.RecordExecution")
	defer span.End()

	if tracking.ID == "" {
		tracking.ID = uuid.New().String()
	}

	sb := sqlbuilder.PostgreSQL.NewInsertBuilder()
	sb.InsertInto("execution_tracking")
	sb.Cols("id", "tenant_id", "execution_id", "plan_id", "entity_type", "entity_count", "started_at", "completed_at")
	sb.Values(tracking.ID, tracking.TenantID, tracking.ExecutionID, tracking.PlanID, tracking.EntityType, tracking.EntityCount, tracking.StartedAt, tracking.CompletedAt)

	query, args := sb.Build()
	query += " ON CONFLICT (tenant_id, execution_id, entity_type) DO UPDATE SET entity_count = EXCLUDED.entity_count, completed_at = EXCLUDED.completed_at"

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to record execution")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to record execution")
	}

	return nil
}

// GetPreviousExecution gets the previous execution for a plan/entity type
func (r *ExecutionRepository) GetPreviousExecution(ctx context.Context, tenantID, planID, entityType, currentExecutionID string) (*models.ExecutionTracking, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.ExecutionRepository.GetPreviousExecution")
	defer span.End()

	query := `
		SELECT id, tenant_id, execution_id, plan_id, entity_type, entity_count, started_at, completed_at, processed_at
		FROM execution_tracking
		WHERE tenant_id = $1 AND plan_id = $2 AND entity_type = $3 AND execution_id != $4
		ORDER BY completed_at DESC
		LIMIT 1
	`

	var tracking models.ExecutionTracking
	if err := r.db.GetContext(ctx, &tracking, query, tenantID, planID, entityType, currentExecutionID); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil // No previous execution
		}
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get previous execution")
	}

	return &tracking, nil
}

// GetUnprocessedExecutions gets executions that haven't been processed for deletions
func (r *ExecutionRepository) GetUnprocessedExecutions(ctx context.Context, tenantID string) ([]models.ExecutionTracking, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.ExecutionRepository.GetUnprocessedExecutions")
	defer span.End()

	sb := sqlbuilder.PostgreSQL.NewSelectBuilder()
	sb.Select("id", "tenant_id", "execution_id", "plan_id", "entity_type", "entity_count", "started_at", "completed_at", "processed_at")
	sb.From("execution_tracking")
	sb.Where(
		sb.Equal("tenant_id", tenantID),
		sb.IsNull("processed_at"),
	)
	sb.OrderBy("completed_at ASC")

	query, args := sb.Build()
	var trackings []models.ExecutionTracking
	if err := r.db.SelectContext(ctx, &trackings, query, args...); err != nil {
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get unprocessed executions")
	}

	return trackings, nil
}

// MarkProcessed marks an execution as processed
func (r *ExecutionRepository) MarkProcessed(ctx context.Context, id string) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.ExecutionRepository.MarkProcessed")
	defer span.End()

	now := time.Now().UTC()
	sb := sqlbuilder.PostgreSQL.NewUpdateBuilder()
	sb.Update("execution_tracking")
	sb.Set(sb.Assign("processed_at", now))
	sb.Where(sb.Equal("id", id))

	query, args := sb.Build()
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark execution processed")
	}

	return nil
}

