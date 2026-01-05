package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/Ramsey-B/stem/pkg/database"
)

// AuthFlow defines an authentication flow for an integration
type AuthFlow struct {
	ID             uuid.UUID                      `db:"id" json:"id"`
	TenantID       uuid.UUID                      `db:"tenant_id" json:"tenant_id"`
	IntegrationID  uuid.UUID                      `db:"integration_id" json:"integration_id"`
	Name           string                         `db:"name" json:"name"`
	PlanDefinition database.JSONB[map[string]any] `db:"plan_definition" json:"plan_definition"`
	TokenPath      string                         `db:"token_path" json:"token_path"`
	HeaderName     string                         `db:"header_name" json:"header_name"`
	HeaderFormat   *string                        `db:"header_format" json:"header_format,omitempty"`
	RefreshPath    *string                        `db:"refresh_path" json:"refresh_path,omitempty"`
	ExpiresInPath  *string                        `db:"expires_in_path" json:"expires_in_path,omitempty"`
	TTLSeconds     *int                           `db:"ttl_seconds" json:"ttl_seconds,omitempty"`
	SkewSeconds    *int                           `db:"skew_seconds" json:"skew_seconds,omitempty"`
	CreatedAt      time.Time                      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time                      `db:"updated_at" json:"updated_at"`
}

// TableName returns the database table name
func (AuthFlow) TableName() string {
	return "auth_flows"
}

