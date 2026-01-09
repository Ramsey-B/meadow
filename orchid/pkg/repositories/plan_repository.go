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
	"github.com/huandu/go-sqlbuilder"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

const plansTable = "plans"

// PlanRepository handles database operations for plans
type PlanRepository struct {
	*Repository
}

// NewPlanRepository creates a new plan repository
func NewPlanRepository(db database.DB, logger ectologger.Logger) *PlanRepository {
	return &PlanRepository{
		Repository: NewRepository(db, logger),
	}
}

// Create creates a new plan
func (r *PlanRepository) Create(ctx context.Context, plan *models.Plan) error {
	ctx, span := tracing.StartSpan(ctx, "PlanRepository.Create")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}
	plan.TenantID = tenantID

	if plan.Key == "" {
		plan.Key = uuid.New().String()
	}

	now := time.Now().UTC()

	// Manual testing via .http files benefits from idempotent plan creation. Implement this as a true
	// SQL upsert on the unique key (tenant_id, integration_id, name).
	ib := sqlbuilder.PostgreSQL.NewInsertBuilder()
	ib.InsertInto(plansTable).
		Cols(
			"key", "tenant_id", "integration_id", "name", "description", "plan_definition",
			"enabled", "wait_seconds", "repeat_count", "created_at", "updated_at",
		).
		Values(
			plan.Key, plan.TenantID, plan.IntegrationID, plan.Name, plan.Description, plan.PlanDefinition,
			plan.Enabled, plan.WaitSeconds, plan.RepeatCount,
			now, now,
		)
	ib.SQL(`
ON CONFLICT (tenant_id, key)
DO UPDATE SET
  description = EXCLUDED.description,
  plan_definition = EXCLUDED.plan_definition,
  enabled = EXCLUDED.enabled,
  wait_seconds = EXCLUDED.wait_seconds,
  repeat_count = EXCLUDED.repeat_count,
  updated_at = EXCLUDED.updated_at
RETURNING key, created_at, updated_at`)

	query, args := ib.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&plan.Key, &plan.CreatedAt, &plan.UpdatedAt)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": plan.Key,
		}).Error("failed to create plan")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create plan")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key": plan.Key,
	}).Debugf("Created %s", plansTable)
	return nil
}

// GetByID retrieves a plan by ID (tenant-scoped)
func (r *PlanRepository) GetByKey(ctx context.Context, key string) (*models.Plan, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanRepository.GetByID")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": key,
		}).Error("failed to get plan")
		return nil, err
	}

	// Use raw SQL with JOIN to get integration name
	query := `
		SELECT 
			p.key, p.tenant_id, p.integration_id, i.name AS integration,
			p.name, p.description, p.plan_definition, p.enabled,
			p.wait_seconds, p.repeat_count, p.created_at, p.updated_at
		FROM plans p
		INNER JOIN integrations i ON p.tenant_id = i.tenant_id AND p.integration_id = i.id
		WHERE p.tenant_id = $1 AND p.key = $2
	`

	var plan models.Plan
	err = r.DB().GetContext(ctx, &plan, query, tenantID, key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "plan %s does not exist", key)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": key,
		}).Error("failed to get plan")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get plan")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key": key,
	}).Debugf("Got %s", plansTable)
	return &plan, nil
}

// ListByIntegration retrieves all plans for an integration
func (r *PlanRepository) ListByIntegration(ctx context.Context, integrationID uuid.UUID) ([]models.Plan, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanRepository.ListByIntegration")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"integration_id": integrationID,
		}).Error("failed to list plans")
		return nil, err
	}

	// Use raw SQL with JOIN to get integration name
	query := `
		SELECT 
			p.key, p.tenant_id, p.integration_id, i.name AS integration,
			p.name, p.description, p.plan_definition, p.enabled,
			p.wait_seconds, p.repeat_count, p.created_at, p.updated_at
		FROM plans p
		INNER JOIN integrations i ON p.tenant_id = i.tenant_id AND p.integration_id = i.id
		WHERE p.tenant_id = $1 AND p.integration_id = $2
		ORDER BY p.name
	`

	var plans []models.Plan
	err = r.DB().SelectContext(ctx, &plans, query, tenantID, integrationID)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id": tenantID,
		}).Error("failed to list plans")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list plans")
	}
	return plans, nil
}

