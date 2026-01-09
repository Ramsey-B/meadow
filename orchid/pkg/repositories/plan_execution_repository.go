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

const planExecutionsTable = "plan_executions"

var planExecutionStruct = database.NewStruct(new(models.PlanExecution))

// PlanExecutionRepository handles database operations for plan executions
type PlanExecutionRepository struct {
	*Repository
}

// NewPlanExecutionRepository creates a new plan execution repository
func NewPlanExecutionRepository(db database.DB, logger ectologger.Logger) *PlanExecutionRepository {
	return &PlanExecutionRepository{
		Repository: NewRepository(db, logger),
	}
}

// Create creates a new plan execution
func (r *PlanExecutionRepository) Create(ctx context.Context, execution *models.PlanExecution) error {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.Create")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": execution.ID,
		}).Error("failed to get tenant ID")
		return err
	}
	execution.TenantID = tenantID

	if execution.ID == uuid.Nil {
		execution.ID = uuid.New()
	}

	ib := database.NewInsertBuilder()
	ib.InsertInto(planExecutionsTable).
		Cols("id", "tenant_id", "plan_key", "config_id", "parent_execution_id",
			"status", "step_path", "started_at", "completed_at", "error_message", "error_type",
			"retry_count", "request_url", "request_method", "response_status_code",
			"response_size_bytes", "created_at", "updated_at").
		Values(execution.ID, execution.TenantID, execution.PlanKey, execution.ConfigID, execution.ParentExecutionID,
			execution.Status, execution.StepPath, execution.StartedAt, execution.CompletedAt, execution.ErrorMessage, execution.ErrorType,
			execution.RetryCount, execution.RequestURL, execution.RequestMethod, execution.ResponseStatusCode,
			execution.ResponseSizeBytes, sqlbuilder.Raw("NOW()"), sqlbuilder.Raw("NOW()")).
		Returning("created_at", "updated_at")

	query, args := ib.Build()
	err = r.DB().QueryRowContext(ctx, query, args...).Scan(&execution.CreatedAt, &execution.UpdatedAt)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": execution.ID,
		}).Error("failed to create execution")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to create execution")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"execution_id": execution.ID,
	}).Debugf("Created %s", planExecutionsTable)
	return nil
}

// GetByID retrieves a plan execution by ID (tenant-scoped)
func (r *PlanExecutionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PlanExecution, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.GetByID")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to get tenant ID")
		return nil, err
	}

	sb := planExecutionStruct.SelectFrom(planExecutionsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("id", id))

	query, args := sb.Build()
	var execution models.PlanExecution
	err = r.DB().GetContext(ctx, &execution, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "execution %s does not exist", id)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to get execution")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get execution")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"execution_id": id,
	}).Debugf("Got %s", planExecutionsTable)
	return &execution, nil
}

// ListByPlan retrieves executions for a plan
func (r *PlanExecutionRepository) ListByPlan(ctx context.Context, planKey string, limit int) ([]models.PlanExecution, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.ListByPlan")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": planKey,
		}).Error("failed to get tenant ID")
		return nil, err
	}

	sb := planExecutionStruct.SelectFrom(planExecutionsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("plan_key", planKey))
	sb.OrderBy("created_at").Desc()
	sb.Limit(limit)

	query, args := sb.Build()
	var executions []models.PlanExecution
	err = r.DB().SelectContext(ctx, &executions, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": planKey,
		}).Error("failed to list by plan")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list by plan")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key": planKey,
	}).Debugf("Listed %d by plan %s", len(executions), planKey)
	return executions, nil
}

// ListByStatus retrieves executions by status
func (r *PlanExecutionRepository) ListByStatus(ctx context.Context, status models.ExecutionStatus, limit int) ([]models.PlanExecution, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.ListByStatus")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"status": status,
		}).Error("failed to get tenant ID")
		return nil, err
	}

	sb := planExecutionStruct.SelectFrom(planExecutionsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("status", status))
	sb.OrderBy("created_at").Desc()
	sb.Limit(limit)

	query, args := sb.Build()
	var executions []models.PlanExecution
	err = r.DB().SelectContext(ctx, &executions, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"status": status,
		}).Error("failed to list by status")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list by status")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"status": status,
	}).Debugf("Listed %d by status %s", len(executions), status)
	return executions, nil
}

