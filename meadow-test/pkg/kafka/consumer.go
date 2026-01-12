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

// BodyFilter specifies a field path and substring that the message body must contain
type BodyFilter struct {
	FieldPath      string // e.g., "payload.after.entity_type"
	ContainsValue  string // substring that must be present in the field value
}

// FindMessage looks for a message matching the given header filters and optional field requirement
// Returns nil if not found
func (bc *BackgroundConsumer) FindMessage(headerFilters map[string]string, requiredField string) *Message {
	return bc.FindMessageWithBodyFilter(headerFilters, requiredField, nil)
}

// FindMessageWithBodyFilter looks for a message matching header filters AND body field filters
func (bc *BackgroundConsumer) FindMessageWithBodyFilter(headerFilters map[string]string, requiredField string, bodyFilter *BodyFilter) *Message {
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

		// Check body filter (if specified)
		if bodyFilter != nil && bodyFilter.FieldPath != "" && bodyFilter.ContainsValue != "" {
			fieldValue := getNestedFieldFromBody(msg.Body, bodyFilter.FieldPath)
			fieldStr := fmt.Sprintf("%v", fieldValue)
			if fieldValue == nil || !containsString(fieldStr, bodyFilter.ContainsValue) {
				continue
			}
		}

		return msg
	}

	return nil
}

// getNestedFieldFromBody extracts a nested field from the message body using dot notation
func getNestedFieldFromBody(body map[string]interface{}, path string) interface{} {
	parts := splitPath(path)
	var current interface{} = body

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

// splitPath splits a dot-notation path into parts
func splitPath(path string) []string {
	var parts []string
	var current string
	for _, c := range path {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// containsString checks if s contains substr (case-sensitive)
func containsString(s, substr string) bool {
	return len(substr) <= len(s) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// WaitForMessage waits for a message matching the filters, with timeout and retry
func (bc *BackgroundConsumer) WaitForMessage(headerFilters map[string]string, requiredField string, timeout time.Duration) (*Message, error) {
	return bc.WaitForMessageWithBodyFilter(headerFilters, requiredField, nil, timeout)
}

// WaitForMessageWithBodyFilter waits for a message matching header AND body filters
func (bc *BackgroundConsumer) WaitForMessageWithBodyFilter(headerFilters map[string]string, requiredField string, bodyFilter *BodyFilter, timeout time.Duration) (*Message, error) {
	deadline := time.Now().Add(timeout)
	attempts := 0

	for time.Now().Before(deadline) {
		attempts++

		if msg := bc.FindMessageWithBodyFilter(headerFilters, requiredField, bodyFilter); msg != nil {
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
