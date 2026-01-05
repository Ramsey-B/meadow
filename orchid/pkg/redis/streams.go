package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// StreamMessage represents a message from a Redis Stream
type StreamMessage struct {
	ID      string
	Stream  string
	Payload map[string]interface{}
}

// JobMessage represents a job in the queue
type JobMessage struct {
	ID        string                 `json:"id"`
	TenantID  string                 `json:"tenant_id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
	Attempts  int                    `json:"attempts"`
}

// Streams provides Redis Streams operations for job queues
type Streams struct {
	client *Client
}

// NewStreams creates a new Streams instance
func NewStreams(client *Client) *Streams {
	return &Streams{client: client}
}

// Publish adds a message to a stream
func (s *Streams) Publish(ctx context.Context, stream string, job *JobMessage) (string, error) {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("failed to marshal job: %w", err)
	}

	result, err := s.client.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{
			"data": string(payload),
		},
	}).Result()

	if err != nil {
		s.client.logger.WithContext(ctx).WithError(err).Errorf("Failed to publish to stream %s", stream)
		return "", err
	}

	s.client.logger.WithContext(ctx).Infof("Published job %s to stream %s (message ID: %s)", job.ID, stream, result)
	return result, nil
}

// CreateConsumerGroup creates a consumer group for a stream
func (s *Streams) CreateConsumerGroup(ctx context.Context, stream, group string) error {
	err := s.client.rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// Consume reads messages from a stream using a consumer group
func (s *Streams) Consume(ctx context.Context, stream, group, consumer string, count int64, block time.Duration) ([]StreamMessage, error) {
	results, err := s.client.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()

	if err == redis.Nil {
		return nil, nil // No messages
	}
	if err != nil {
		return nil, err
	}

	var messages []StreamMessage
	for _, result := range results {
		for _, msg := range result.Messages {
			data, ok := msg.Values["data"].(string)
			if !ok {
				continue
			}

			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				s.client.logger.WithContext(ctx).WithError(err).Warnf("Failed to unmarshal message %s", msg.ID)
				continue
			}

			messages = append(messages, StreamMessage{
				ID:      msg.ID,
				Stream:  result.Stream,
				Payload: payload,
			})
		}
	}

	return messages, nil
}

// Ack acknowledges a message
func (s *Streams) Ack(ctx context.Context, stream, group string, ids ...string) error {
	return s.client.rdb.XAck(ctx, stream, group, ids...).Err()
}

// Pending returns pending messages that need to be processed
func (s *Streams) Pending(ctx context.Context, stream, group string, count int64) ([]redis.XPendingExt, error) {
	return s.client.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  "-",
		End:    "+",
		Count:  count,
	}).Result()
}

// Claim claims pending messages for a consumer
func (s *Streams) Claim(ctx context.Context, stream, group, consumer string, minIdle time.Duration, ids ...string) ([]StreamMessage, error) {
	results, err := s.client.rdb.XClaim(ctx, &redis.XClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Messages: ids,
	}).Result()

	if err != nil {
		return nil, err
	}

	var messages []StreamMessage
	for _, msg := range results {
		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}

		messages = append(messages, StreamMessage{
			ID:      msg.ID,
			Stream:  stream,
			Payload: payload,
		})
	}

	return messages, nil
}

// Len returns the length of a stream
func (s *Streams) Len(ctx context.Context, stream string) (int64, error) {
	return s.client.rdb.XLen(ctx, stream).Result()
}

// Trim trims a stream to approximately maxLen entries
func (s *Streams) Trim(ctx context.Context, stream string, maxLen int64) error {
	return s.client.rdb.XTrimMaxLenApprox(ctx, stream, maxLen, 0).Err()
}

// Range returns messages in a stream between start and end IDs
func (s *Streams) Range(ctx context.Context, stream, start, end string) ([]StreamMessage, error) {
	results, err := s.client.rdb.XRange(ctx, stream, start, end).Result()
	if err != nil {
		return nil, err
	}

	var messages []StreamMessage
	for _, msg := range results {
		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}

		messages = append(messages, StreamMessage{
			ID:      msg.ID,
			Stream:  stream,
			Payload: payload,
		})
	}

	return messages, nil
}

