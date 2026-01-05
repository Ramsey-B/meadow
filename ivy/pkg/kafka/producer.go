package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/segmentio/kafka-go"

	"github.com/Ramsey-B/ivy/pkg/models"
)

// Producer handles Kafka event emission
type Producer struct {
	writer *kafka.Writer
	logger ectologger.Logger
	topic  string
}

// ProducerConfig holds Kafka producer configuration
type ProducerConfig struct {
	Brokers      []string
	Topic        string
	BatchSize    int
	BatchTimeout time.Duration
	RequiredAcks int
	Compression  string
}

// NewProducer creates a new Kafka producer
func NewProducer(cfg ProducerConfig, logger ectologger.Logger) *Producer {
	compression := kafka.Snappy
	switch cfg.Compression {
	case "gzip":
		compression = kafka.Gzip
	case "lz4":
		compression = kafka.Lz4
	case "zstd":
		compression = kafka.Zstd
	case "none":
		compression = 0
	}

	requiredAcks := kafka.RequiredAcks(cfg.RequiredAcks)

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Balancer:               &kafka.LeastBytes{},
		BatchSize:              cfg.BatchSize,
		BatchTimeout:           cfg.BatchTimeout,
		RequiredAcks:           requiredAcks,
		Compression:            compression,
		AllowAutoTopicCreation: true,
	}

	return &Producer{
		writer: writer,
		logger: logger,
		topic:  cfg.Topic,
	}
}

// Close closes the producer
func (p *Producer) Close() error {
	return p.writer.Close()
}

// EntityEvent represents an event about an entity
type EntityEvent struct {
	EventType      string          `json:"event_type"` // created, updated, deleted, merged
	TenantID       string          `json:"tenant_id"`
	EntityID       string          `json:"entity_id"`
	EntityType     string          `json:"entity_type"`
	Data           json.RawMessage `json:"data,omitempty"`
	SourceEntities []string        `json:"source_entities,omitempty"`
	Version        int             `json:"version"`
	Timestamp      time.Time       `json:"timestamp"`
}

// RelationshipEvent represents an event about a relationship
type RelationshipEvent struct {
	EventType        string          `json:"event_type"` // created, updated, deleted
	TenantID         string          `json:"tenant_id"`
	RelationshipID   string          `json:"relationship_id"`
	RelationshipType string          `json:"relationship_type"`
	FromEntityID     string          `json:"from_entity_id"`
	FromEntityType   string          `json:"from_entity_type"`
	ToEntityID       string          `json:"to_entity_id"`
	ToEntityType     string          `json:"to_entity_type"`
	Properties       json.RawMessage `json:"properties,omitempty"`
	Timestamp        time.Time       `json:"timestamp"`
}

// PublishEntityEvent publishes an entity event to Kafka
func (p *Producer) PublishEntityEvent(ctx context.Context, event *EntityEvent) error {
	ctx, span := tracing.StartSpan(ctx, "kafka.Producer.PublishEntityEvent")
	defer span.End()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Topic: p.topic,
		Key:   []byte(event.EntityID),
		Value: data,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "tenant_id", Value: []byte(event.TenantID)},
			{Key: "entity_type", Value: []byte(event.EntityType)},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to publish entity event")
		return err
	}

	p.logger.WithContext(ctx).WithFields(map[string]any{
		"event_type":  event.EventType,
		"entity_id":   event.EntityID,
		"entity_type": event.EntityType,
	}).Debug("Published entity event")

	return nil
}

// PublishRelationshipEvent publishes a relationship event to Kafka
func (p *Producer) PublishRelationshipEvent(ctx context.Context, event *RelationshipEvent) error {
	ctx, span := tracing.StartSpan(ctx, "kafka.Producer.PublishRelationshipEvent")
	defer span.End()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Topic: p.topic,
		Key:   []byte(event.RelationshipID),
		Value: data,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "tenant_id", Value: []byte(event.TenantID)},
			{Key: "relationship_type", Value: []byte(event.RelationshipType)},
		},
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to publish relationship event")
		return err
	}

	p.logger.WithContext(ctx).WithFields(map[string]any{
		"event_type":        event.EventType,
		"relationship_id":   event.RelationshipID,
		"relationship_type": event.RelationshipType,
	}).Debug("Published relationship event")

	return nil
}

// PublishMergeResult publishes a merge result as an entity event
func (p *Producer) PublishMergeResult(ctx context.Context, result *models.MergeResult) error {
	ctx, span := tracing.StartSpan(ctx, "kafka.Producer.PublishMergeResult")
	defer span.End()

	eventType := "entity.merged"
	if result.IsNew {
		eventType = "entity.created"
	}

	event := &EntityEvent{
		EventType:      eventType,
		TenantID:       result.MergedEntity.TenantID,
		EntityID:       result.MergedEntity.ID,
		EntityType:     result.MergedEntity.EntityType,
		Data:           result.MergedEntity.Data,
		SourceEntities: result.SourceEntities,
		Version:        result.Version,
	}

	return p.PublishEntityEvent(ctx, event)
}

// PublishEntityEvents publishes multiple entity events in a batch
func (p *Producer) PublishEntityEvents(ctx context.Context, events []*EntityEvent) error {
	ctx, span := tracing.StartSpan(ctx, "kafka.Producer.PublishEntityEvents")
	defer span.End()

	if len(events) == 0 {
		return nil
	}

	messages := make([]kafka.Message, len(events))
	for i, event := range events {
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now().UTC()
		}

		data, err := json.Marshal(event)
		if err != nil {
			return err
		}

		messages[i] = kafka.Message{
			Topic: p.topic,
			Key:   []byte(event.EntityID),
			Value: data,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(event.EventType)},
				{Key: "tenant_id", Value: []byte(event.TenantID)},
				{Key: "entity_type", Value: []byte(event.EntityType)},
				{Key: "schema_version", Value: []byte("1.0")},
			},
		}
	}

	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		p.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"batch_size": len(events),
		}).Error("Failed to publish entity events batch")
		return err
	}

	p.logger.WithContext(ctx).WithFields(map[string]any{
		"batch_size": len(events),
	}).Debug("Published entity events batch")

	return nil
}

// PublishRelationshipEvents publishes multiple relationship events in a batch
func (p *Producer) PublishRelationshipEvents(ctx context.Context, events []*RelationshipEvent) error {
	ctx, span := tracing.StartSpan(ctx, "kafka.Producer.PublishRelationshipEvents")
	defer span.End()

	if len(events) == 0 {
		return nil
	}

	messages := make([]kafka.Message, len(events))
	for i, event := range events {
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now().UTC()
		}

		data, err := json.Marshal(event)
		if err != nil {
			return err
		}

		messages[i] = kafka.Message{
			Topic: p.topic,
			Key:   []byte(event.RelationshipID),
			Value: data,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(event.EventType)},
				{Key: "tenant_id", Value: []byte(event.TenantID)},
				{Key: "relationship_type", Value: []byte(event.RelationshipType)},
				{Key: "schema_version", Value: []byte("1.0")},
			},
		}
	}

	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		p.logger.WithContext(ctx).WithError(err).WithFields(map[string]any{
			"batch_size": len(events),
		}).Error("Failed to publish relationship events batch")
		return err
	}

	p.logger.WithContext(ctx).WithFields(map[string]any{
		"batch_size": len(events),
	}).Debug("Published relationship events batch")

	return nil
}
