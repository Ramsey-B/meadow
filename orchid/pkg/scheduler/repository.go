package scheduler

import (
	"context"
	"time"

	"github.com/Gobusters/ectologger"

	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

const (
	// DefaultWaitSeconds is the default wait time between executions if not specified
	DefaultWaitSeconds = 300 // 5 minutes
)

// SchedulerRepositoryImpl implements SchedulerRepository with cross-tenant access
// This is a system-level repository not scoped to a single tenant
type SchedulerRepositoryImpl struct {
	db     database.DB
	logger ectologger.Logger
}

// NewSchedulerRepository creates a new scheduler repository
func NewSchedulerRepository(db database.DB, logger ectologger.Logger) *SchedulerRepositoryImpl {
	return &SchedulerRepositoryImpl{
		db:     db,
		logger: logger,
	}
}

// ListSchedulablePlans returns all enabled plan+config combinations that are due for execution
// This query is complex:
// 1. Finds all enabled plans
// 2. Joins with configs (via integration_id) to find valid configs
// 3. Left joins with plan_statistics to get last execution time
// 4. Filters to only include plans that are due (last_execution + wait_seconds < now OR never executed)
func (r *SchedulerRepositoryImpl) ListSchedulablePlans(ctx context.Context, limit int) ([]SchedulablePlan, error) {
	ctx, span := tracing.StartSpan(ctx, "SchedulerRepository.ListSchedulablePlans")
	defer span.End()

	// This query:
	// 1. Joins plans -> configs (via integration_id)
	// 2. Left joins plan_statistics to get last_execution_at
	// 3. Filters for enabled plans and configs
	// 4. Filters for plans that are due (never executed OR last_execution + wait_seconds < now)
	query := `
		SELECT 
			p.tenant_id,
			i.name AS integration,
			p.key AS plan_key,
			c.id AS config_id,
			i.id AS integration_id,
			COALESCE(p.wait_seconds, $1) AS wait_seconds,
			ps.last_execution_at
		FROM plans p
		INNER JOIN configs c ON p.tenant_id = c.tenant_id AND p.integration_id = c.integration_id AND c.enabled = true
		INNER JOIN integrations i ON p.tenant_id = i.tenant_id AND p.integration_id = i.id
		LEFT JOIN plan_statistics ps ON p.tenant_id = ps.tenant_id AND p.key = ps.plan_key AND c.id = ps.config_id
		WHERE p.enabled = true
		AND (
			ps.last_execution_at IS NULL
			OR ps.last_execution_at + (COALESCE(p.wait_seconds, $1) * INTERVAL '1 second') < NOW()
		)
		ORDER BY ps.last_execution_at ASC NULLS FIRST
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, DefaultWaitSeconds, limit)
	if err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Failed to query schedulable plans")
		return nil, err
	}
	defer rows.Close()

	var plans []SchedulablePlan
	for rows.Next() {
		var plan SchedulablePlan
		var lastExec *time.Time

		err := rows.Scan(
			&plan.TenantID,
			&plan.Integration,
			&plan.PlanKey,
			&plan.ConfigID,
			&plan.IntegrationID,
			&plan.WaitSeconds,
			&lastExec,
		)
		if err != nil {
			r.logger.WithContext(ctx).WithError(err).Error("Failed to scan schedulable plan")
			continue
		}

		plan.LastExecutionAt = lastExec
		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		r.logger.WithContext(ctx).WithError(err).Error("Error iterating schedulable plans")
		return nil, err
	}

	r.logger.WithContext(ctx).Debugf("Found %d schedulable plans", len(plans))
	return plans, nil
}
