package models

import (
	"time"

	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/google/uuid"
)

// Plan defines an API polling plan
type Plan struct {
	Key            string                         `db:"key" json:"key"`
	TenantID       uuid.UUID                      `db:"tenant_id" json:"tenant_id"`
	IntegrationID  uuid.UUID                      `db:"integration_id" json:"integration_id"`
	Integration    string                         `db:"integration" json:"integration"`
	Name           string                         `db:"name" json:"name"`
	Description    *string                        `db:"description" json:"description,omitempty"`
	PlanDefinition database.JSONB[map[string]any] `db:"plan_definition" json:"plan_definition"`
	Enabled        bool                           `db:"enabled" json:"enabled"`
	WaitSeconds    *int                           `db:"wait_seconds" json:"wait_seconds,omitempty"`
	RepeatCount    *int                           `db:"repeat_count" json:"repeat_count,omitempty"`
	CreatedAt      time.Time                      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time                      `db:"updated_at" json:"updated_at"`
}

// TableName returns the database table name
func (Plan) TableName() string {
	return "plans"
}
