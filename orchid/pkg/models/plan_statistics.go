package models

import (
	"time"

	"github.com/google/uuid"
)

// PlanStatistics holds aggregated statistics for a plan/config combination
type PlanStatistics struct {
	ID                     uuid.UUID  `db:"id" json:"id"`
	TenantID               uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	PlanKey                string     `db:"plan_key" json:"plan_key"`
	ConfigID               uuid.UUID  `db:"config_id" json:"config_id"`
	LastExecutionAt        *time.Time `db:"last_execution_at" json:"last_execution_at,omitempty"`
	LastSuccessAt          *time.Time `db:"last_success_at" json:"last_success_at,omitempty"`
	LastFailureAt          *time.Time `db:"last_failure_at" json:"last_failure_at,omitempty"`
	TotalExecutions        int64      `db:"total_executions" json:"total_executions"`
	TotalSuccesses         int64      `db:"total_successes" json:"total_successes"`
	TotalFailures          int64      `db:"total_failures" json:"total_failures"`
	TotalAPICalls          int64      `db:"total_api_calls" json:"total_api_calls"`
	AverageExecutionTimeMs *int       `db:"average_execution_time_ms" json:"average_execution_time_ms,omitempty"`
	CreatedAt              time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at" json:"updated_at"`
}

// TableName returns the database table name
func (PlanStatistics) TableName() string {
	return "plan_statistics"
}
