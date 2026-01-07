package kafka

import (
	"encoding/json"
	"time"

	"github.com/Ramsey-B/ivy/pkg/models"
)

// IncomingMessage wraps a raw Kafka message with parsed headers
type IncomingMessage struct {
	Key       string
	Value     []byte
	Headers   map[string]string
	Partition int
	Offset    int64
	Timestamp time.Time
	Topic     string

	// Trace context (extracted from Kafka headers)
	TraceParent string
	TraceState  string

	// Parsed content
	LotusMessage  *models.LotusMessage
	DeleteMessage *models.LotusDeleteMessage
}

// ParseLotusMessage parses the message value as a Lotus message
func (m *IncomingMessage) ParseLotusMessage() error {
	var msg models.LotusMessage
	if err := json.Unmarshal(m.Value, &msg); err != nil {
		return err
	}
	m.LotusMessage = &msg
	return nil
}

// GetTenantID returns the tenant ID from the Lotus message
func (m *IncomingMessage) GetTenantID() string {
	if m.LotusMessage != nil {
		return m.LotusMessage.Source.TenantID
	}
	if m.DeleteMessage != nil {
		return m.DeleteMessage.Source.TenantID
	}
	if m.LotusMessage != nil && m.LotusMessage.TenantID != "" {
		return m.LotusMessage.TenantID
	}
	if m.DeleteMessage != nil && m.DeleteMessage.TenantID != "" {
		return m.DeleteMessage.TenantID
	}
	// Fallback to header
	return m.Headers["tenant_id"]
}

// GetExecutionID returns the execution ID from the Lotus message
func (m *IncomingMessage) GetExecutionID() string {
	if m.LotusMessage != nil {
		return m.LotusMessage.Source.ExecutionID
	}
	if m.DeleteMessage != nil {
		return m.DeleteMessage.Source.ExecutionID
	}
	return ""
}

// GetSourceKey returns the plan key from the Lotus message
func (m *IncomingMessage) GetSourceKey() string {
	if m.LotusMessage != nil {
		return m.LotusMessage.Source.Key
	}
	if m.DeleteMessage != nil {
		return m.DeleteMessage.Source.Key
	}
	return ""
}

// GetConfigID returns the integration configuration ID from the Lotus message
func (m *IncomingMessage) GetConfigID() string {
	if m.LotusMessage != nil {
		return m.LotusMessage.Source.ConfigID
	}
	if m.DeleteMessage != nil {
		return m.DeleteMessage.Source.ConfigID
	}
	return ""
}

func (m *IncomingMessage) IsDeleteMessage() bool {
	var del models.LotusDeleteMessage
	if err := json.Unmarshal(m.Value, &del); err != nil {
		return false
	}
	return del.Action == "delete" && del.EntityType != "" && del.EntityID != "" && del.Source.TenantID != ""
}

func (m *IncomingMessage) ParseDeleteMessage() (*models.LotusDeleteMessage, error) {
	var del models.LotusDeleteMessage
	if err := json.Unmarshal(m.Value, &del); err != nil {
		return nil, err
	}
	return &del, nil
}

