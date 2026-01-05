package kafka

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/Ramsey-B/ivy/pkg/models"
)

// DebeziumEnvelope is the standard Debezium CDC message format
type DebeziumEnvelope struct {
	Schema  json.RawMessage `json:"schema,omitempty"`
	Payload DebeziumPayload `json:"payload"`
}

// DebeziumPayload contains the before/after state of a row
type DebeziumPayload struct {
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
	Source DebeziumSource  `json:"source"`
	Op     string          `json:"op"` // c=create, u=update, d=delete, r=read (snapshot)
	TsMs   int64           `json:"ts_ms"`
	TsUsMs int64           `json:"ts_us,omitempty"`
	TsNsMs int64           `json:"ts_ns,omitempty"`
}

// DebeziumSource contains metadata about the source of the change
type DebeziumSource struct {
	Version   string `json:"version"`
	Connector string `json:"connector"`
	Name      string `json:"name"`
	TsMs      int64  `json:"ts_ms"`
	Snapshot  string `json:"snapshot,omitempty"`
	Db        string `json:"db"`
	Sequence  string `json:"sequence,omitempty"`
	Schema    string `json:"schema"`
	Table     string `json:"table"`
	TxId      int64  `json:"txId,omitempty"`
	Lsn       int64  `json:"lsn,omitempty"`
	Xmin      *int64 `json:"xmin,omitempty"`
}

// IsCreate returns true if this is a create operation
func (p *DebeziumPayload) IsCreate() bool {
	return p.Op == "c" || p.Op == "r"
}

// IsUpdate returns true if this is an update operation
func (p *DebeziumPayload) IsUpdate() bool {
	return p.Op == "u"
}

// IsDelete returns true if this is a delete operation
func (p *DebeziumPayload) IsDelete() bool {
	return p.Op == "d"
}

// StagedEntityRow represents a row from the staged_entities table
type StagedEntityRow struct {
	ID                  string          `json:"id"`
	TenantID            string          `json:"tenant_id"`
	EntityType          string          `json:"entity_type"`
	SourceID            string          `json:"source_id"`
	Integration         string          `json:"integration"`
	SourceKey           string          `json:"source_key"`
	ConfigID            string          `json:"config_id"`
	SourceExecutionID   *string         `json:"source_execution_id"`
	ExecutionID         *string         `json:"execution_id"`
	LastSeenExecution   *string         `json:"last_seen_execution"`
	Data                json.RawMessage `json:"data"`
	Fingerprint         string          `json:"fingerprint"`
	PreviousFingerprint string          `json:"previous_fingerprint"`
	CreatedAt           string          `json:"created_at"`
	UpdatedAt           string          `json:"updated_at"`
	DeletedAt           *string         `json:"deleted_at"`
}

// ToStagedEntity converts the Debezium row to a StagedEntity model.
// Returns nil if the ID cannot be parsed.
func (r *StagedEntityRow) ToStagedEntity() *models.StagedEntity {
	entity := &models.StagedEntity{
		ID:                  r.ID,
		TenantID:            r.TenantID,
		EntityType:          r.EntityType,
		SourceID:            r.SourceID,
		Integration:         r.Integration,
		SourceKey:           r.SourceKey,
		ConfigID:            r.ConfigID,
		SourceExecutionID:   r.SourceExecutionID,
		ExecutionID:         r.ExecutionID,
		LastSeenExecution:   r.LastSeenExecution,
		Data:                r.Data,
		Fingerprint:         r.Fingerprint,
		PreviousFingerprint: r.PreviousFingerprint,
		CreatedAt:           parseDebeziumTimestamp(r.CreatedAt),
		UpdatedAt:           parseDebeziumTimestamp(r.UpdatedAt),
		DeletedAt:           parseDebeziumTimestampPtr(r.DeletedAt),
	}

	return entity
}

// parseDebeziumTimestamp parses a timestamp string from Debezium.
// Debezium can send timestamps in various formats depending on the connector config.
func parseDebeziumTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}

	// Try common formats Debezium uses
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05.999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	// Try parsing as Unix microseconds (Debezium io.debezium.time.MicroTimestamp)
	if len(s) > 10 {
		// Could be microseconds since epoch
		return time.Time{}
	}

	return time.Time{}
}

// parseDebeziumTimestampPtr parses an optional timestamp string.
func parseDebeziumTimestampPtr(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t := parseDebeziumTimestamp(*s)
	if t.IsZero() {
		return nil
	}
	return &t
}

// IsDeleted returns true if the row has been soft-deleted
func (r *StagedEntityRow) IsDeleted() bool {
	return r.DeletedAt != nil && *r.DeletedAt != ""
}

