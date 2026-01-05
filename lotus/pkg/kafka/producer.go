package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/segmentio/kafka-go"
)

// Producer publishes messages to Kafka
type Producer struct {
	writer *kafka.Writer
	logger ectologger.Logger
	config ProducerConfig
}

// NewProducer creates a new Kafka producer
func NewProducer(config ProducerConfig, logger ectologger.Logger) (*Producer, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}

	var compression kafka.Compression
	switch config.Compression {
	case "gzip":
		compression = kafka.Gzip
	case "snappy":
		compression = kafka.Snappy
	case "lz4":
		compression = kafka.Lz4
	case "zstd":
		compression = kafka.Zstd
	default:
		compression = 0 // No compression
	}

	// NOTE: Do not set Topic on the Writer when you need to publish to multiple topics.
	// When Topic is set on Writer, individual messages cannot specify their own topic.
	// We leave Topic empty here so that each message can specify its destination topic.
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(config.Brokers...),
		Balancer:               &kafka.Hash{}, // Hash by key for partition affinity
		BatchSize:              config.BatchSize,
		BatchTimeout:           config.BatchTimeout,
		MaxAttempts:            config.MaxAttempts,
		WriteTimeout:           config.WriteTimeout,
		Async:                  config.Async,
		Compression:            compression,
		RequiredAcks:           kafka.RequiredAcks(config.RequiredAcks),
		AllowAutoTopicCreation: true,
	}

	return &Producer{
		writer: writer,
		logger: logger,
		config: config,
	}, nil
}

// Publish publishes a mapped message to Kafka
func (p *Producer) Publish(ctx context.Context, msg *MappedMessage) error {
	return p.PublishToTopic(ctx, p.config.Topic, msg)
}

// PublishToTopic publishes a mapped message to a specific topic
func (p *Producer) PublishToTopic(ctx context.Context, topic string, msg *MappedMessage) error {
	// Serialize message
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Build key for partition affinity (tenant:binding)
	key := fmt.Sprintf("%s:%s", msg.Source.TenantID, msg.BindingID)

	// Build headers
	headers := MessageHeaders{
		TenantID:  msg.Source.TenantID,
		BindingID: msg.BindingID,
	}
	if msg.TraceID != "" {
		headers.TraceParent = fmt.Sprintf("00-%s-%s-01", msg.TraceID, msg.SpanID)
	}

	kafkaHeaders := make([]kafka.Header, 0)
	for _, h := range headers.ToKafkaHeaders() {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{Key: h.Key, Value: h.Value})
	}

	// Create Kafka message
	kafkaMsg := kafka.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   data,
		Headers: kafkaHeaders,
		Time:    msg.Timestamp,
	}

	// Publish
	if err := p.writer.WriteMessages(ctx, kafkaMsg); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// PublishRawToTopic publishes raw JSON bytes to a topic, preserving the payload exactly.
// This is used for passthrough events like `execution.completed`.
func (p *Producer) PublishRawToTopic(ctx context.Context, topic string, key string, headers map[string]string, value []byte) error {
	kafkaHeaders := make([]kafka.Header, 0, len(headers))
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{Key: k, Value: []byte(v)})
	}

	kafkaMsg := kafka.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   value,
		Headers: kafkaHeaders,
		Time:    time.Now().UTC(),
	}

	if err := p.writer.WriteMessages(ctx, kafkaMsg); err != nil {
		return fmt.Errorf("failed to publish raw message: %w", err)
	}
	return nil
}

// PublishBatch publishes multiple messages in a batch
func (p *Producer) PublishBatch(ctx context.Context, messages []*MappedMessage) error {
	if len(messages) == 0 {
		return nil
	}

	kafkaMessages := make([]kafka.Message, 0, len(messages))

	for _, msg := range messages {
		data, err := msg.ToJSON()
		if err != nil {
			p.logger.WithError(err).Error("Failed to serialize message in batch, skipping")
			continue
		}

		key := fmt.Sprintf("%s:%s", msg.Source.TenantID, msg.BindingID)

		headers := MessageHeaders{
			TenantID:  msg.Source.TenantID,
			BindingID: msg.BindingID,
		}
		if msg.TraceID != "" {
			headers.TraceParent = fmt.Sprintf("00-%s-%s-01", msg.TraceID, msg.SpanID)
		}

		kafkaHeaders := make([]kafka.Header, 0)
		for _, h := range headers.ToKafkaHeaders() {
			kafkaHeaders = append(kafkaHeaders, kafka.Header{Key: h.Key, Value: h.Value})
		}

		kafkaMessages = append(kafkaMessages, kafka.Message{
			Topic:   p.config.Topic,
			Key:     []byte(key),
			Value:   data,
			Headers: kafkaHeaders,
			Time:    msg.Timestamp,
		})
	}

	if err := p.writer.WriteMessages(ctx, kafkaMessages...); err != nil {
		return fmt.Errorf("failed to publish batch: %w", err)
	}

	return nil
}

// Close closes the producer
func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("failed to close producer: %w", err)
	}
	p.logger.Info("Kafka producer closed")
	return nil
}

// Stats returns producer statistics
func (p *Producer) Stats() kafka.WriterStats {
	return p.writer.Stats()
}

// CreateMappedMessage creates a MappedMessage from an Orchid message and mapping result
func CreateMappedMessage(
	orchidMsg *OrchidMessage,
	bindingID string,
	mappingID string,
	mappingVersion int,
	mappedData map[string]any,
) *MappedMessage {
	return &MappedMessage{
		Source: MessageSource{
			Type:        "orchid",
			TenantID:    orchidMsg.TenantID,
			Integration: orchidMsg.Integration,
			Key:         orchidMsg.PlanKey,
			ConfigID:    orchidMsg.ConfigID,
			ExecutionID: orchidMsg.ExecutionID,
		},
		BindingID:      bindingID,
		MappingID:      mappingID,
		MappingVersion: mappingVersion,
		Timestamp:      time.Now().UTC(),
		Data:           mappedData,
		TraceID:        orchidMsg.TraceID,
		SpanID:         orchidMsg.SpanID,
	}
}