// ListChildren retrieves child executions for a parent
func (r *PlanExecutionRepository) ListChildren(ctx context.Context, parentID uuid.UUID) ([]models.PlanExecution, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.ListChildren")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": parentID,
		}).Error("failed to get tenant ID")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list children")
	}

	sb := planExecutionStruct.SelectFrom(planExecutionsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("parent_execution_id", parentID))
	sb.OrderBy("created_at")

	query, args := sb.Build()
	var executions []models.PlanExecution
	err = r.DB().SelectContext(ctx, &executions, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": parentID,
		}).Error("failed to list children")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list children")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"execution_id": parentID,
	}).Debugf("Listed children of %s", planExecutionsTable)
	return executions, nil
}

// UpdateStatus updates the status of an execution
func (r *PlanExecutionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExecutionStatus) error {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.UpdateStatus")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to get tenant ID")
		return err
	}

	ub := database.NewUpdateBuilder()
	ub.Update(planExecutionsTable).
		Set(
			ub.Assign("status", status),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("id", id))

	query, args := ub.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to update status")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update status")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to update status")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to update status")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "execution %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"execution_id": id,
	}).Debugf("Updated status of %s to %s", planExecutionsTable, status)
	return nil
}

// MarkStarted marks an execution as started
func (r *PlanExecutionRepository) MarkStarted(ctx context.Context, id uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.MarkStarted")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to get tenant ID")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark started")
	}

	now := time.Now()
	ub := database.NewUpdateBuilder()
	ub.Update(planExecutionsTable).
		Set(
			ub.Assign("status", models.ExecutionStatusRunning),
			ub.Assign("started_at", now),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("id", id))

	query, args := ub.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to mark started")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark started")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to mark started")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark started")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "execution %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"execution_id": id,
	}).Infof("Marked %s as started", planExecutionsTable)
	return nil
}

// MarkCompleted marks an execution as completed (success or failure)
func (r *PlanExecutionRepository) MarkCompleted(ctx context.Context, id uuid.UUID, status models.ExecutionStatus, errorMsg *string, errorType *models.ErrorType) error {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.MarkCompleted")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to get tenant ID")
		return err
	}

	now := time.Now()
	ub := database.NewUpdateBuilder()
	ub.Update(planExecutionsTable).
		Set(
			ub.Assign("status", status),
			ub.Assign("completed_at", now),
			ub.Assign("error_message", errorMsg),
			ub.Assign("error_type", errorType),
			ub.Assign("updated_at", sqlbuilder.Raw("NOW()")),
		).
		Where(ub.Equal("tenant_id", tenantID), ub.Equal("id", id))

	query, args := ub.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to mark completed")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark completed")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to mark completed")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to mark completed")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "execution %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"execution_id": id,
	}).Infof("Marked %s as %s", planExecutionsTable, status)
	return nil
}

// IncrementRetry increments the retry count
func (r *PlanExecutionRepository) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.IncrementRetry")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to get tenant ID")
		return err
	}

	// Using raw SQL for increment since sqlbuilder doesn't have a nice way to do this
	query := `
		UPDATE plan_executions
		SET retry_count = retry_count + 1, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2`

	result, err := r.DB().ExecContext(ctx, query, tenantID, id)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to increment retry count")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to increment retry count")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to increment retry count")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to increment retry count")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "execution %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"execution_id": id,
	}).Debugf("Incremented retry count for %s", planExecutionsTable)
	return nil
}

// Delete deletes a plan execution by ID
func (r *PlanExecutionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.Delete")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to get tenant ID")
		return err
	}

	db := database.NewDeleteBuilder()
	db.DeleteFrom(planExecutionsTable).
		Where(db.Equal("tenant_id", tenantID), db.Equal("id", id))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to delete execution")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete execution")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"execution_id": id,
		}).Error("failed to delete execution")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete execution")
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "execution %s does not exist", id)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"execution_id": id,
	}).Debugf("Deleted %s", planExecutionsTable)
	return nil
}

// DeleteByTenantID deletes all executions for a tenant (for testing cleanup)
func (r *PlanExecutionRepository) DeleteByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutionRepository.DeleteByTenantID")
	defer span.End()

	db := database.NewDeleteBuilder()
	db.DeleteFrom(planExecutionsTable).
		Where(db.Equal("tenant_id", tenantID))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"tenant_id": tenantID,
		}).Error("failed to delete executions by tenant")
		return 0, err
	}

	rows, _ := result.RowsAffected()
	r.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
		"count":     rows,
	}).Info("Deleted executions by tenant")
	return rows, nil
}
