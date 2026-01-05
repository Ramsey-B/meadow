package repositories

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

const planContextsTable = "plan_contexts"

var planContextStruct = database.NewStruct(new(models.PlanContext))

// PlanContextRepository handles database operations for plan contexts
type PlanContextRepository struct {
	*Repository
}

// NewPlanContextRepository creates a new plan context repository
func NewPlanContextRepository(db database.DB, logger ectologger.Logger) *PlanContextRepository {
	return &PlanContextRepository{
		Repository: NewRepository(db, logger),
	}
}

// Upsert creates or updates a plan context
// Using raw SQL for complex upsert with ON CONFLICT
func (r *PlanContextRepository) Upsert(ctx context.Context, planCtx *models.PlanContext) error {
	ctx, span := tracing.StartSpan(ctx, "PlanContextRepository.Upsert")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}
	planCtx.TenantID = tenantID

	if planCtx.ID == uuid.Nil {
		planCtx.ID = uuid.New()
	}

	now := time.Now()

	// Use parameterized timestamp instead of NOW() for Citus compatibility
	query := `
		INSERT INTO plan_contexts (id, tenant_id, plan_key, config_id, context_data, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (tenant_id, plan_key, config_id)
		DO UPDATE SET context_data = $5, updated_at = $6
		RETURNING id, created_at, updated_at`

	err = r.DB().QueryRowContext(ctx, query,
		planCtx.ID,
		planCtx.TenantID,
		planCtx.PlanKey,
		planCtx.ConfigID,
		planCtx.ContextData,
		now,
	).Scan(&planCtx.ID, &planCtx.CreatedAt, &planCtx.UpdatedAt)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planCtx.PlanKey,
			"config_id": planCtx.ConfigID,
		}).Error("failed to upsert plan context")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to upsert plan context")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key":  planCtx.PlanKey,
		"config_id": planCtx.ConfigID,
	}).Infof("Upserted %s for plan=%s config=%s", planContextsTable, planCtx.PlanKey, planCtx.ConfigID)
	return nil
}

// GetByPlanAndConfig retrieves a plan context by plan and config IDs
func (r *PlanContextRepository) GetByPlanAndConfig(ctx context.Context, planKey string, configID uuid.UUID) (*models.PlanContext, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanContextRepository.GetByPlanAndConfig")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := planContextStruct.SelectFrom(planContextsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("plan_key", planKey), sb.Equal("config_id", configID))

	query, args := sb.Build()
	var planCtx models.PlanContext
	err = r.DB().GetContext(ctx, &planCtx, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "context for plan %s with config %s does not exist", planKey, configID)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Error("failed to get plan context")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get plan context")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key":  planKey,
		"config_id": configID,
	}).Debugf("Retrieved %s for plan=%s config=%s", planContextsTable, planKey, configID)
	return &planCtx, nil
}

// ListByPlan retrieves all contexts for a plan
func (r *PlanContextRepository) ListByPlan(ctx context.Context, planKey string) ([]models.PlanContext, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanContextRepository.ListByPlan")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	sb := planContextStruct.SelectFrom(planContextsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("plan_key", planKey))
	sb.OrderBy("created_at")

	query, args := sb.Build()
	var contexts []models.PlanContext
	err = r.DB().SelectContext(ctx, &contexts, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": planKey,
		}).Error("failed to list contexts")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list contexts")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key": planKey,
	}).Debugf("Listed %d contexts for plan=%s", len(contexts), planKey)
	return contexts, nil
}

// Delete deletes a plan context
func (r *PlanContextRepository) Delete(ctx context.Context, planKey string, configID uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "PlanContextRepository.Delete")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	db := database.NewDeleteBuilder()
	db.DeleteFrom(planContextsTable).
		Where(db.Equal("tenant_id", tenantID), db.Equal("plan_key", planKey), db.Equal("config_id", configID))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Error("failed to delete plan context")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete plan context")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Error("failed to delete plan context")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete plan context")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "context for plan %s with config %s does not exist", planKey, configID)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key":  planKey,
		"config_id": configID,
	}).Infof("Deleted %s for plan=%s config=%s", planContextsTable, planKey, configID)
	return nil
}
