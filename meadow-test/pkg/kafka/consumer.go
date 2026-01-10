package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

// Message represents a stored Kafka message with its headers and parsed body
type Message struct {
	Offset  int64
	Headers map[string]string
	Body    map[string]interface{}
	Raw     []byte
}

// BackgroundConsumer continuously consumes Kafka messages in the background
type BackgroundConsumer struct {
	reader   *kafka.Reader
	messages []Message
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewBackgroundConsumer creates and starts a background Kafka consumer
func NewBackgroundConsumer(brokers []string, topic string, startOffset int64) (*BackgroundConsumer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   brokers,
		Topic:     topic,
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
	})

	// Set starting offset
	if err := reader.SetOffset(startOffset); err != nil {
		reader.Close()
		cancel()
		return nil, fmt.Errorf("failed to set offset: %w", err)
	}

	bc := &BackgroundConsumer{
		reader:   reader,
		messages: make([]Message, 0),
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
	}

	// Start consuming in background
	go bc.consume()

	return bc, nil
}

// consume runs in a goroutine and continuously reads messages
func (bc *BackgroundConsumer) consume() {
	defer close(bc.done)

	for {
		// Check if context is cancelled
		select {
		case <-bc.ctx.Done():
			return
		default:
		}

		// Read message with a short timeout to allow checking cancellation
		readCtx, cancel := context.WithTimeout(bc.ctx, 1*time.Second)
		msg, err := bc.reader.ReadMessage(readCtx)
		cancel()

		if err != nil {
			// If context cancelled, exit
			if bc.ctx.Err() != nil {
				return
			}
			// Timeout is expected, just continue
			continue
		}

		// Parse message
		headers := make(map[string]string)
		for _, h := range msg.Headers {
			headers[h.Key] = string(h.Value)
		}

		var body map[string]interface{}
		if err := json.Unmarshal(msg.Value, &body); err != nil {
			// If JSON parse fails, store raw value
			body = map[string]interface{}{
				"_raw":         string(msg.Value),
				"_parse_error": err.Error(),
			}
		}

		// Store message
		bc.mu.Lock()
		bc.messages = append(bc.messages, Message{
			Offset:  msg.Offset,
			Headers: headers,
			Body:    body,
			Raw:     msg.Value,
		})
		bc.mu.Unlock()
	}
}

// FindMessage looks for a message matching the given header filters and optional field requirement
// Returns nil if not found
func (bc *BackgroundConsumer) FindMessage(headerFilters map[string]string, requiredField string) *Message {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for i := range bc.messages {
		msg := &bc.messages[i]

		// Check if all headers match
		allMatch := true
		for key, expectedValue := range headerFilters {
			actualValue, found := msg.Headers[key]
			if !found || actualValue != expectedValue {
				allMatch = false
				break
			}
		}

		if !allMatch {
			continue
		}

		// Check if required field exists (if specified)
		if requiredField != "" {
			if _, found := msg.Body[requiredField]; !found {
				continue
			}
		}

		return msg
	}

	return nil
}

// WaitForMessage waits for a message matching the filters, with timeout and retry
func (bc *BackgroundConsumer) WaitForMessage(headerFilters map[string]string, requiredField string, timeout time.Duration) (*Message, error) {
	deadline := time.Now().Add(timeout)
	attempts := 0

	for time.Now().Before(deadline) {
		attempts++

		if msg := bc.FindMessage(headerFilters, requiredField); msg != nil {
			return msg, nil
		}

		// Wait a bit before retrying
		time.Sleep(100 * time.Millisecond)
	}

	// Get message count for better error message
	bc.mu.RLock()
	msgCount := len(bc.messages)
	bc.mu.RUnlock()

	return nil, fmt.Errorf("no message found matching filters after %d attempts (scanned %d messages, timeout %s)", attempts, msgCount, timeout)
}

// MessageCount returns the number of messages consumed so far
func (bc *BackgroundConsumer) MessageCount() int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return len(bc.messages)
}

// Close stops the background consumer and releases resources
func (bc *BackgroundConsumer) Close() error {
	bc.cancel()

	// Wait for consumer goroutine to exit (with timeout)
	select {
	case <-bc.done:
	case <-time.After(5 * time.Second):
		// Force close after timeout
	}

	return bc.reader.Close()
}
