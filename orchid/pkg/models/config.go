package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/Ramsey-B/stem/pkg/database"
)

// Config represents an actual configuration instance with values
type Config struct {
	ID             uuid.UUID                      `db:"id" json:"id"`
	TenantID       uuid.UUID                      `db:"tenant_id" json:"tenant_id"`
	IntegrationID  uuid.UUID                      `db:"integration_id" json:"integration_id"`
	Name           string                         `db:"name" json:"name"`
	Values         database.JSONB[map[string]any] `db:"values" json:"values"`
	Enabled        bool                           `db:"enabled" json:"enabled"`
	CreatedAt      time.Time                      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time                      `db:"updated_at" json:"updated_at"`
}

// TableName returns the database table name
func (Config) TableName() string {
	return "configs"
}

