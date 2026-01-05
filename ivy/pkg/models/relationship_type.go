package models

import (
	"encoding/json"
	"time"
)

// Cardinality defines the relationship cardinality
type Cardinality string

const (
	CardinalityOneToOne   Cardinality = "one_to_one"
	CardinalityOneToMany  Cardinality = "one_to_many"
	CardinalityManyToOne  Cardinality = "many_to_one"
	CardinalityManyToMany Cardinality = "many_to_many"
)

// RelationshipType defines the schema for a type of relationship between entities
type RelationshipType struct {
	ID       string `json:"id" db:"id"`
	TenantID string `json:"tenant_id" db:"tenant_id"`
	// Key is typically snake_case (e.g. "member_of").
	Key            string      `json:"key" db:"key" validate:"required"`
	Name           string      `json:"name" db:"name" validate:"required"`
	Description    string      `json:"description,omitempty" db:"description"`
	FromEntityType string      `json:"from_entity_type" db:"from_entity_type" validate:"required"`
	ToEntityType   string      `json:"to_entity_type" db:"to_entity_type" validate:"required"`
	Cardinality    Cardinality `json:"cardinality" db:"cardinality"`
	// Schema defines the JSON schema (with Ivy extensions like merge_strategy) for relationship properties.
	// Stored in the database column `properties`.
	Schema    json.RawMessage `json:"schema,omitempty" db:"properties"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// CreateRelationshipTypeRequest is the request body for creating a relationship type
type CreateRelationshipTypeRequest struct {
	// Key is typically snake_case (e.g. "member_of").
	Key            string          `json:"key" validate:"required"`
	Name           string          `json:"name" validate:"required"`
	Description    string          `json:"description,omitempty"`
	FromEntityType string          `json:"from_entity_type" validate:"required"`
	ToEntityType   string          `json:"to_entity_type" validate:"required"`
	Cardinality    Cardinality     `json:"cardinality" validate:"required,oneof=one_to_one one_to_many many_to_one many_to_many"`
	Schema         json.RawMessage `json:"schema,omitempty"`
}

// UpdateRelationshipTypeRequest is the request body for updating a relationship type
type UpdateRelationshipTypeRequest struct {
	Name        *string          `json:"name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Cardinality *Cardinality     `json:"cardinality,omitempty"`
	Schema      *json.RawMessage `json:"schema,omitempty"`
}

// RelationshipTypeResponse is the API response for relationship type operations
type RelationshipTypeResponse struct {
	RelationshipType
}

// RelationshipTypeListResponse is the API response for listing relationship types
type RelationshipTypeListResponse struct {
	Items      []RelationshipType `json:"items"`
	TotalCount int                `json:"total_count"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
}
