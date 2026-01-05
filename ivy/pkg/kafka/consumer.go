package kafka

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/segmentio/kafka-go"

	"github.com/Ramsey-B/ivy/config"
)

// MessageHandler processes incoming Kafka messages
type MessageHandler func(ctx context.Context, msg *IncomingMessage) error

// Consumer handles Kafka message consumption
type Consumer struct {
	reader  *kafka.Reader
	logger  ectologger.Logger
	handler MessageHandler
	wg      sync.WaitGroup
	cancel  context.CancelFunc
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(cfg config.Config, logger ectologger.Logger, handler MessageHandler) *Consumer {
	return NewConsumerWithConfig(ConsumerConfig{
		Brokers:       cfg.KafkaBrokers,
		Topic:         cfg.KafkaInputTopic,
		ConsumerGroup: cfg.KafkaConsumerGroup,
	}, logger, handler)
}

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers       []string
	Topic         string
	ConsumerGroup string
}

// NewConsumerWithConfig creates a new Kafka consumer with explicit config
func NewConsumerWithConfig(cfg ConsumerConfig, logger ectologger.Logger, handler MessageHandler) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.ConsumerGroup,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		MaxWait:        500 * time.Millisecond,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: time.Second,
	})

	return &Consumer{
		reader:  reader,
		logger:  logger,
		handler: handler,
	}
}

// Start begins consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go c.consumeLoop(ctx)

	c.logger.WithContext(ctx).WithFields(map[string]any{
		"topic": c.reader.Config().Topic,
	}).Info("Kafka consumer started")
	return nil
}

// Stop gracefully stops the consumer
func (c *Consumer) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	return c.reader.Close()
}

func (c *Consumer) consumeLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			c.logger.WithContext(ctx).Info("Consumer loop stopping")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled || err == io.EOF {
					return
				}
				c.logger.WithContext(ctx).WithError(err).Error("Failed to fetch message")
				continue
			}

			c.processMessage(ctx, msg)
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) {
	ctx, span := tracing.StartSpan(ctx, "kafka.Consumer.processMessage")
	defer span.End()

	log := c.logger.WithContext(ctx).WithFields(map[string]any{
		"topic":     msg.Topic,
		"partition": msg.Partition,
		"offset":    msg.Offset,
	})

	// Parse headers
	headers := make(map[string]string)
	for _, h := range msg.Headers {
		headers[h.Key] = string(h.Value)
	}

	// Create incoming message
	incoming := &IncomingMessage{
		Key:       string(msg.Key),
		Value:     msg.Value,
		Headers:   headers,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Timestamp: msg.Time,
		Topic:     msg.Topic,
	}

	// Parse the Lotus message
	if err := incoming.ParseLotusMessage(); err != nil {
		// Allow non-Lotus messages used for pipeline coordination.
		if incoming.IsExecutionCompleted() {
			// ok
		} else if incoming.IsDeleteMessage() {
			if del, delErr := incoming.ParseDeleteMessage(); delErr == nil {
				incoming.DeleteMessage = del
			} else {
				log.WithError(delErr).Error("Failed to parse delete message")
			}
		} else {
			log.WithError(err).Error("Failed to parse message")
			// Still commit to avoid getting stuck
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.WithError(err).Error("Failed to commit message")
			}
			return
		}
	}

	// Process the message
	if err := c.handler(ctx, incoming); err != nil {
		// Do NOT commit on processing failure. This ensures at-least-once processing so
		// downstream materializations (like merged_entities) are not silently skipped.
		log.WithError(err).Error("Failed to process message (not committing)")
		// TODO: Send to DLQ if this is a permanent error, otherwise it will retry indefinitely.
		return
	}

	// Commit the message (success only)
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		log.WithError(err).Error("Failed to commit message")
	}
}

// Health returns the consumer health status
func (c *Consumer) Health() bool {
	return c.reader != nil
}
