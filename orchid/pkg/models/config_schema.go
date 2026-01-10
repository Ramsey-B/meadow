package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/Ramsey-B/stem/pkg/database"
)

// ConfigSchema defines the schema/structure for configuration values
type ConfigSchema struct {
	ID            uuid.UUID                      `db:"id" json:"id"`
	TenantID      uuid.UUID                      `db:"tenant_id" json:"tenant_id"`
	IntegrationID uuid.UUID                      `db:"integration_id" json:"integration_id"`
	Name          string                         `db:"name" json:"name"`
	Schema        database.JSONB[map[string]any] `db:"schema" json:"schema"`
	CreatedAt     time.Time                      `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time                      `db:"updated_at" json:"updated_at"`
}

// TableName returns the database table name
func (ConfigSchema) TableName() string {
	return "config_schemas"
}

