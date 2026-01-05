package models

import (
	"encoding/json"
	"time"
)

// MergeStrategyType defines how to merge a field when combining entities
type MergeStrategyType string

const (
	// MergeStrategyMostRecent uses the most recently updated value
	MergeStrategyMostRecent MergeStrategyType = "most_recent"
	// MergeStrategyMostTrusted uses the value from the most trusted source
	MergeStrategyMostTrusted MergeStrategyType = "most_trusted"
	// MergeStrategyCollectAll combines all values into an array
	MergeStrategyCollectAll MergeStrategyType = "collect_all"
	// MergeStrategyLongestValue uses the longest string value
	MergeStrategyLongestValue MergeStrategyType = "longest"
	// MergeStrategyShortestValue uses the shortest string value
	MergeStrategyShortestValue MergeStrategyType = "shortest"
	// MergeStrategyFirstValue uses the first encountered value
	MergeStrategyFirstValue MergeStrategyType = "first"
	// MergeStrategyLastValue uses the last encountered value
	MergeStrategyLastValue MergeStrategyType = "last"
	// MergeStrategyMax uses the maximum numeric value
	MergeStrategyMax MergeStrategyType = "max"
	// MergeStrategyMin uses the minimum numeric value
	MergeStrategyMin MergeStrategyType = "min"
	// MergeStrategySum sums numeric values
	MergeStrategySum MergeStrategyType = "sum"
	// MergeStrategyAverage averages numeric values
	MergeStrategyAverage MergeStrategyType = "average"
	// MergeStrategyPreferNonEmpty uses the first non-empty value
	MergeStrategyPreferNonEmpty MergeStrategyType = "prefer_non_empty"
	// MergeStrategySourcePriority uses value from highest priority source
	MergeStrategySourcePriority MergeStrategyType = "source_priority"
)

// MergeStrategy defines merge strategies for an entity type
type MergeStrategy struct {
	ID               string          `json:"id" db:"id"`
	TenantID         string          `json:"tenant_id" db:"tenant_id"`
	EntityType       string          `json:"entity_type" db:"entity_type"`
	Name             string          `json:"name" db:"name"`
	Description      *string         `json:"description,omitempty" db:"description"`
	IsDefault        bool            `json:"is_default" db:"is_default"`
	FieldStrategies  json.RawMessage `json:"field_strategies" db:"field_strategies"`
	SourcePriorities json.RawMessage `json:"source_priorities,omitempty" db:"source_priorities"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt        *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// FieldMergeStrategy defines how to merge a specific field
type FieldMergeStrategy struct {
	Field    string            `json:"field"`
	Strategy MergeStrategyType `json:"strategy"`
	Fallback MergeStrategyType `json:"fallback,omitempty"`  // Used if primary fails
	MaxItems int               `json:"max_items,omitempty"` // For collect_all
	Dedup    bool              `json:"dedup,omitempty"`     // For collect_all
}

// SourcePriority defines source trust levels
type SourcePriority struct {
	Integration string `json:"integration"`
	Priority    int    `json:"priority"` // Higher = more trusted
}

// CreateMergeStrategyRequest is the request to create a merge strategy
type CreateMergeStrategyRequest struct {
	EntityType       string          `json:"entity_type" validate:"required"`
	Name             string          `json:"name" validate:"required"`
	Description      *string         `json:"description,omitempty"`
	IsDefault        bool            `json:"is_default"`
	FieldStrategies  json.RawMessage `json:"field_strategies" validate:"required"`
	SourcePriorities json.RawMessage `json:"source_priorities,omitempty"`
}

// MergedEntity represents the golden record after merging
type MergedEntity struct {
	ID              string          `json:"id" db:"id"`
	TenantID        string          `json:"tenant_id" db:"tenant_id"`
	EntityType      string          `json:"entity_type" db:"entity_type"`
	Data            json.RawMessage `json:"data" db:"data"`
	SourceCount     int             `json:"source_count" db:"source_count"`
	PrimarySourceID *string         `json:"primary_source_id,omitempty" db:"primary_source_id"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt       *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
	Version         int             `json:"version" db:"version"`
}

// EntityCluster links staged entities to their merged entity
type EntityCluster struct {
	ID             string     `json:"id" db:"id"`
	TenantID       string     `json:"tenant_id" db:"tenant_id"`
	MergedEntityID string     `json:"merged_entity_id" db:"merged_entity_id"`
	StagedEntityID string     `json:"staged_entity_id" db:"staged_entity_id"`
	IsPrimary      bool       `json:"is_primary" db:"is_primary"`
	AddedAt        time.Time  `json:"added_at" db:"added_at"`
	RemovedAt      *time.Time `json:"removed_at,omitempty" db:"removed_at"`
}

// RelationshipCluster links staged relationships to their merged relationship
type RelationshipCluster struct {
	ID                   string     `json:"id" db:"id"`
	TenantID             string     `json:"tenant_id" db:"tenant_id"`
	MergedRelationshipID string     `json:"merged_relationship_id" db:"merged_relationship_id"`
	StagedRelationshipID string     `json:"staged_relationship_id" db:"staged_relationship_id"`
	IsPrimary            bool       `json:"is_primary" db:"is_primary"`
	AddedAt              time.Time  `json:"added_at" db:"added_at"`
	RemovedAt            *time.Time `json:"removed_at,omitempty" db:"removed_at"`
}

// MergeAuditLog records merge operations for audit trail
type MergeAuditLog struct {
	ID             string          `json:"id" db:"id"`
	TenantID       string          `json:"tenant_id" db:"tenant_id"`
	MergedEntityID string          `json:"merged_entity_id" db:"merged_entity_id"`
	Action         string          `json:"action" db:"action"` // created, merged, split, updated
	SourceEntities json.RawMessage `json:"source_entities" db:"source_entities"`
	OldData        json.RawMessage `json:"old_data,omitempty" db:"old_data"`
	NewData        json.RawMessage `json:"new_data" db:"new_data"`
	StrategyUsed   string          `json:"strategy_used" db:"strategy_used"`
	Conflicts      json.RawMessage `json:"conflicts,omitempty" db:"conflicts"`
	PerformedBy    *string         `json:"performed_by,omitempty" db:"performed_by"`
	PerformedAt    time.Time       `json:"performed_at" db:"performed_at"`
}

// MergeConflict represents a conflict during merge
type MergeConflict struct {
	Field         string   `json:"field"`
	Values        []any    `json:"values"`
	Integrations  []string `json:"integrations"`
	Resolution    string   `json:"resolution"`
	ResolvedValue any      `json:"resolved_value"`
}

// MergeResult contains the result of a merge operation
type MergeResult struct {
	MergedEntity   *MergedEntity   `json:"merged_entity"`
	SourceEntities []string        `json:"source_entities"`
	Conflicts      []MergeConflict `json:"conflicts,omitempty"`
	IsNew          bool            `json:"is_new"`
	Version        int             `json:"version"`
}
