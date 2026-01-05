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

const planStatisticsTable = "plan_statistics"

var planStatisticsStruct = database.NewStruct(new(models.PlanStatistics))

// PlanStatisticsRepository handles database operations for plan statistics
type PlanStatisticsRepository struct {
	*Repository
}

// NewPlanStatisticsRepository creates a new plan statistics repository
func NewPlanStatisticsRepository(db database.DB, logger ectologger.Logger) *PlanStatisticsRepository {
	return &PlanStatisticsRepository{
		Repository: NewRepository(db, logger),
	}
}

// GetOrCreate retrieves or creates statistics for a plan/config combination
// Using raw SQL for complex upsert
func (r *PlanStatisticsRepository) GetOrCreate(ctx context.Context, planKey string, configID uuid.UUID) (*models.PlanStatistics, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanStatisticsRepository.GetOrCreate")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Error("failed to get or create plan statistics")
		return nil, err
	}

	now := time.Now()

	// Use parameterized timestamp instead of NOW() for Citus compatibility
	query := `
		INSERT INTO plan_statistics (id, tenant_id, plan_key, config_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (tenant_id, plan_key, config_id)
		DO UPDATE SET updated_at = plan_statistics.updated_at
		RETURNING id, tenant_id, plan_key, config_id, last_execution_at, last_success_at,
				  last_failure_at, total_executions, total_successes, total_failures,
				  total_api_calls, average_execution_time_ms, created_at, updated_at`

	var stats models.PlanStatistics
	err = r.DB().QueryRowContext(ctx, query,
		uuid.New(),
		tenantID,
		planKey,
		configID,
		now,
	).Scan(
		&stats.ID,
		&stats.TenantID,
		&stats.PlanKey,
		&stats.ConfigID,
		&stats.LastExecutionAt,
		&stats.LastSuccessAt,
		&stats.LastFailureAt,
		&stats.TotalExecutions,
		&stats.TotalSuccesses,
		&stats.TotalFailures,
		&stats.TotalAPICalls,
		&stats.AverageExecutionTimeMs,
		&stats.CreatedAt,
		&stats.UpdatedAt,
	)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Error("failed to get or create plan statistics")
		return nil, err
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key":  planKey,
		"config_id": configID,
	}).Debugf("Got or created %s for plan=%s config=%s", planStatisticsTable, planKey, configID)
	return &stats, nil
}

// GetByPlanAndConfig retrieves statistics by plan and config IDs
func (r *PlanStatisticsRepository) GetByPlanAndConfig(ctx context.Context, planKey string, configID uuid.UUID) (*models.PlanStatistics, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanStatisticsRepository.GetByPlanAndConfig")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Error("failed to get plan statistics")
		return nil, err
	}

	sb := planStatisticsStruct.SelectFrom(planStatisticsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("plan_key", planKey), sb.Equal("config_id", configID))

	query, args := sb.Build()
	var stats models.PlanStatistics
	err = r.DB().GetContext(ctx, &stats, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperror.NewHTTPErrorf(http.StatusNotFound, "statistics for plan %s with config %s do not exist", planKey, configID)
	}
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Error("failed to get plan statistics")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to get plan statistics")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key":  planKey,
		"config_id": configID,
	}).Debugf("Retrieved %s for plan=%s config=%s", planStatisticsTable, planKey, configID)
	return &stats, nil
}

