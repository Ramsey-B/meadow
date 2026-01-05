package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Gobusters/ectologger"
	"github.com/segmentio/kafka-go"
)

// MessageHandler is called for each message received from Kafka
type MessageHandler func(ctx context.Context, msg *ReceivedMessage) error

// ReceivedMessage wraps a Kafka message with parsed data
type ReceivedMessage struct {
	// Raw Kafka message data
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   MessageHeaders

	// Parsed Orchid message (if applicable)
	OrchidMessage *OrchidMessage

	// Parsed as generic map (for direct mapping execution)
	Data map[string]any
}

// Consumer consumes messages from Kafka
type Consumer struct {
	reader  *kafka.Reader
	logger  ectologger.Logger
	config  ConsumerConfig
	handler MessageHandler
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	running bool
	mu      sync.Mutex
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(config ConsumerConfig, logger ectologger.Logger) (*Consumer, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}
	if config.Topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	if config.GroupID == "" {
		return nil, fmt.Errorf("group ID is required")
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:           config.Brokers,
		Topic:             config.Topic,
		GroupID:           config.GroupID,
		MinBytes:          config.MinBytes,
		MaxBytes:          config.MaxBytes,
		MaxWait:           config.MaxWait,
		CommitInterval:    config.CommitInterval,
		StartOffset:       config.StartOffset,
		SessionTimeout:    config.SessionTimeout,
		HeartbeatInterval: config.HeartbeatInterval,
		RebalanceTimeout:  config.RebalanceTimeout,
	})

	return &Consumer{
		reader: reader,
		logger: logger,
		config: config,
	}, nil
}

// Start begins consuming messages in the background
func (c *Consumer) Start(ctx context.Context, handler MessageHandler) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("consumer is already running")
	}
	c.running = true
	c.handler = handler
	c.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go c.consumeLoop(ctx)

	c.logger.Infof("Kafka consumer started for topic %s (group: %s)", c.config.Topic, c.config.GroupID)
	return nil
}

// Stop gracefully stops the consumer
func (c *Consumer) Stop() error {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = false
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	c.wg.Wait()

	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("failed to close reader: %w", err)
	}

	c.logger.Info("Kafka consumer stopped")
	return nil
}

// consumeLoop continuously fetches and processes messages
func (c *Consumer) consumeLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Fetch message
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, exit gracefully
			}
			c.logger.WithError(err).Error("Failed to fetch message")
			continue
		}

		// Process message
		received, err := c.parseMessage(msg)
		if err != nil {
			c.logger.WithError(err).Errorf("Failed to parse message at offset %d", msg.Offset)
			// Commit anyway to avoid getting stuck on bad messages
			if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
				c.logger.WithError(commitErr).Error("Failed to commit bad message")
			}
			continue
		}

		// Call handler
		if err := c.handler(ctx, received); err != nil {
			c.logger.WithError(err).Errorf("Handler failed for message at offset %d", msg.Offset)
			// Depending on your error handling strategy, you may want to:
			// - Retry the message
			// - Send to DLQ
			// - Continue and commit anyway
			// For now, we continue and commit to avoid getting stuck
		}

		// Commit offset
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.WithError(err).Errorf("Failed to commit message at offset %d", msg.Offset)
		}
	}
}

// parseMessage parses a raw Kafka message into ReceivedMessage
func (c *Consumer) parseMessage(msg kafka.Message) (*ReceivedMessage, error) {
	received := &ReceivedMessage{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       msg.Key,
		Value:     msg.Value,
	}

	// Extract headers
	kafkaHeaders := make([]Header, len(msg.Headers))
	for i, h := range msg.Headers {
		kafkaHeaders[i] = Header{Key: h.Key, Value: h.Value}
	}
	received.Headers = ExtractHeaders(kafkaHeaders)

	// Parse as generic map for mapping execution
	if err := json.Unmarshal(msg.Value, &received.Data); err != nil {
		return nil, fmt.Errorf("failed to parse message as JSON: %w", err)
	}

	// Try to parse as Orchid message
	orchidMsg, err := ParseOrchidMessage(msg.Value)
	if err == nil {
		received.OrchidMessage = orchidMsg
	}

	return received, nil
}

// Stats returns consumer statistics
func (c *Consumer) Stats() kafka.ReaderStats {
	return c.reader.Stats()
}

// Lag returns the current consumer lag
func (c *Consumer) Lag() int64 {
	return c.reader.Stats().Lag
}

