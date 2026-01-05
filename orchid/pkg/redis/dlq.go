package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

const (
	// DefaultDLQStream is the default dead letter queue stream name
	DefaultDLQStream = "orchid:dlq"

	// DLQMaxLen is the maximum length of the DLQ stream (oldest entries trimmed)
	DLQMaxLen = 10000
)

// DeadLetterQueue handles dead letter queue operations
type DeadLetterQueue struct {
	client     *Client
	streamName string
	logger     ectologger.Logger
}

// NewDeadLetterQueue creates a new dead letter queue handler
func NewDeadLetterQueue(client *Client, streamName string, logger ectologger.Logger) *DeadLetterQueue {
	if streamName == "" {
		streamName = DefaultDLQStream
	}
	return &DeadLetterQueue{
		client:     client,
		streamName: streamName,
		logger:     logger,
	}
}

// DLQEntry represents a dead letter queue entry
type DLQEntry struct {
	ID           string                  `json:"id"`
	TenantID     string                  `json:"tenant_id"`
	PlanKey      string                  `json:"plan_key"`
	ConfigID     string                  `json:"config_id"`
	ExecutionID  string                  `json:"execution_id,omitempty"`
	OriginalJob  *JobMessage             `json:"original_job"` // The original job message
	Reason       models.DeadLetterReason `json:"reason"`
	ErrorMessage string                  `json:"error_message"`
	RetryCount   int                     `json:"retry_count"`
	CreatedAt    time.Time               `json:"created_at"`
	TraceID      string                  `json:"trace_id,omitempty"`
}

// Add adds a job to the dead letter queue
func (d *DeadLetterQueue) Add(ctx context.Context, entry *DLQEntry) (string, error) {
	ctx, span := tracing.StartSpan(ctx, "DLQ.Add")
	defer span.End()

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.TraceID = tracing.GetTraceID(ctx)

	// Serialize the entry
	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("failed to marshal DLQ entry: %w", err)
	}

	// Add to stream with max length trimming
	messageID, err := d.client.Redis().XAdd(ctx, &redis.XAddArgs{
		Stream: d.streamName,
		MaxLen: DLQMaxLen,
		Approx: true,
		Values: map[string]interface{}{
			"data":      string(data),
			"tenant_id": entry.TenantID,
			"plan_key":  entry.PlanKey,
			"reason":    string(entry.Reason),
		},
	}).Result()

	if err != nil {
		d.logger.WithContext(ctx).WithError(err).Error("Failed to add job to DLQ")
		return "", fmt.Errorf("failed to add to DLQ: %w", err)
	}

	d.logger.WithContext(ctx).Infof("Added job to DLQ: id=%s plan=%s reason=%s", entry.ID, entry.PlanKey, entry.Reason)
	return messageID, nil
}

// List returns entries from the dead letter queue
func (d *DeadLetterQueue) List(ctx context.Context, count int64) ([]DLQEntry, error) {
	ctx, span := tracing.StartSpan(ctx, "DLQ.List")
	defer span.End()

	if count <= 0 {
		count = 100
	}

	messages, err := d.client.Redis().XRevRangeN(ctx, d.streamName, "+", "-", count).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read DLQ: %w", err)
	}

	entries := make([]DLQEntry, 0, len(messages))
	for _, msg := range messages {
		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var entry DLQEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			d.logger.WithContext(ctx).WithError(err).Warnf("Failed to unmarshal DLQ entry: %s", msg.ID)
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// ListByTenant returns entries for a specific tenant
func (d *DeadLetterQueue) ListByTenant(ctx context.Context, tenantID string, count int64) ([]DLQEntry, error) {
	entries, err := d.List(ctx, count*2) // Fetch more to filter
	if err != nil {
		return nil, err
	}

	filtered := make([]DLQEntry, 0)
	for _, entry := range entries {
		if entry.TenantID == tenantID {
			filtered = append(filtered, entry)
			if int64(len(filtered)) >= count {
				break
			}
		}
	}

	return filtered, nil
}

// Get retrieves a specific DLQ entry by message ID
func (d *DeadLetterQueue) Get(ctx context.Context, messageID string) (*DLQEntry, error) {
	ctx, span := tracing.StartSpan(ctx, "DLQ.Get")
	defer span.End()

	messages, err := d.client.Redis().XRange(ctx, d.streamName, messageID, messageID).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get DLQ entry: %w", err)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	data, ok := messages[0].Values["data"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid DLQ entry format")
	}

	var entry DLQEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DLQ entry: %w", err)
	}

	return &entry, nil
}

// Delete removes an entry from the dead letter queue
func (d *DeadLetterQueue) Delete(ctx context.Context, messageID string) error {
	ctx, span := tracing.StartSpan(ctx, "DLQ.Delete")
	defer span.End()

	count, err := d.client.Redis().XDel(ctx, d.streamName, messageID).Result()
	if err != nil {
		return fmt.Errorf("failed to delete DLQ entry: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("DLQ entry not found: %s", messageID)
	}

	d.logger.WithContext(ctx).Infof("Deleted DLQ entry: %s", messageID)
	return nil
}

// Count returns the number of entries in the DLQ
func (d *DeadLetterQueue) Count(ctx context.Context) (int64, error) {
	return d.client.Redis().XLen(ctx, d.streamName).Result()
}

// Retry re-enqueues a DLQ entry back to the main job queue
func (d *DeadLetterQueue) Retry(ctx context.Context, messageID string, jobQueue *Streams, queueName string) error {
	ctx, span := tracing.StartSpan(ctx, "DLQ.Retry")
	defer span.End()

	entry, err := d.Get(ctx, messageID)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("DLQ entry not found: %s", messageID)
	}

	if entry.OriginalJob == nil {
		return fmt.Errorf("DLQ entry has no original job: %s", messageID)
	}

	// Reset attempts for retry
	entry.OriginalJob.Attempts = 0

	// Re-enqueue the original job
	_, err = jobQueue.Publish(ctx, queueName, entry.OriginalJob)
	if err != nil {
		return fmt.Errorf("failed to re-enqueue job: %w", err)
	}

	// Delete from DLQ
	if err := d.Delete(ctx, messageID); err != nil {
		d.logger.WithContext(ctx).WithError(err).Warn("Failed to delete DLQ entry after retry")
	}

	d.logger.WithContext(ctx).Infof("Retried DLQ entry: %s plan=%s", messageID, entry.PlanKey)
	return nil
}
