package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/Ramsey-B/stem/pkg/tracing"
)

// Config holds Kafka configuration
type Config struct {
	Brokers       []string
	ResponseTopic string
	ErrorTopic    string
}

// ParseConfig parses a comma-separated broker string
func ParseConfig(brokers string, responseTopic string, errorTopic string) Config {
	brokerList := strings.Split(brokers, ",")
	for i := range brokerList {
		brokerList[i] = strings.TrimSpace(brokerList[i])
	}

	return Config{
		Brokers:       brokerList,
		ResponseTopic: responseTopic,
		ErrorTopic:    errorTopic,
	}
}

// Producer handles producing messages to Kafka
type Producer struct {
	writer      *kafka.Writer
	errorWriter *kafka.Writer
	logger      ectologger.Logger
	topic       string
	errorTopic  string
}

// NewProducer creates a new Kafka producer
func NewProducer(cfg Config, logger ectologger.Logger) *Producer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.ResponseTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		// Allow Kafka to auto-create the topic in dev environments when it doesn't exist yet.
		// Without this, a first publish may fail with "Unknown Topic Or Partition".
		AllowAutoTopicCreation: true,
	}

	errorWriter := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Topic:                  cfg.ErrorTopic,
		Balancer:               &kafka.LeastBytes{},
		BatchSize:              100,
		BatchTimeout:           10 * time.Millisecond,
		RequiredAcks:           kafka.RequireOne,
		Async:                  false,
		AllowAutoTopicCreation: true,
	}

	return &Producer{
		writer:      writer,
		errorWriter: errorWriter,
		logger:      logger,
		topic:       cfg.ResponseTopic,
		errorTopic:  cfg.ErrorTopic,
	}
}

