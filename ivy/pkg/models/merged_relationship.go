package models

import (
	"encoding/json"
	"time"
)

// MergedRelationship is the canonical golden relationship between merged entities.
type MergedRelationship struct {
	ID                 string          `json:"id" db:"id"`
	TenantID           string          `json:"tenant_id" db:"tenant_id"`
	RelationshipType   string          `json:"relationship_type" db:"relationship_type"`
	FromEntityType     string          `json:"from_entity_type" db:"from_entity_type"`
	FromMergedEntityID string          `json:"from_merged_entity_id" db:"from_merged_entity_id"`
	ToEntityType       string          `json:"to_entity_type" db:"to_entity_type"`
	ToMergedEntityID   string          `json:"to_merged_entity_id" db:"to_merged_entity_id"`
	SourceKey          *string         `json:"source_key,omitempty" db:"source_key"`
	SourceExecutionID  *string         `json:"source_execution_id,omitempty" db:"source_execution_id"`
	Data               json.RawMessage `json:"data,omitempty" db:"data"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt          *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// CreateMergedRelationshipRequest is the write model used by the processor/repo.
type CreateMergedRelationshipRequest struct {
	RelationshipType   string          `json:"relationship_type"`
	FromEntityType     string          `json:"from_entity_type"`
	FromMergedEntityID string          `json:"from_merged_entity_id"`
	ToEntityType       string          `json:"to_entity_type"`
	ToMergedEntityID   string          `json:"to_merged_entity_id"`
	SourceKey          *string         `json:"source_key,omitempty"`
	SourceExecutionID  *string         `json:"source_execution_id,omitempty"`
	Data               json.RawMessage `json:"data,omitempty"`
}
