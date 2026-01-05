package models

import (
	"encoding/json"
	"time"
)

// StagedEntity represents a raw entity before merging
// Field order matches schema: id, tenant_id, config_id, entity_type, source_id, integration, source_key, ...
type StagedEntity struct {
	ID                  string          `json:"id" db:"id"`
	TenantID            string          `json:"tenant_id" db:"tenant_id"`
	ConfigID            string          `json:"config_id" db:"config_id"` // Integration configuration ID from Orchid
	EntityType          string          `json:"entity_type" db:"entity_type"`
	SourceID            string          `json:"source_id" db:"source_id"`
	Integration         string          `json:"integration" db:"integration"`
	SourceKey           string          `json:"source_key" db:"source_key"`
	SourceExecutionID   *string         `json:"source_execution_id,omitempty" db:"source_execution_id"`
	ExecutionID         *string         `json:"execution_id,omitempty" db:"execution_id"`
	LastSeenExecution   *string         `json:"last_seen_execution,omitempty" db:"last_seen_execution"`
	Data                json.RawMessage `json:"data" db:"data"`
	Fingerprint         string          `json:"fingerprint" db:"fingerprint"`
	PreviousFingerprint string          `json:"previous_fingerprint,omitempty" db:"previous_fingerprint"`
	CreatedAt           time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt           *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// CreateStagedEntityRequest is the request for creating/upserting a staged entity
// Field order matches schema: config_id, entity_type, source_id, integration, source_key, ...
type CreateStagedEntityRequest struct {
	ConfigID          string          `json:"config_id" validate:"required"`  // Integration configuration ID
	EntityType        string          `json:"entity_type" validate:"required"`
	SourceID          string          `json:"source_id" validate:"required"`
	Integration       string          `json:"integration" validate:"required"`
	SourceKey         string          `json:"source_key" validate:"required"` // Plan key
	SourceExecutionID *string         `json:"source_execution_id,omitempty"`
	Data              json.RawMessage `json:"data" validate:"required"`
}

// StagedEntityListResponse is the response for listing staged entities
type StagedEntityListResponse struct {
	Items      []StagedEntity `json:"items"`
	TotalCount int            `json:"total_count"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
}

// StagedRelationship represents a relationship (direct or criteria-materialized)
// Field order matches schema: id, tenant_id, config_id, integration, source_key, ...
// All relationships flow through the same merge pipeline to create merged_relationships
type StagedRelationship struct {
	ID               string `json:"id" db:"id"`
	TenantID         string `json:"tenant_id" db:"tenant_id"`
	ConfigID         string `json:"config_id" db:"config_id"`
	Integration      string `json:"integration" db:"integration"`
	SourceKey        string `json:"source_key" db:"source_key"`
	RelationshipType string `json:"relationship_type" db:"relationship_type"`

	// From side (always by source_id)
	FromEntityType     string  `json:"from_entity_type" db:"from_entity_type"`
	FromSourceID       string  `json:"from_source_id" db:"from_source_id"`
	FromIntegration    string  `json:"from_integration" db:"from_integration"`
	FromStagedEntityID *string `json:"from_staged_entity_id,omitempty" db:"from_staged_entity_id"`

	// To side (by source_id)
	ToEntityType     string  `json:"to_entity_type" db:"to_entity_type"`
	ToSourceID       string  `json:"to_source_id" db:"to_source_id"`
	ToIntegration    string  `json:"to_integration" db:"to_integration"`
	ToStagedEntityID *string `json:"to_staged_entity_id,omitempty" db:"to_staged_entity_id"`

	// Optional: criteria that created this relationship (NULL for direct relationships)
	CriteriaID *string `json:"criteria_id,omitempty" db:"criteria_id"`

	// Execution tracking for deletion strategies
	SourceExecutionID *string `json:"source_execution_id,omitempty" db:"source_execution_id"`
	ExecutionID       *string `json:"execution_id,omitempty" db:"execution_id"`
	LastSeenExecution *string `json:"last_seen_execution,omitempty" db:"last_seen_execution"`

	Data      json.RawMessage `json:"data,omitempty" db:"data"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// IsCriteriaMaterialized returns true if this relationship was created from a criteria match
func (r *StagedRelationship) IsCriteriaMaterialized() bool {
	return r.CriteriaID != nil
}

// CreateStagedRelationshipRequest is the request for creating a staged relationship
// (direct or criteria-materialized)
type CreateStagedRelationshipRequest struct {
	RelationshipType string `json:"relationship_type" validate:"required"`

	// From side
	FromEntityType  string `json:"from_entity_type" validate:"required"`
	FromSourceID    string `json:"from_source_id" validate:"required"`
	FromIntegration string `json:"from_integration" validate:"required"`

	// To side
	ToEntityType  string `json:"to_entity_type" validate:"required"`
	ToSourceID    string `json:"to_source_id" validate:"required"`
	ToIntegration string `json:"to_integration" validate:"required"`

	// Optional: criteria that created this relationship (for criteria-materialized relationships)
	CriteriaID *string `json:"criteria_id,omitempty"`

	// Source metadata
	Integration       string  `json:"integration" validate:"required"`
	SourceKey         string  `json:"source_key" validate:"omitempty"`
	ConfigID          string  `json:"config_id" validate:"required"`
	SourceExecutionID *string `json:"source_execution_id,omitempty"`

	Data json.RawMessage `json:"data,omitempty"`
}

// StagedRelationshipCriteria represents a criteria-based relationship definition
// These are "subscriptions" that match multiple target entities based on criteria
type StagedRelationshipCriteria struct {
	ID               string `json:"id" db:"id"`
	TenantID         string `json:"tenant_id" db:"tenant_id"`
	ConfigID         string `json:"config_id" db:"config_id"`
	Integration      string `json:"integration" db:"integration"`
	SourceKey        string `json:"source_key" db:"source_key"`
	RelationshipType string `json:"relationship_type" db:"relationship_type"`

	// From side (always by source_id)
	FromEntityType     string  `json:"from_entity_type" db:"from_entity_type"`
	FromSourceID       string  `json:"from_source_id" db:"from_source_id"`
	FromIntegration    string  `json:"from_integration" db:"from_integration"`
	FromStagedEntityID *string `json:"from_staged_entity_id,omitempty" db:"from_staged_entity_id"`

	// To side (criteria-based, always scoped by type + integration)
	ToEntityType  string          `json:"to_entity_type" db:"to_entity_type"`
	ToIntegration string          `json:"to_integration" db:"to_integration"`
	Criteria      json.RawMessage `json:"criteria" db:"criteria"`
	CriteriaHash  string          `json:"criteria_hash" db:"criteria_hash"`

	// Execution tracking for deletion strategies
	SourceExecutionID *string `json:"source_execution_id,omitempty" db:"source_execution_id"`
	ExecutionID       *string `json:"execution_id,omitempty" db:"execution_id"`
	LastSeenExecution *string `json:"last_seen_execution,omitempty" db:"last_seen_execution"`

	Data      json.RawMessage `json:"data,omitempty" db:"data"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// CreateStagedRelationshipCriteriaRequest is the request for creating a criteria-based relationship
type CreateStagedRelationshipCriteriaRequest struct {
	RelationshipType string `json:"relationship_type" validate:"required"`

	// From side
	FromEntityType  string `json:"from_entity_type" validate:"required"`
	FromSourceID    string `json:"from_source_id" validate:"required"`
	FromIntegration string `json:"from_integration" validate:"required"`

	// To side (criteria-based)
	ToEntityType  string         `json:"to_entity_type" validate:"required"`
	ToIntegration string         `json:"to_integration" validate:"required"`
	Criteria      map[string]any `json:"criteria" validate:"required"`

	// Source metadata
	Integration       string  `json:"integration" validate:"required"`
	SourceKey         string  `json:"source_key" validate:"omitempty"`
	ConfigID          string  `json:"config_id" validate:"required"`
	SourceExecutionID *string `json:"source_execution_id,omitempty"`

	Data json.RawMessage `json:"data,omitempty"`
}

// StagedRelationshipCriteriaMatch represents a materialized match between criteria and an entity
// Each match creates a staged_relationship that flows through the normal merge pipeline
type StagedRelationshipCriteriaMatch struct {
	CriteriaID           string    `json:"criteria_id" db:"criteria_id"`
	ToStagedEntityID     string    `json:"to_staged_entity_id" db:"to_staged_entity_id"`
	StagedRelationshipID string    `json:"staged_relationship_id" db:"staged_relationship_id"`
	TenantID             string    `json:"tenant_id" db:"tenant_id"`
	RelationshipType     string    `json:"relationship_type" db:"relationship_type"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	LastVerifiedAt       time.Time `json:"last_verified_at" db:"last_verified_at"`
}

// EntityMatchIndex represents denormalized match fields for fast lookups
type EntityMatchIndex struct {
	ID              string    `json:"id" db:"id"`
	TenantID        string    `json:"tenant_id" db:"tenant_id"`
	StagedEntityID  string    `json:"staged_entity_id" db:"staged_entity_id"`
	EntityType      string    `json:"entity_type" db:"entity_type"`
	Field1          *string   `json:"field_1,omitempty" db:"field_1"`
	Field2          *string   `json:"field_2,omitempty" db:"field_2"`
	Field3          *string   `json:"field_3,omitempty" db:"field_3"`
	Field4          *string   `json:"field_4,omitempty" db:"field_4"`
	Field5          *string   `json:"field_5,omitempty" db:"field_5"`
	Field3Soundex   *string   `json:"field_3_soundex,omitempty" db:"field_3_soundex"`
	Field4Soundex   *string   `json:"field_4_soundex,omitempty" db:"field_4_soundex"`
	Field3Metaphone *string   `json:"field_3_metaphone,omitempty" db:"field_3_metaphone"`
	Field4Metaphone *string   `json:"field_4_metaphone,omitempty" db:"field_4_metaphone"`
	NameCombined    *string   `json:"name_combined,omitempty" db:"name_combined"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// MatchFieldMapping maps entity type fields to match index columns
type MatchFieldMapping struct {
	ID              string          `json:"id" db:"id"`
	TenantID        string          `json:"tenant_id" db:"tenant_id"`
	EntityType      string          `json:"entity_type" db:"entity_type"`
	SourcePath      string          `json:"source_path" db:"source_path"`
	TargetColumn    string          `json:"target_column" db:"target_column"`
	Normalizer      *string         `json:"normalizer,omitempty" db:"normalizer"`
	ArrayHandling   string          `json:"array_handling" db:"array_handling"`
	ArrayFilter     json.RawMessage `json:"array_filter,omitempty" db:"array_filter"`
	IncludePhonetic bool            `json:"include_phonetic" db:"include_phonetic"`
	IncludeTrigram  bool            `json:"include_trigram" db:"include_trigram"`
}

// LotusMessage represents an incoming message from Lotus
type LotusMessage struct {
	Source         LotusMessageSource  `json:"source"`
	TenantID       string              `json:"tenant_id"`
	ExecutionID    string              `json:"execution_id"`
	SourceKey      string              `json:"source_key"`
	ConfigID       string              `json:"config_id"`
	Integration    string              `json:"integration"`
	BindingID      string              `json:"binding_id"`
	MappingID      string              `json:"mapping_id"`
	MappingVersion int                 `json:"mapping_version"`
	Timestamp      time.Time           `json:"timestamp"`
	TargetSchema   *TargetSchema       `json:"target_schema,omitempty"`
	Data           map[string]any      `json:"data"`
	Relationships  []LotusRelationship `json:"relationships,omitempty"`
}

// LotusMessageSource identifies the source of the data
type LotusMessageSource struct {
	Type        string `json:"type"`
	TenantID    string `json:"tenant_id"`
	Integration string `json:"integration"`
	Key         string `json:"key,omitempty"`       // Plan key
	ConfigID    string `json:"config_id,omitempty"` // Integration configuration ID
	ExecutionID string `json:"execution_id,omitempty"`
	MappingID   string `json:"mapping_id,omitempty"`
}

// TargetSchema identifies the target entity/relationship type
type TargetSchema struct {
	Type       string `json:"type"`        // "entity" or "relationship"
	EntityType string `json:"entity_type"` // e.g., "person"
	Version    int    `json:"version,omitempty"`
}

// LotusRelationship represents a relationship in a Lotus message
type LotusRelationship struct {
	Type         string         `json:"type"` // relationship type key
	ToEntityType string         `json:"to_entity_type"`
	ToSourceID   string         `json:"to_source_id"`
	Data         map[string]any `json:"data,omitempty"`
}