// Close closes the producer
func (p *Producer) Close() error {
	var firstErr error
	if err := p.writer.Close(); err != nil {
		firstErr = err
	}
	if p.errorWriter != nil {
		if err := p.errorWriter.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// APIResponseMessage represents an API response message for Kafka
type APIResponseMessage struct {
	// Metadata
	TenantID    string    `json:"tenant_id"`
	Integration string    `json:"integration"`
	PlanKey     string    `json:"plan_key"`
	ConfigID    string    `json:"config_id"`
	ExecutionID string    `json:"execution_id"`
	StepPath    string    `json:"step_path"`
	Timestamp   time.Time `json:"timestamp"`

	// Tracing
	TraceID string `json:"trace_id,omitempty"`
	SpanID  string `json:"span_id,omitempty"`

	// Request details
	RequestURL     string            `json:"request_url"`
	RequestMethod  string            `json:"request_method"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`

	// Response details
	StatusCode      int               `json:"status_code"`
	ResponseBody    json.RawMessage   `json:"response_body"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseSize    int64             `json:"response_size"`
	DurationMs      int64             `json:"duration_ms"`

	// Extracted data (optional)
	ExtractedData map[string]any `json:"extracted_data,omitempty"`
}

// ExecutionEventMessage is a lifecycle event for a plan execution.
// These are intended for downstream services (Lotus/Ivy) to coordinate execution-based deletion.
type ExecutionEventMessage struct {
	Type        string    `json:"type"` // "execution.started" | "execution.completed"
	TenantID    string    `json:"tenant_id"`
	Integration string    `json:"integration"`
	PlanKey     string    `json:"plan_key"`
	ConfigID    string    `json:"config_id,omitempty"`
	ExecutionID string    `json:"execution_id"`
	Status      string    `json:"status,omitempty"` // e.g. "running", "success", "failed", "aborted"
	Timestamp   time.Time `json:"timestamp"`
}

func (p *Producer) PublishExecutionEvent(ctx context.Context, evt *ExecutionEventMessage) error {
	if evt == nil {
		return fmt.Errorf("execution event is nil")
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal execution event: %w", err)
	}

	key := fmt.Sprintf("%s:%s", evt.TenantID, evt.ExecutionID)
	headers := []kafka.Header{
		{Key: "tenant_id", Value: []byte(evt.TenantID)},
		{Key: "integration", Value: []byte(evt.Integration)},
		{Key: "plan_key", Value: []byte(evt.PlanKey)},
		{Key: "execution_id", Value: []byte(evt.ExecutionID)},
		{Key: "type", Value: []byte(evt.Type)},
	}
	if traceparent := tracing.GetTraceParent(ctx); traceparent != "" {
		headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(traceparent)})
	}
	if tracestate := tracing.GetTraceState(ctx); tracestate != "" {
		headers = append(headers, kafka.Header{Key: "tracestate", Value: []byte(tracestate)})
	}

	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:     []byte(key),
		Value:   data,
		Headers: headers,
	}); err != nil {
		p.logger.WithContext(ctx).WithError(err).Errorf("Failed to publish execution event to Kafka topic %s", p.topic)
		return err
	}

	return nil
}

// Publish publishes an API response message to Kafka
func (p *Producer) Publish(ctx context.Context, msg *APIResponseMessage) error {
	ctx, span := tracing.StartSpan(ctx, "Kafka.Publish")
	defer span.End()

	// Add span attributes
	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", p.topic),
		attribute.String("messaging.operation", "publish"),
		attribute.String("tenant_id", msg.TenantID),
		attribute.String("integration", msg.Integration),
		attribute.String("plan_key", msg.PlanKey),
		attribute.String("execution_id", msg.ExecutionID),
	)

	// Inject trace context into the message
	msg.TraceID = tracing.GetTraceID(ctx)
	msg.SpanID = tracing.GetSpanID(ctx)

	data, err := json.Marshal(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal message")
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Use tenant_id + execution_id as key for partitioning
	key := fmt.Sprintf("%s:%s", msg.TenantID, msg.ExecutionID)

	// Build headers with trace context
	headers := []kafka.Header{
		{Key: "tenant_id", Value: []byte(msg.TenantID)},
		{Key: "integration", Value: []byte(msg.Integration)},
		{Key: "plan_key", Value: []byte(msg.PlanKey)},
		{Key: "execution_id", Value: []byte(msg.ExecutionID)},
	}

	// Add W3C trace context headers for distributed tracing
	if traceparent := tracing.GetTraceParent(ctx); traceparent != "" {
		headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(traceparent)})
	}
	if tracestate := tracing.GetTraceState(ctx); tracestate != "" {
		headers = append(headers, kafka.Header{Key: "tracestate", Value: []byte(tracestate)})
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:     []byte(key),
		Value:   data,
		Headers: headers,
	})

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to publish message")
		p.logger.WithContext(ctx).WithError(err).Errorf("Failed to publish to Kafka topic %s", p.topic)
		return err
	}

	span.SetStatus(codes.Ok, "message published")
	p.logger.WithContext(ctx).Debugf("Published API response to Kafka: execution=%s step=%s status=%d trace=%s",
		msg.ExecutionID, msg.StepPath, msg.StatusCode, msg.TraceID)

	return nil
}

// PublishError publishes an API response message to the error topic.
func (p *Producer) PublishError(ctx context.Context, msg *APIResponseMessage) error {
	ctx, span := tracing.StartSpan(ctx, "Kafka.PublishError")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", p.errorTopic),
		attribute.String("messaging.operation", "publish"),
		attribute.String("tenant_id", msg.TenantID),
		attribute.String("integration", msg.Integration),
		attribute.String("plan_key", msg.PlanKey),
		attribute.String("execution_id", msg.ExecutionID),
	)

	msg.TraceID = tracing.GetTraceID(ctx)
	msg.SpanID = tracing.GetSpanID(ctx)

	data, err := json.Marshal(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal message")
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	key := fmt.Sprintf("%s:%s", msg.TenantID, msg.ExecutionID)

	headers := []kafka.Header{
		{Key: "tenant_id", Value: []byte(msg.TenantID)},
		{Key: "integration", Value: []byte(msg.Integration)},
		{Key: "plan_key", Value: []byte(msg.PlanKey)},
		{Key: "execution_id", Value: []byte(msg.ExecutionID)},
	}
	if traceparent := tracing.GetTraceParent(ctx); traceparent != "" {
		headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(traceparent)})
	}
	if tracestate := tracing.GetTraceState(ctx); tracestate != "" {
		headers = append(headers, kafka.Header{Key: "tracestate", Value: []byte(tracestate)})
	}

	if p.errorWriter == nil {
		return fmt.Errorf("errorWriter is nil (error topic not configured)")
	}

	if err := p.errorWriter.WriteMessages(ctx, kafka.Message{
		Key:     []byte(key),
		Value:   data,
		Headers: headers,
	}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to publish message")
		p.logger.WithContext(ctx).WithError(err).Errorf("Failed to publish to Kafka error topic %s", p.errorTopic)
		return err
	}

	span.SetStatus(codes.Ok, "message published")
	p.logger.WithContext(ctx).Debugf("Published API response to Kafka error topic: execution=%s step=%s status=%d trace=%s",
		msg.ExecutionID, msg.StepPath, msg.StatusCode, msg.TraceID)
	return nil
}

// PublishBatch publishes multiple messages in a batch
func (p *Producer) PublishBatch(ctx context.Context, messages []*APIResponseMessage) error {
	if len(messages) == 0 {
		return nil
	}

	ctx, span := tracing.StartSpan(ctx, "Kafka.PublishBatch")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination", p.topic),
		attribute.String("messaging.operation", "publish"),
		attribute.Int("messaging.batch_size", len(messages)),
		attribute.String("integration", messages[0].Integration),
	)

	// Get trace context for all messages in the batch
	traceID := tracing.GetTraceID(ctx)
	spanID := tracing.GetSpanID(ctx)
	traceparent := tracing.GetTraceParent(ctx)
	tracestate := tracing.GetTraceState(ctx)

	kafkaMessages := make([]kafka.Message, len(messages))

	for i, msg := range messages {
		// Inject trace context
		msg.TraceID = traceID
		msg.SpanID = spanID

		data, err := json.Marshal(msg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, fmt.Sprintf("failed to marshal message %d", i))
			return fmt.Errorf("failed to marshal message %d: %w", i, err)
		}

		key := fmt.Sprintf("%s:%s", msg.TenantID, msg.ExecutionID)

		headers := []kafka.Header{
			{Key: "tenant_id", Value: []byte(msg.TenantID)},
			{Key: "integration", Value: []byte(msg.Integration)},
			{Key: "plan_key", Value: []byte(msg.PlanKey)},
			{Key: "execution_id", Value: []byte(msg.ExecutionID)},
		}

		if traceparent != "" {
			headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(traceparent)})
		}
		if tracestate != "" {
			headers = append(headers, kafka.Header{Key: "tracestate", Value: []byte(tracestate)})
		}

		kafkaMessages[i] = kafka.Message{
			Key:     []byte(key),
			Value:   data,
			Headers: headers,
		}
	}

	err := p.writer.WriteMessages(ctx, kafkaMessages...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to publish batch")
		p.logger.WithContext(ctx).WithError(err).Errorf("Failed to publish batch to Kafka topic %s", p.topic)
		return err
	}

	span.SetStatus(codes.Ok, "batch published")
	p.logger.WithContext(ctx).Infof("Published %d API responses to Kafka", len(messages))
	return nil
}

// Stats returns producer statistics
func (p *Producer) Stats() kafka.WriterStats {
	return p.writer.Stats()
}
