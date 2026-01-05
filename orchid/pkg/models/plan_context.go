package models

import (
	"time"

	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/google/uuid"
)

// PlanContext stores context data for a plan/config combination
type PlanContext struct {
	ID          uuid.UUID                      `db:"id" json:"id"`
	TenantID    uuid.UUID                      `db:"tenant_id" json:"tenant_id"`
	PlanKey     string                         `db:"plan_key" json:"plan_key"`
	ConfigID    uuid.UUID                      `db:"config_id" json:"config_id"`
	ContextData database.JSONB[map[string]any] `db:"context_data" json:"context_data"`
	CreatedAt   time.Time                      `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time                      `db:"updated_at" json:"updated_at"`
}

// TableName returns the database table name
func (PlanContext) TableName() string {
	return "plan_contexts"
}
