package models

import (
	"time"

	"github.com/google/uuid"
)

// ExecutionStatus represents the status of a plan execution
type ExecutionStatus string

const (
	ExecutionStatusPending ExecutionStatus = "pending"
	ExecutionStatusRunning ExecutionStatus = "running"
	ExecutionStatusSuccess ExecutionStatus = "success"
	ExecutionStatusFailed  ExecutionStatus = "failed"
	ExecutionStatusAborted ExecutionStatus = "aborted"
)

// ErrorType represents the type of error in a failed execution
type ErrorType string

const (
	ErrorTypeTransient ErrorType = "transient"
	ErrorTypePermanent ErrorType = "permanent"
	ErrorTypeRateLimit ErrorType = "rate_limit"
)

// PlanExecution tracks an individual plan/step execution
type PlanExecution struct {
	ID                 uuid.UUID       `db:"id" json:"id"`
	TenantID           uuid.UUID       `db:"tenant_id" json:"tenant_id"`
	PlanKey            string          `db:"plan_key" json:"plan_key"`
	ConfigID           uuid.UUID       `db:"config_id" json:"config_id"`
	ParentExecutionID  *uuid.UUID      `db:"parent_execution_id" json:"parent_execution_id,omitempty"`
	Status             ExecutionStatus `db:"status" json:"status"`
	StepPath           *string         `db:"step_path" json:"step_path,omitempty"`
	StartedAt          *time.Time      `db:"started_at" json:"started_at,omitempty"`
	CompletedAt        *time.Time      `db:"completed_at" json:"completed_at,omitempty"`
	ErrorMessage       *string         `db:"error_message" json:"error_message,omitempty"`
	ErrorType          *ErrorType      `db:"error_type" json:"error_type,omitempty"`
	RetryCount         int             `db:"retry_count" json:"retry_count"`
	RequestURL         *string         `db:"request_url" json:"request_url,omitempty"`
	RequestMethod      *string         `db:"request_method" json:"request_method,omitempty"`
	ResponseStatusCode *int            `db:"response_status_code" json:"response_status_code,omitempty"`
	ResponseSizeBytes  *int64          `db:"response_size_bytes" json:"response_size_bytes,omitempty"`
	CreatedAt          time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at" json:"updated_at"`
}

// TableName returns the database table name
func (PlanExecution) TableName() string {
	return "plan_executions"
}