// RecordExecution records an execution and updates statistics
// Using raw SQL for complex upsert with arithmetic
func (r *PlanStatisticsRepository) RecordExecution(ctx context.Context, planKey string, configID uuid.UUID, success bool, executionTimeMs int) error {
	ctx, span := tracing.StartSpan(ctx, "PlanStatisticsRepository.RecordExecution")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	now := time.Now()

	// Use parameterized timestamp instead of NOW() for Citus compatibility
	var query string
	if success {
		query = `
			INSERT INTO plan_statistics (id, tenant_id, plan_key, config_id, 
				last_execution_at, last_success_at, total_executions, total_successes,
				average_execution_time_ms, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5, 1, 1, $6, $7, $7)
			ON CONFLICT (tenant_id, plan_key, config_id)
			DO UPDATE SET
				last_execution_at = $5,
				last_success_at = $5,
				total_executions = plan_statistics.total_executions + 1,
				total_successes = plan_statistics.total_successes + 1,
				average_execution_time_ms = (
					COALESCE(plan_statistics.average_execution_time_ms, 0) * plan_statistics.total_executions + $6
				) / (plan_statistics.total_executions + 1),
				updated_at = $7`
	} else {
		query = `
			INSERT INTO plan_statistics (id, tenant_id, plan_key, config_id,
				last_execution_at, last_failure_at, total_executions, total_failures,
				average_execution_time_ms, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5, 1, 1, $6, $7, $7)
			ON CONFLICT (tenant_id, plan_key, config_id)
			DO UPDATE SET
				last_execution_at = $5,
				last_failure_at = $5,
				total_executions = plan_statistics.total_executions + 1,
				total_failures = plan_statistics.total_failures + 1,
				average_execution_time_ms = (
					COALESCE(plan_statistics.average_execution_time_ms, 0) * plan_statistics.total_executions + $6
				) / (plan_statistics.total_executions + 1),
				updated_at = $7`
	}

	_, err = r.DB().ExecContext(ctx, query, uuid.New(), tenantID, planKey, configID, now, executionTimeMs, now)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":          planKey,
			"config_id":         configID,
			"success":           success,
			"execution_time_ms": executionTimeMs,
		}).Error("failed to record execution")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to record execution")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key":  planKey,
		"config_id": configID,
		"success":   success,
	}).Debugf("Recorded execution for %s plan=%s config=%s success=%v", planStatisticsTable, planKey, configID, success)
	return nil
}

// ListByPlan retrieves all statistics for a plan
func (r *PlanStatisticsRepository) ListByPlan(ctx context.Context, planKey string) ([]models.PlanStatistics, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanStatisticsRepository.ListByPlan")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": planKey,
		}).Error("failed to list plan statistics")
		return nil, err
	}

	sb := planStatisticsStruct.SelectFrom(planStatisticsTable)
	sb.Where(sb.Equal("tenant_id", tenantID), sb.Equal("plan_key", planKey))
	sb.OrderBy("last_execution_at").Desc()

	query, args := sb.Build()
	var stats []models.PlanStatistics
	err = r.DB().SelectContext(ctx, &stats, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key": planKey,
		}).Error("failed to list plan statistics")
		return nil, httperror.NewHTTPError(http.StatusInternalServerError, "failed to list plan statistics")
	}

	return stats, nil
}

// Delete deletes statistics for a plan/config combination
func (r *PlanStatisticsRepository) Delete(ctx context.Context, planKey string, configID uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "PlanStatisticsRepository.Delete")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to delete plan statistics")
	}

	db := database.NewDeleteBuilder()
	db.DeleteFrom(planStatisticsTable).
		Where(db.Equal("tenant_id", tenantID), db.Equal("plan_key", planKey), db.Equal("config_id", configID))

	query, args := db.Build()
	result, err := r.DB().ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Errorf("Failed to delete %s", planStatisticsTable)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
		}).Errorf("Failed to delete %s", planStatisticsTable)
		return err
	}
	if rows == 0 {
		return httperror.NewHTTPErrorf(http.StatusNotFound, "statistics for plan %s with config %s do not exist", planKey, configID)
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key":  planKey,
		"config_id": configID,
	}).Infof("Deleted %s", planStatisticsTable)
	return nil
}

func (r *PlanStatisticsRepository) IncrementAPICalls(ctx context.Context, planKey string, configID uuid.UUID, count int) error {
	ctx, span := tracing.StartSpan(ctx, "PlanStatisticsRepository.IncrementAPICalls")
	defer span.End()

	tenantID, err := GetTenantID(ctx)
	if err != nil {
		return err
	}

	now := time.Now()

	query := `
		INSERT INTO plan_statistics (id, tenant_id, plan_key, config_id, total_api_calls, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (tenant_id, plan_key, config_id)
		DO UPDATE SET
		total_api_calls = plan_statistics.total_api_calls + $5,
		updated_at = $6`

	_, err = r.DB().ExecContext(ctx, query, uuid.New(), tenantID, planKey, configID, count, now, now)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"plan_key":  planKey,
			"config_id": configID,
			"count":     count,
		}).Error("failed to increment API calls")
		return httperror.NewHTTPError(http.StatusInternalServerError, "failed to increment API calls")
	}

	r.logger.WithContext(ctx).WithFields(map[string]any{
		"plan_key":  planKey,
		"config_id": configID,
		"count":     count,
	}).Debugf("Incremented API calls for %s plan=%s config=%s count=%d", planStatisticsTable, planKey, configID, count)
	return nil
}