// ListEnabled retrieves all enabled plans for the current tenant
func (r *PlanRepository) ListEnabled(ctx context.Context) ([]models.Plan, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanRepository.ListEnabled")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id": tenantID,
		}).Error("failed to list plans")
		return nil, err
	}

	// Use raw SQL with JOIN to get integration name
	query := `
		SELECT 
			p.key, p.tenant_id, p.integration_id, i.name AS integration,
			p.name, p.description, p.plan_definition, p.enabled,
			p.wait_seconds, p.repeat_count, p.created_at, p.updated_at
		FROM plans p
		INNER JOIN integrations i ON p.tenant_id = i.tenant_id AND p.integration_id = i.id
		WHERE p.tenant_id = $1 AND p.enabled = true
		ORDER BY p.name
	`

	var plans []models.Plan
	err = r.DB().SelectContext(ctx, &plans, query, tenantID)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id": tenantID,
		}).Error("failed to list plans")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list plans")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
	}).Debugf("Listed %s", plansTable)
	return plans, nil
}

// Update updates an existing plan
func (r *PlanRepository) Update(ctx context.Context, plan *models.Plan) error {
	ctx, span := tracing.StartSpan(ctx, "PlanRepository.Update")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	ub := database.NewUpdateBuilder()
	ub.Update(plansTable).
		Set(
			ub.Assign("name", plan.Name),
			ub.Assign("description", plan.Description),
			ub.Assign("plan_definition", plan.PlanDefinition),
			ub.Assign("enabled", plan.Enabled),
			ub.Assign("wait_seconds", plan.WaitSeconds),
			ub.Assign("repeat_count", plan.RepeatCount),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("key", plan.Key))
	ub.SQL("RETURNING updated_at")

	query, args := ub.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&plan.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "plan %s does not exist", plan.Key)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": plan.Key,
		}).Error("failed to update plan")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update plan")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key": plan.Key,
	}).Debugf("Updated %s", plansTable)
	return nil
}

// SetEnabled enables or disables a plan
func (r *PlanRepository) SetEnabled(ctx context.Context, key string, enabled bool) error {
	ctx, span := tracing.StartSpan(ctx, "PlanRepository.SetEnabled")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	ub := database.NewUpdateBuilder()
	ub.Update(plansTable).
		Set(
			ub.Assign("enabled", enabled),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("key", key))

	query, args := ub.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": key,
		}).Error("failed to set enabled")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to set enabled")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": key,
		}).Error("failed to set enabled")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to set enabled")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "plan %s does not exist", key)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key": key,
	}).Debugf("Updated %s", plansTable)
	return nil
}

// Delete deletes a plan by ID
func (r *PlanRepository) Delete(ctx context.Context, key string) error {
	ctx, span := tracing.StartSpan(ctx, "PlanRepository.Delete")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	db := database.NewDeleteBuilder()
	db.DeleteFrom(plansTable).
		Where(db.Equal("tenant_id", tenantID), db.Equal("key", key))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": key,
		}).Error("failed to delete plan")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete plan")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": key,
		}).Error("failed to delete plan")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete plan")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "plan %s does not exist", key)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key": key,
	}).Debugf("Deleted %s", plansTable)
	return nil
}

// DeleteByTenantID deletes all plans for a tenant (for testing cleanup)
func (r *PlanRepository) DeleteByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanRepository.DeleteByTenantID")
	defer span.End()

	db := database.NewDeleteBuilder()
	db.DeleteFrom(plansTable).
		Where(db.Equal("tenant_id", tenantID))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id": tenantID,
		}).Error("failed to delete plans by tenant")
		return 0, err
	}

	rows, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
		"count":     rows,
	}).Info("Deleted plans by tenant")
	return rows, nil
}
