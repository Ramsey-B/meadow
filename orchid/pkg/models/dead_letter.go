package models

import (
	"time"

	"github.com/google/uuid"
)

// DeadLetterReason represents why a job was sent to the DLQ
type DeadLetterReason string

const (
	DLQReasonMaxRetries   DeadLetterReason = "max_retries_exceeded"
	DLQReasonInvalidJob   DeadLetterReason = "invalid_job"
	DLQReasonPlanNotFound DeadLetterReason = "plan_not_found"
	DLQReasonConfigError  DeadLetterReason = "config_error"
	DLQReasonAuthError    DeadLetterReason = "auth_error"
	DLQReasonTimeout      DeadLetterReason = "timeout"
	DLQReasonPanic        DeadLetterReason = "panic"
	DLQReasonUnknown      DeadLetterReason = "unknown"
)

// DeadLetterJob represents a job that failed and was moved to the dead letter queue
type DeadLetterJob struct {
	ID           uuid.UUID        `json:"id" db:"id"`
	TenantID     uuid.UUID        `json:"tenant_id" db:"tenant_id"`
	PlanKey      string           `json:"plan_key" db:"plan_key"`
	ConfigID     uuid.UUID        `json:"config_id" db:"config_id"`
	ExecutionID  uuid.UUID        `json:"execution_id,omitempty" db:"execution_id"`
	OriginalJob  string           `json:"original_job" db:"original_job"` // JSON of original job message
	Reason       DeadLetterReason `json:"reason" db:"reason"`
	ErrorMessage string           `json:"error_message" db:"error_message"`
	RetryCount   int              `json:"retry_count" db:"retry_count"`
	CreatedAt    time.Time        `json:"created_at" db:"created_at"`
	ProcessedAt  *time.Time       `json:"processed_at,omitempty" db:"processed_at"` // When it was retried/deleted
	ProcessedBy  string           `json:"processed_by,omitempty" db:"processed_by"` // Who processed it (user/system)
}

// DeadLetterJobStatus represents the processing status
type DeadLetterJobStatus string

const (
	DLQStatusPending   DeadLetterJobStatus = "pending"
	DLQStatusRetried   DeadLetterJobStatus = "retried"
	DLQStatusDiscarded DeadLetterJobStatus = "discarded"
)
