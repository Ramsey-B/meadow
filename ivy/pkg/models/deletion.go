package models

import (
	"encoding/json"
	"time"
)

// LotusDeleteMessage represents a delete instruction from Lotus
type LotusDeleteMessage struct {
	TenantID    string          `json:"tenant_id"`
	ExecutionID string          `json:"execution_id"`
	SourceKey   string          `json:"source_key"`
	ConfigID    string          `json:"config_id"`
	Integration string          `json:"integration"`
	Source      LotusSource     `json:"source"`
	Action      string          `json:"action"` // "delete"
	EntityType  string          `json:"entity_type"`
	EntityID    string          `json:"entity_id"` // Source ID to delete
	Timestamp   time.Time       `json:"timestamp"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// LotusSource is the source metadata from a Lotus message
type LotusSource struct {
	Type        string `json:"type"`
	TenantID    string `json:"tenant_id"`
	Key         string `json:"key,omitempty"`       // Plan key
	ConfigID    string `json:"config_id,omitempty"` // Integration configuration ID
	ExecutionID string `json:"execution_id,omitempty"`
}

// ExecutionCompletedMessage represents an execution completion event from Orchid
type ExecutionCompletedMessage struct {
	TenantID     string         `json:"tenant_id"`
	SourceKey    string         `json:"source_key"`
	ExecutionID  string         `json:"execution_id"`
	Status       string         `json:"status"` // success, partial_success, failure
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  time.Time      `json:"completed_at"`
	EntityCounts map[string]int `json:"entity_counts,omitempty"` // entity_type -> count
}