// StagedRelationshipRow represents a row from the staged_relationships table
type StagedRelationshipRow struct {
	ID                 string          `json:"id"`
	TenantID           string          `json:"tenant_id"`
	ConfigID           string          `json:"config_id"`
	RelationshipType   string          `json:"relationship_type"`
	FromStagedEntityID *string         `json:"from_staged_entity_id"`
	ToStagedEntityID   *string         `json:"to_staged_entity_id"`
	FromEntityType     string          `json:"from_entity_type"`
	FromSourceID       string          `json:"from_source_id"`
	FromSourceField    string          `json:"from_source_field"`
	ToEntityType       string          `json:"to_entity_type"`
	ToSourceID         string          `json:"to_source_id"`
	ToSourceField      string          `json:"to_source_field"`
	Integration        string          `json:"integration"`
	SourceKey          string          `json:"source_key"`
	SourceExecutionID  *string         `json:"source_execution_id"`
	Data               json.RawMessage `json:"data"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`
	DeletedAt          *string         `json:"deleted_at"`
}

// IsDeleted returns true if the row has been soft-deleted
func (r *StagedRelationshipRow) IsDeleted() bool {
	return r.DeletedAt != nil && *r.DeletedAt != ""
}

// GetFromEntityUUID parses the from entity ID as a UUID
func (r *StagedRelationshipRow) GetFromEntityUUID() *string {
	if r.FromStagedEntityID == nil {
		return nil
	}
	return r.FromStagedEntityID
}

// GetToEntityUUID parses the to entity ID as a UUID
func (r *StagedRelationshipRow) GetToEntityUUID() *string {
	if r.ToStagedEntityID == nil {
		return nil
	}
	return r.ToStagedEntityID
}

// ToStagedRelationship converts the Debezium row to a StagedRelationship model.
// Returns nil if the ID cannot be parsed.
func (r *StagedRelationshipRow) ToStagedRelationship() *models.StagedRelationship {
	rel := &models.StagedRelationship{
		ID:                 r.ID,
		TenantID:           r.TenantID,
		ConfigID:           r.ConfigID,
		RelationshipType:   r.RelationshipType,
		FromEntityType:     r.FromEntityType,
		FromSourceID:       r.FromSourceID,
		ToEntityType:       r.ToEntityType,
		ToSourceID:         r.ToSourceID,
		FromStagedEntityID: r.GetFromEntityUUID(),
		ToStagedEntityID:   r.GetToEntityUUID(),
		Integration:        r.Integration,
		SourceKey:          r.SourceKey,
		SourceExecutionID:  r.SourceExecutionID,
		Data:               r.Data,
		CreatedAt:          parseDebeziumTimestamp(r.CreatedAt),
		UpdatedAt:          parseDebeziumTimestamp(r.UpdatedAt),
		DeletedAt:          parseDebeziumTimestampPtr(r.DeletedAt),
	}

	return rel
}

// ParseDebeziumMessage parses a raw Kafka message as a Debezium envelope
func ParseDebeziumMessage(data []byte) (*DebeziumEnvelope, error) {
	var envelope DebeziumEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

func unwrapJSONStringJSON(raw json.RawMessage) (json.RawMessage, error) {
	raw = json.RawMessage(bytes.TrimSpace(raw))
	if len(raw) == 0 {
		return raw, nil
	}
	if raw[0] != '"' {
		return raw, nil // already object/array/etc.
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return json.RawMessage(s), nil
}

// ParseStagedEntityRow parses the After payload as a StagedEntityRow
func (p *DebeziumPayload) ParseStagedEntityRow() (*StagedEntityRow, error) {
	if len(p.After) == 0 || string(p.After) == "null" {
		return nil, nil
	}

	var row StagedEntityRow
	if err := json.Unmarshal(p.After, &row); err != nil {
		return nil, err
	}

	unwrapped, err := unwrapJSONStringJSON(row.Data)
	if err != nil {
		return nil, err
	}
	row.Data = unwrapped

	return &row, nil
}

// ParseStagedRelationshipRow parses the After payload as a StagedRelationshipRow
func (p *DebeziumPayload) ParseStagedRelationshipRow() (*StagedRelationshipRow, error) {
	if len(p.After) == 0 || string(p.After) == "null" {
		return nil, nil
	}
	var row StagedRelationshipRow
	if err := json.Unmarshal(p.After, &row); err != nil {
		return nil, err
	}
	return &row, nil
}

// Timestamp returns the event timestamp
func (p *DebeziumPayload) Timestamp() time.Time {
	return time.UnixMilli(p.TsMs)
}
