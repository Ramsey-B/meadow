package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType defines the type of event
type EventType string

const (
	// Entity events
	EventTypeEntityCreated EventType = "entity.created"
	EventTypeEntityUpdated EventType = "entity.updated"
	EventTypeEntityDeleted EventType = "entity.deleted"
	EventTypeEntityMerged  EventType = "entity.merged"

	// Relationship events
	EventTypeRelationshipCreated EventType = "relationship.created"
	EventTypeRelationshipDeleted EventType = "relationship.deleted"

	// Match events
	EventTypeMatchCandidate EventType = "match.candidate"
	EventTypeMatchApproved  EventType = "match.approved"
	EventTypeMatchRejected  EventType = "match.rejected"
)

// BaseEvent contains common fields for all events
type BaseEvent struct {
	EventType     EventType `json:"event_type"`
	SchemaVersion string    `json:"schema_version"`
	TenantID      string    `json:"tenant_id"`
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id,omitempty"`
}

// EntityCreatedEvent is emitted when a new merged entity is created
type EntityCreatedEvent struct {
	BaseEvent
	EntityID       string       `json:"entity_id"`
	EntityType     string          `json:"entity_type"`
	Data           json.RawMessage `json:"data"`
	SourceEntities []string     `json:"source_entities"`
	Version        int             `json:"version"`
}

// EntityUpdatedEvent is emitted when a merged entity is updated
type EntityUpdatedEvent struct {
	BaseEvent
	EntityID       string       `json:"entity_id"`
	EntityType     string          `json:"entity_type"`
	Data           json.RawMessage `json:"data"`
	OldData        json.RawMessage `json:"old_data,omitempty"`
	SourceEntities []string     `json:"source_entities"`
	Version        int             `json:"version"`
	ChangedFields  []string        `json:"changed_fields,omitempty"`
}

// EntityDeletedEvent is emitted when a merged entity is deleted
type EntityDeletedEvent struct {
	BaseEvent
	EntityID   string `json:"entity_id"`
	EntityType string    `json:"entity_type"`
	Reason     string    `json:"reason"` // explicit, execution_based, staleness
	Version    int       `json:"version"`
}

// EntityMergedEvent is emitted when entities are merged
type EntityMergedEvent struct {
	BaseEvent
	MergedEntityID string       `json:"merged_entity_id"`
	EntityType     string          `json:"entity_type"`
	SourceEntities []string     `json:"source_entities"`
	Data           json.RawMessage `json:"data"`
	Conflicts      []ConflictInfo  `json:"conflicts,omitempty"`
	IsNew          bool            `json:"is_new"`
	Confidence     float64         `json:"confidence,omitempty"`
	Version        int             `json:"version"`
}

// ConflictInfo describes a merge conflict
type ConflictInfo struct {
	Field         string   `json:"field"`
	Values        []any    `json:"values"`
	Sources       []string `json:"sources"`
	Resolution    string   `json:"resolution"`
	ResolvedValue any      `json:"resolved_value"`
}

// RelationshipCreatedEvent is emitted when a relationship is created
type RelationshipCreatedEvent struct {
	BaseEvent
	RelationshipID   string       `json:"relationship_id"`
	RelationshipType string          `json:"relationship_type"`
	FromEntityID     string       `json:"from_entity_id"`
	FromEntityType   string          `json:"from_entity_type"`
	ToEntityID       string       `json:"to_entity_id"`
	ToEntityType     string          `json:"to_entity_type"`
	Properties       json.RawMessage `json:"properties,omitempty"`
}

// RelationshipDeletedEvent is emitted when a relationship is deleted
type RelationshipDeletedEvent struct {
	BaseEvent
	RelationshipID   string `json:"relationship_id"`
	RelationshipType string    `json:"relationship_type"`
	FromEntityID     string `json:"from_entity_id"`
	ToEntityID       string `json:"to_entity_id"`
	Reason           string    `json:"reason,omitempty"`
}

// MatchCandidateEvent is emitted when a potential match is identified
type MatchCandidateEvent struct {
	BaseEvent
	MatchID     string `json:"match_id,omitempty"`
	EntityAID   string `json:"entity_a_id"`
	EntityBID   string `json:"entity_b_id"`
	EntityType  string    `json:"entity_type"`
	Score       float64   `json:"score"`
	Status      string    `json:"status"` // pending, approved, rejected
	MatchedOn   []string  `json:"matched_on,omitempty"`
	RuleName    string    `json:"rule_name,omitempty"`
}

// NewBaseEvent creates a base event with common fields
func NewBaseEvent(eventType EventType, tenantID string) BaseEvent {
	return BaseEvent{
		EventType:     eventType,
		SchemaVersion: SchemaVersion,
		TenantID:      tenantID,
		Timestamp:     time.Now().UTC(),
		CorrelationID: uuid.New().String(),
	}
}

