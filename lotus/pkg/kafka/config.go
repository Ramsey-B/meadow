package kafka

import (
	"time"
)

// ConsumerConfig configures the Kafka consumer
type ConsumerConfig struct {
	// Brokers is a list of Kafka broker addresses
	Brokers []string

	// Topic is the Kafka topic to consume from
	Topic string

	// GroupID is the consumer group ID
	GroupID string

	// MinBytes is the minimum batch size for fetching messages
	MinBytes int

	// MaxBytes is the maximum batch size for fetching messages
	MaxBytes int

	// MaxWait is the maximum time to wait for messages
	MaxWait time.Duration

	// CommitInterval is how often to commit offsets
	CommitInterval time.Duration

	// StartOffset determines where to start reading when there's no committed offset
	// Use FirstOffset (-2) to start from the beginning, or LastOffset (-1) to start from the end
	StartOffset int64

	// SessionTimeout is the session timeout for the consumer group
	SessionTimeout time.Duration

	// HeartbeatInterval is how often to send heartbeats to the broker
	HeartbeatInterval time.Duration

	// RebalanceTimeout is the timeout for rebalancing
	RebalanceTimeout time.Duration
}

// DefaultConsumerConfig returns a ConsumerConfig with sensible defaults
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		Brokers:           []string{"localhost:9092"},
		Topic:             "api-responses",
		GroupID:           "lotus-consumer",
		MinBytes:          1,
		MaxBytes:          10e6, // 10MB
		MaxWait:           3 * time.Second,
		CommitInterval:    time.Second,
		StartOffset:       LastOffset,
		SessionTimeout:    30 * time.Second,
		HeartbeatInterval: 3 * time.Second,
		RebalanceTimeout:  30 * time.Second,
	}
}

// ProducerConfig configures the Kafka producer
type ProducerConfig struct {
	// Brokers is a list of Kafka broker addresses
	Brokers []string

	// Topic is the default output topic (can be overridden per message)
	Topic string

	// BatchSize is the number of messages to batch before sending
	BatchSize int

	// BatchTimeout is the maximum time to wait before sending a batch
	BatchTimeout time.Duration

	// RequiredAcks specifies the number of acks required
	// 0 = no acks, 1 = leader only, -1 = all replicas
	RequiredAcks int

	// Async enables asynchronous writes
	Async bool

	// MaxAttempts is the maximum number of retries
	MaxAttempts int

	// WriteTimeout is the timeout for write operations
	WriteTimeout time.Duration

	// Compression is the compression algorithm to use
	// Options: none, gzip, snappy, lz4, zstd
	Compression string
}

// DefaultProducerConfig returns a ProducerConfig with sensible defaults
func DefaultProducerConfig() ProducerConfig {
	return ProducerConfig{
		Brokers:      []string{"localhost:9092"},
		Topic:        "mapped-data",
		BatchSize:    100,
		BatchTimeout: 100 * time.Millisecond,
		RequiredAcks: 1,
		Async:        false,
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		Compression:  "snappy",
	}
}

// Offset constants
const (
	FirstOffset int64 = -2 // Start from the oldest message
	LastOffset  int64 = -1 // Start from the newest message
)

