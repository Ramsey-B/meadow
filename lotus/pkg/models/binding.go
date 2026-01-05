package models

import (
	"time"

	"github.com/Ramsey-B/stem/pkg/database"
)

// Binding connects incoming messages to mapping definitions.
// When a message matches a binding's filter criteria, the associated mapping is executed.
type Binding struct {
	ID        string    `json:"id" db:"id"`
	TenantID  string    `json:"tenant_id" db:"tenant_id"`
	Name      string    `json:"name" db:"name"`
	MappingID string    `json:"mapping_id" db:"mapping_id"`
	IsEnabled bool      `json:"is_enabled" db:"is_enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Filter criteria for matching incoming messages
	Filter BindingFilter `json:"filter" db:"filter"`

	// Output configuration
	OutputTopic string `json:"output_topic" db:"output_topic"`
}

// BindingFilter defines criteria for matching incoming messages
type BindingFilter struct {
	// Integration filters by integration (e.g., "orchid")
	Integration string `json:"integration,omitempty"`

	// Keys filters by specific Orchid plan keys (empty = all plans)
	Keys []string `json:"keys,omitempty"`

	// StatusCodes filters by HTTP status codes (empty = all status codes)
	StatusCodes []int `json:"status_codes,omitempty"`

	// StepPathPrefix filters by step path prefix (e.g., "root" matches "root", "root.sub[0]", etc.)
	StepPathPrefix string `json:"step_path_prefix,omitempty"`

	// RequestURLContains filters by substring match against Orchid's request_url
	// (e.g. "/v1.0/users", "/api/v1/users", etc.). Useful for manual testing without
	// needing to hardcode plan UUIDs.
	RequestURLContains string `json:"request_url_contains,omitempty"`

	// MinStatusCode filters messages with status_code >= this value
	MinStatusCode int `json:"min_status_code,omitempty"`

	// MaxStatusCode filters messages with status_code <= this value
	MaxStatusCode int `json:"max_status_code,omitempty"`
}

// BindingWithMapping includes the mapping definition for execution
type BindingWithMapping struct {
	Binding
	MappingVersion int `json:"mapping_version" db:"mapping_version"`
}

// BindingDBModel is the database representation of a binding
type BindingDBModel struct {
	ID          string                        `db:"id"`
	TenantID    string                        `db:"tenant_id"`
	Name        string                        `db:"name"`
	MappingID   string                        `db:"mapping_id"`
	IsEnabled   bool                          `db:"is_enabled"`
	Filter      database.JSONB[BindingFilter] `db:"filter"`
	OutputTopic string                        `db:"output_topic"`
	CreatedAt   time.Time                     `db:"created_at"`
	UpdatedAt   time.Time                     `db:"updated_at"`
}

// ToBinding converts the database model to the domain model
func (b *BindingDBModel) ToBinding() *Binding {
	return &Binding{
		ID:          b.ID,
		TenantID:    b.TenantID,
		Name:        b.Name,
		MappingID:   b.MappingID,
		IsEnabled:   b.IsEnabled,
		Filter:      b.Filter.GetValue(),
		OutputTopic: b.OutputTopic,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}

// ToDBModel converts the domain model to the database model
func (b *Binding) ToDBModel() *BindingDBModel {
	return &BindingDBModel{
		ID:          b.ID,
		TenantID:    b.TenantID,
		Name:        b.Name,
		MappingID:   b.MappingID,
		IsEnabled:   b.IsEnabled,
		Filter:      database.JSONB[BindingFilter]{Data: b.Filter},
		OutputTopic: b.OutputTopic,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}