// GetEntityType returns the entity type from the target schema
func (m *IncomingMessage) GetEntityType() string {
	if m.LotusMessage != nil && m.LotusMessage.TargetSchema != nil {
		return m.LotusMessage.TargetSchema.EntityType
	}
	if m.LotusMessage != nil && m.LotusMessage.Data != nil {
		if v, ok := m.LotusMessage.Data["_entity_type"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
		// Allow non-underscore variant for compatibility
		if v, ok := m.LotusMessage.Data["entity_type"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// GetSourceID returns a unique source identifier for the entity
func (m *IncomingMessage) GetSourceID() string {
	if m.LotusMessage != nil {
		if m.LotusMessage.Data != nil {
			// Prefer the explicit Ivy identity fields produced by Lotus mappings
			if v, ok := m.LotusMessage.Data["_source_id"]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
			if v, ok := m.LotusMessage.Data["source_id"]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
		// Fallback to binding + key
		return m.LotusMessage.BindingID + ":" + m.Key
	}
	return m.Key
}

// GetIntegration returns the integration for this message
func (m *IncomingMessage) GetIntegration() string {
	if m.LotusMessage != nil {
		if m.LotusMessage.Data != nil {
			// Prefer the explicit Ivy identity fields produced by Lotus mappings
			if v, ok := m.LotusMessage.Data["_integration"]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
			if v, ok := m.LotusMessage.Data["integration"]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
		return m.LotusMessage.Source.Integration
	}
	return ""
}

// IsEntity returns true if this message represents an entity
func (m *IncomingMessage) IsEntity() bool {
	if m.LotusMessage != nil && m.LotusMessage.TargetSchema != nil {
		return m.LotusMessage.TargetSchema.Type == "entity"
	}
	// Default to true if no schema info
	return true
}

// IsRelationship returns true if this message represents a relationship
func (m *IncomingMessage) IsRelationship() bool {
	if m.LotusMessage != nil && m.LotusMessage.TargetSchema != nil {
		return m.LotusMessage.TargetSchema.Type == "relationship"
	}
	// Heuristic: treat messages containing relationship meta fields as relationship messages.
	// This supports legacy/manual Lotus mappings that don't set target_schema.type.
	if m.LotusMessage != nil && m.LotusMessage.Data != nil {
		if _, ok := m.LotusMessage.Data["_relationship_type"]; ok {
			return true
		}
		if _, ok := m.LotusMessage.Data["_from_entity_type"]; ok {
			return true
		}
		if _, ok := m.LotusMessage.Data["_to_entity_type"]; ok {
			return true
		}
	}
	return false
}

// GetData returns the entity data as JSON
func (m *IncomingMessage) GetData() json.RawMessage {
	if m.LotusMessage != nil {
		b, _ := json.Marshal(m.LotusMessage.Data)
		return b
	}
	return m.Value
}

// GetRelationships returns embedded relationships from the message
func (m *IncomingMessage) GetRelationships() []models.LotusRelationship {
	if m.LotusMessage != nil {
		return m.LotusMessage.Relationships
	}
	return nil
}

// ExecutionCompletedMessage represents an execution.completed event from Orchid
type ExecutionCompletedMessage struct {
	Type        string         `json:"type"` // "execution.completed"
	TenantID    string         `json:"tenant_id"`
	SourceKey   string         `json:"source_key"`
	ExecutionID string         `json:"execution_id"`
	Status      string         `json:"status"` // "success", "partial", "failed"
	Timestamp   time.Time      `json:"timestamp"`
	Stats       ExecutionStats `json:"stats,omitempty"`
}

// ExecutionStats contains statistics about the execution
type ExecutionStats struct {
	TotalSteps      int   `json:"total_steps"`
	SuccessfulSteps int   `json:"successful_steps"`
	FailedSteps     int   `json:"failed_steps"`
	ItemsEmitted    int   `json:"items_emitted"`
	DurationMs      int64 `json:"duration_ms"`
}

// IsExecutionCompleted checks if the message is an execution.completed event
func (m *IncomingMessage) IsExecutionCompleted() bool {
	// Check header first
	if msgType := m.Headers["type"]; msgType == "execution.completed" {
		return true
	}

	// Try parsing as execution completed
	var evt ExecutionCompletedMessage
	if err := json.Unmarshal(m.Value, &evt); err == nil {
		return evt.Type == "execution.completed"
	}

	return false
}

// ParseExecutionCompleted parses the message as an execution.completed event
func (m *IncomingMessage) ParseExecutionCompleted() (*ExecutionCompletedMessage, error) {
	var evt ExecutionCompletedMessage
	if err := json.Unmarshal(m.Value, &evt); err != nil {
		return nil, err
	}
	return &evt, nil
}
