package models

import (
	"encoding/json"
	"time"
)

// DeletionStrategyType defines the type of deletion strategy
type DeletionStrategyType string

const (
	// DeletionStrategyExecutionBased deletes records not seen in the most recent execution
	DeletionStrategyExecutionBased DeletionStrategyType = "execution_based"
	// DeletionStrategyExplicit only deletes when explicitly requested (no automatic deletion)
	DeletionStrategyExplicit DeletionStrategyType = "explicit"
	// DeletionStrategyStaleness deletes records not updated within a retention period
	DeletionStrategyStaleness DeletionStrategyType = "staleness"
	// DeletionStrategyRetention deletes records after a fixed period from creation
	DeletionStrategyRetention DeletionStrategyType = "retention"
	// DeletionStrategyComposite combines multiple strategies with AND/OR logic
	DeletionStrategyComposite DeletionStrategyType = "composite"
)

// DeletionStrategy defines deletion policies for entities and relationships
type DeletionStrategy struct {
	ID               string               `json:"id" db:"id"`
	TenantID         string               `json:"tenant_id" db:"tenant_id"`
	EntityType       *string              `json:"entity_type,omitempty" db:"entity_type"`
	RelationshipType *string              `json:"relationship_type,omitempty" db:"relationship_type"`
	Integration      *string              `json:"integration,omitempty" db:"integration"` // NULL = applies to all sources
	SourceKey        *string              `json:"source_key,omitempty" db:"source_key"`   // NULL = applies to all plans within source
	StrategyType     DeletionStrategyType `json:"strategy_type" db:"strategy_type"`
	Config           json.RawMessage      `json:"config" db:"config"`
	Priority         int                  `json:"priority" db:"priority"` // Higher = takes precedence
	Enabled          bool                 `json:"enabled" db:"enabled"`
	Description      *string              `json:"description,omitempty" db:"description"`
	CreatedAt        time.Time            `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at" db:"updated_at"`
	DeletedAt        *time.Time           `json:"deleted_at,omitempty" db:"deleted_at"`
}

// CreateDeletionStrategyRequest is the request to create a deletion strategy
type CreateDeletionStrategyRequest struct {
	EntityType       *string              `json:"entity_type,omitempty"`
	RelationshipType *string              `json:"relationship_type,omitempty"`
	Integration      *string              `json:"integration,omitempty"`
	SourceKey        *string              `json:"source_key,omitempty"`
	StrategyType     DeletionStrategyType `json:"strategy_type" validate:"required"`
	Config           json.RawMessage      `json:"config"`
	Priority         int                  `json:"priority"`
	Enabled          bool                 `json:"enabled"`
	Description      *string              `json:"description,omitempty"`
}

// UpdateDeletionStrategyRequest is the request to update a deletion strategy
type UpdateDeletionStrategyRequest struct {
	Config      *json.RawMessage `json:"config,omitempty"`
	Priority    *int             `json:"priority,omitempty"`
	Enabled     *bool            `json:"enabled,omitempty"`
	Description *string          `json:"description,omitempty"`
}

// DeletionStrategyListResponse is the response for listing deletion strategies
type DeletionStrategyListResponse struct {
	Items      []DeletionStrategy `json:"items"`
	TotalCount int                `json:"total_count"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
}

// Strategy-specific config structures

// ExecutionBasedConfig has no additional configuration
type ExecutionBasedConfig struct {
	// No additional fields - deletes records not in the most recent execution
}

// ExplicitConfig has no additional configuration
type ExplicitConfig struct {
	// No additional fields - only explicit deletes allowed
}

// StalenessConfig configures staleness-based deletion
type StalenessConfig struct {
	MaxAgeDays int    `json:"max_age_days"` // Delete if not updated for this many days
	CheckField string `json:"check_field"`  // Field to check (default: "updated_at")
}

// RetentionConfig configures retention-based deletion
type RetentionConfig struct {
	RetentionDays int    `json:"retention_days"` // Delete after this many days from creation
	CheckField    string `json:"check_field"`    // Field to check (default: "created_at")
}

// CompositeConfig combines multiple strategies
type CompositeConfig struct {
	Strategies []CompositeStrategyItem `json:"strategies"`
	Operator   string                  `json:"operator"` // "AND" or "OR"
}

// CompositeStrategyItem is a sub-strategy in a composite
type CompositeStrategyItem struct {
	Type          DeletionStrategyType `json:"type"`
	MaxAgeDays    *int                 `json:"max_age_days,omitempty"`   // For staleness
	RetentionDays *int                 `json:"retention_days,omitempty"` // For retention
	CheckField    string               `json:"check_field,omitempty"`    // For staleness/retention
}

// DeletionStrategyMatch represents a matched strategy for application
type DeletionStrategyMatch struct {
	Strategy    *DeletionStrategy
	MatchedBy   string // "exact" (source-specific) or "default" (entity-type only)
	AppliesTo   string // "entity" or "relationship"
	TargetType  string // The entity_type or relationship_type
	Integration string // The integration (if applicable)
}

// ExecutionTracking tracks execution completions for deletion processing
type ExecutionTracking struct {
	ID          string     `json:"id" db:"id"`
	TenantID    string     `json:"tenant_id" db:"tenant_id"`
	ExecutionID string     `json:"execution_id" db:"execution_id"`
	PlanID      string     `json:"plan_id" db:"plan_id"`
	EntityType  string     `json:"entity_type" db:"entity_type"`
	EntityCount int        `json:"entity_count" db:"entity_count"`
	StartedAt   time.Time  `json:"started_at" db:"started_at"`
	CompletedAt time.Time  `json:"completed_at" db:"completed_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty" db:"processed_at"`
}

// PendingDeletion represents a deletion that is scheduled but not yet executed
type PendingDeletion struct {
	ID              string     `json:"id" db:"id"`
	TenantID        string     `json:"tenant_id" db:"tenant_id"`
	StagedEntityID  string     `json:"staged_entity_id" db:"staged_entity_id"`
	MergedEntityID  *string    `json:"merged_entity_id,omitempty" db:"merged_entity_id"`
	EntityType      string     `json:"entity_type" db:"entity_type"`
	Reason          string     `json:"reason" db:"reason"`
	ScheduledFor    time.Time  `json:"scheduled_for" db:"scheduled_for"`
	ExecutedAt      *time.Time `json:"executed_at,omitempty" db:"executed_at"`
	Cancelled       bool       `json:"cancelled" db:"cancelled"`
	CancelledReason *string    `json:"cancelled_reason,omitempty" db:"cancelled_reason"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
}
