package models

import (
	"encoding/json"
	"time"
)

// EntityType defines the schema for a type of entity (e.g., Person, Company)
type EntityType struct {
	ID          string       `json:"id" db:"id"`
	TenantID    string          `json:"tenant_id" db:"tenant_id"`
	Key         string          `json:"key" db:"key" validate:"required,alphanum"`
	Name        string          `json:"name" db:"name" validate:"required"`
	Description string          `json:"description,omitempty" db:"description"`
	Schema      json.RawMessage `json:"schema" db:"schema"`
	Version     int             `json:"version" db:"version"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
}

// EntityTypeSchema defines the JSON schema structure for entity data validation
type EntityTypeSchema struct {
	Properties map[string]PropertyDefinition `json:"properties"`
	Required   []string                      `json:"required,omitempty"`

	// SourcePriorities is an Ivy extension that configures source trust ordering for
	// merge strategies like "source_priority" / "prefer_source".
	// Higher priority wins.
	SourcePriorities []SourcePriority `json:"source_priorities,omitempty"`
}

// PropertyDefinition defines a single property in the entity schema
type PropertyDefinition struct {
	Type        string              `json:"type"`                   // string, number, boolean, array, object
	Format      string              `json:"format,omitempty"`       // email, date, phone, etc.
	Description string              `json:"description,omitempty"`
	IsIdentity  bool                `json:"is_identity,omitempty"`  // Used for matching
	IsRequired  bool                `json:"is_required,omitempty"`
	// MergeStrategy is a custom Ivy extension to JSON schema that defines how this field should be merged
	// across sources (e.g. "most_recent", "prefer_non_empty").
	MergeStrategy MergeStrategyType `json:"merge_strategy,omitempty"`
	// ExcludeFromFingerprint excludes this field from change detection fingerprinting.
	// Use for fields that change frequently but don't represent meaningful data changes
	// (e.g., last_synced_at, record_version, audit fields).
	ExcludeFromFingerprint bool `json:"exclude_from_fingerprint,omitempty"`
	Items       *PropertyDefinition `json:"items,omitempty"`        // For array types
	Properties  map[string]PropertyDefinition `json:"properties,omitempty"` // For object types
}

// CreateEntityTypeRequest is the request body for creating an entity type
type CreateEntityTypeRequest struct {
	Key         string          `json:"key" validate:"required,alphanum"`
	Name        string          `json:"name" validate:"required"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema" validate:"required"`
}

// UpdateEntityTypeRequest is the request body for updating an entity type
type UpdateEntityTypeRequest struct {
	Name        *string          `json:"name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Schema      *json.RawMessage `json:"schema,omitempty"` // Updating schema increments version
}

// EntityTypeResponse is the API response for entity type operations
type EntityTypeResponse struct {
	EntityType
}

// EntityTypeListResponse is the API response for listing entity types
type EntityTypeListResponse struct {
	Items      []EntityType `json:"items"`
	TotalCount int          `json:"total_count"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
}

// SchemaExportResponse is the response for schema export (Lotus integration)
type SchemaExportResponse struct {
	EntityType string         `json:"entity_type"`
	Version    int            `json:"version"`
	Fields     []SchemaField  `json:"fields"`
}

// SchemaField represents a field definition compatible with Lotus target fields
type SchemaField struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// GetFingerprintExclusions returns a set of field paths that should be excluded
// from fingerprint calculation based on the schema's ExcludeFromFingerprint settings.
func (s *EntityTypeSchema) GetFingerprintExclusions() map[string]bool {
	exclusions := make(map[string]bool)
	s.collectExclusions("", s.Properties, exclusions)
	return exclusions
}

// collectExclusions recursively collects excluded fields from the schema.
func (s *EntityTypeSchema) collectExclusions(prefix string, properties map[string]PropertyDefinition, exclusions map[string]bool) {
	for name, prop := range properties {
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		if prop.ExcludeFromFingerprint {
			exclusions[path] = true
		}

		// Recurse into nested objects
		if prop.Properties != nil {
			s.collectExclusions(path, prop.Properties, exclusions)
		}
	}
}

