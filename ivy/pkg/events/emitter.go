// Package events handles event emission for entity lifecycle changes
package events

import (
	"context"
	"encoding/json"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"

	"github.com/Ramsey-B/ivy/pkg/kafka"
	"github.com/Ramsey-B/ivy/pkg/models"
)

// SchemaVersion is the current event schema version
const SchemaVersion = "1.0"

// Emitter handles event emission for Ivy
type Emitter struct {
	producer *kafka.Producer
	logger   ectologger.Logger
}

// NewEmitter creates a new event emitter
func NewEmitter(producer *kafka.Producer, logger ectologger.Logger) *Emitter {
	return &Emitter{
		producer: producer,
		logger:   logger,
	}
}

// EmitEntityCreated emits an entity created event
func (e *Emitter) EmitEntityCreated(ctx context.Context, entity *models.MergedEntity, sourceIDs []string) error {
	ctx, span := tracing.StartSpan(ctx, "events.Emitter.EmitEntityCreated")
	defer span.End()

	event := &kafka.EntityEvent{
		EventType:      "entity.created",
		TenantID:       entity.TenantID,
		EntityID:       entity.ID,
		EntityType:     entity.EntityType,
		Data:           entity.Data,
		SourceEntities: sourceIDs,
		Version:        entity.Version,
	}

	if err := e.producer.PublishEntityEvent(ctx, event); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to emit entity.created event")
		return err
	}

	return nil
}

// EmitEntityUpdated emits an entity updated event
func (e *Emitter) EmitEntityUpdated(ctx context.Context, entity *models.MergedEntity, sourceIDs []string) error {
	ctx, span := tracing.StartSpan(ctx, "events.Emitter.EmitEntityUpdated")
	defer span.End()

	event := &kafka.EntityEvent{
		EventType:      "entity.updated",
		TenantID:       entity.TenantID,
		EntityID:       entity.ID,
		EntityType:     entity.EntityType,
		Data:           entity.Data,
		SourceEntities: sourceIDs,
		Version:        entity.Version,
	}

	if err := e.producer.PublishEntityEvent(ctx, event); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to emit entity.updated event")
		return err
	}

	return nil
}

// EmitEntityDeleted emits an entity deleted event
func (e *Emitter) EmitEntityDeleted(ctx context.Context, tenantID string, entityID string, entityType string, version int) error {
	ctx, span := tracing.StartSpan(ctx, "events.Emitter.EmitEntityDeleted")
	defer span.End()

	event := &kafka.EntityEvent{
		EventType:  "entity.deleted",
		TenantID:   tenantID,
		EntityID:   entityID,
		EntityType: entityType,
		Version:    version,
	}

	if err := e.producer.PublishEntityEvent(ctx, event); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to emit entity.deleted event")
		return err
	}

	return nil
}

// EmitEntityMerged emits an entity merged event with details about the merge
func (e *Emitter) EmitEntityMerged(ctx context.Context, result *models.MergeResult) error {
	ctx, span := tracing.StartSpan(ctx, "events.Emitter.EmitEntityMerged")
	defer span.End()

	// Build merge metadata
	mergeData := map[string]any{
		"schema_version": SchemaVersion,
		"is_new":         result.IsNew,
		"source_count":   len(result.SourceEntities),
		"data":           json.RawMessage(result.MergedEntity.Data),
	}

	if len(result.Conflicts) > 0 {
		mergeData["conflicts"] = result.Conflicts
	}

	dataJSON, _ := json.Marshal(mergeData)

	event := &kafka.EntityEvent{
		EventType:      "entity.merged",
		TenantID:       result.MergedEntity.TenantID,
		EntityID:       result.MergedEntity.ID,
		EntityType:     result.MergedEntity.EntityType,
		Data:           dataJSON,
		SourceEntities: result.SourceEntities,
		Version:        result.Version,
	}

	if err := e.producer.PublishEntityEvent(ctx, event); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to emit entity.merged event")
		return err
	}

	return nil
}

// EmitRelationshipCreated emits a relationship created event
func (e *Emitter) EmitRelationshipCreated(ctx context.Context, tenantID string, relID string, relType string, fromID, toID string, fromType, toType string, props json.RawMessage) error {
	ctx, span := tracing.StartSpan(ctx, "events.Emitter.EmitRelationshipCreated")
	defer span.End()

	event := &kafka.RelationshipEvent{
		EventType:        "relationship.created",
		TenantID:         tenantID,
		RelationshipID:   relID,
		RelationshipType: relType,
		FromEntityID:     fromID,
		FromEntityType:   fromType,
		ToEntityID:       toID,
		ToEntityType:     toType,
		Properties:       props,
	}

	if err := e.producer.PublishRelationshipEvent(ctx, event); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to emit relationship.created event")
		return err
	}

	return nil
}

// EmitRelationshipDeleted emits a relationship deleted event
func (e *Emitter) EmitRelationshipDeleted(ctx context.Context, tenantID string, relID string, relType string) error {
	ctx, span := tracing.StartSpan(ctx, "events.Emitter.EmitRelationshipDeleted")
	defer span.End()

	event := &kafka.RelationshipEvent{
		EventType:        "relationship.deleted",
		TenantID:         tenantID,
		RelationshipID:   relID,
		RelationshipType: relType,
	}

	if err := e.producer.PublishRelationshipEvent(ctx, event); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to emit relationship.deleted event")
		return err
	}

	return nil
}

// EmitMatchCandidate emits an event when a match candidate is identified
func (e *Emitter) EmitMatchCandidate(ctx context.Context, tenantID string, entityAID, entityBID string, score float64, status string) error {
	ctx, span := tracing.StartSpan(ctx, "events.Emitter.EmitMatchCandidate")
	defer span.End()

	data := map[string]any{
		"schema_version": SchemaVersion,
		"entity_a_id":    entityAID,
		"entity_b_id":    entityBID,
		"score":          score,
		"status":         status,
	}
	dataJSON, _ := json.Marshal(data)

	event := &kafka.EntityEvent{
		EventType:  "match.candidate",
		TenantID:   tenantID,
		EntityID:   entityAID, // Use entity A as the key
		EntityType: "match_candidate",
		Data:       dataJSON,
		Version:    1,
	}

	if err := e.producer.PublishEntityEvent(ctx, event); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to emit match.candidate event")
		return err
	}

	return nil
}
