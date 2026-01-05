package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/Ramsey-B/stem/pkg/database"
)

// Integration represents a third-party API integration
type Integration struct {
	ID           uuid.UUID                      `db:"id" json:"id"`
	TenantID     uuid.UUID                      `db:"tenant_id" json:"tenant_id"`
	Name         string                         `db:"name" json:"name"`
	Description  *string                        `db:"description" json:"description,omitempty"`
	// ConfigSchema is integration-owned metadata (no separate config_schema_id)
	ConfigSchema database.JSONB[map[string]any] `db:"config_schema" json:"config_schema,omitempty"`
	CreatedAt    time.Time                      `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time                      `db:"updated_at" json:"updated_at"`
}

// TableName returns the database table name
func (Integration) TableName() string {
	return "integrations"
}

