package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"

	"github.com/Ramsey-B/orchid/pkg/execution"
	"github.com/Ramsey-B/orchid/pkg/metrics"
	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/redis"
	appctx "github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

var (
	// ErrProcessorStopped is returned when the processor is stopped
	ErrProcessorStopped = errors.New("processor stopped")

	// ErrInvalidJobMessage is returned when a job message is invalid
	ErrInvalidJobMessage = errors.New("invalid job message")
)

const (
	// DefaultBatchSize is the default number of messages to consume at once
	DefaultBatchSize = 10

	// DefaultBlockTimeout is how long to block waiting for messages
	DefaultBlockTimeout = 5 * time.Second

	// DefaultMaxRetries is the default number of retries for a job
	DefaultMaxRetries = 3

	// DefaultClaimInterval is how often to claim stale pending messages
	DefaultClaimInterval = 30 * time.Second

	// DefaultClaimMinIdle is the minimum idle time before claiming a message
	DefaultClaimMinIdle = 60 * time.Second

	// JobTypePlanExecution is the job type for plan execution
	JobTypePlanExecution = "plan_execution"
)

// ProcessorConfig holds configuration for the job processor
type ProcessorConfig struct {
	// Stream name for the job queue
	Stream string

	// Consumer group name
	ConsumerGroup string

	// Consumer name (unique per instance)
	ConsumerName string

	// Number of messages to fetch per batch
	BatchSize int64

	// How long to block waiting for new messages
	BlockTimeout time.Duration

	// Maximum number of retries for a job
	MaxRetries int

	// How often to check for and claim stale pending messages
	ClaimInterval time.Duration

	// Minimum idle time before claiming a pending message
	ClaimMinIdle time.Duration

	// Number of worker goroutines
	WorkerCount int
}

// DefaultProcessorConfig returns the default processor configuration
func DefaultProcessorConfig() ProcessorConfig {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = uuid.New().String()[:8]
	}

	return ProcessorConfig{
		Stream:        "orchid:jobs",
		ConsumerGroup: "orchid-workers",
		ConsumerName:  hostname,
		BatchSize:     DefaultBatchSize,
		BlockTimeout:  DefaultBlockTimeout,
		MaxRetries:    DefaultMaxRetries,
		ClaimInterval: DefaultClaimInterval,
		ClaimMinIdle:  DefaultClaimMinIdle,
		WorkerCount:   1,
	}
}

// PlanExecutionJob represents a job to execute a plan
type PlanExecutionJob struct {
	PlanKey     string `json:"plan_key"`
	Integration string `json:"integration"`
	ConfigID    string `json:"config_id"`
	TenantID    string `json:"tenant_id"`

	// Optional fields
	ContextOverride   map[string]any `json:"context_override,omitempty"`
	ParentExecutionID string         `json:"parent_execution_id,omitempty"`
	ScheduledAt       time.Time      `json:"scheduled_at,omitempty"`
}

// JobResult holds the result of processing a job
type JobResult struct {
	JobID       string
	MessageID   string
	Success     bool
	Error       error
	Duration    time.Duration
	ExecutionID uuid.UUID
}

// Processor processes jobs from a Redis Streams queue
type Processor struct {
	streams      *redis.Streams
	dlq          *redis.DeadLetterQueue
	planExecutor *execution.PlanExecutor
	config       ProcessorConfig
	logger       ectologger.Logger

	// Channels for coordination
	stopCh   chan struct{}
	stoppedC chan struct{}
	jobsCh   chan jobItem

	// State
	running bool
	mu      sync.RWMutex
}

type jobItem struct {
	message redis.StreamMessage
	job     *redis.JobMessage
}

// NewProcessor creates a new job processor
func NewProcessor(
	streams *redis.Streams,
	dlq *redis.DeadLetterQueue,
	planExecutor *execution.PlanExecutor,
	config ProcessorConfig,
	logger ectologger.Logger,
) *Processor {
	// Apply defaults
	if config.BatchSize <= 0 {
		config.BatchSize = DefaultBatchSize
	}
	if config.BlockTimeout <= 0 {
		config.BlockTimeout = DefaultBlockTimeout
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = DefaultMaxRetries
	}
	if config.ClaimInterval <= 0 {
		config.ClaimInterval = DefaultClaimInterval
	}
	if config.ClaimMinIdle <= 0 {
		config.ClaimMinIdle = DefaultClaimMinIdle
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = 1
	}

	return &Processor{
		streams:      streams,
		dlq:          dlq,
		planExecutor: planExecutor,
		config:       config,
		logger:       logger,
		stopCh:       make(chan struct{}),
		stoppedC:     make(chan struct{}),
		jobsCh:       make(chan jobItem, config.BatchSize*2),
	}
}

// Start starts the processor
func (p *Processor) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return errors.New("processor already running")
	}
	p.running = true
	p.mu.Unlock()

	ctx, span := tracing.StartSpan(ctx, "Processor.Start")
	defer span.End()

	p.logger.WithContext(ctx).Infof("Starting job processor: stream=%s group=%s consumer=%s workers=%d",
		p.config.Stream, p.config.ConsumerGroup, p.config.ConsumerName, p.config.WorkerCount)

	// Create consumer group if it doesn't exist
	if err := p.streams.CreateConsumerGroup(ctx, p.config.Stream, p.config.ConsumerGroup); err != nil {
		p.logger.WithContext(ctx).WithError(err).Error("Failed to create consumer group")
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.config.WorkerCount; i++ {
		wg.Add(1)
		go p.worker(ctx, &wg, i)
	}

	// Start consumer loop
	wg.Add(1)
	go p.consumeLoop(ctx, &wg)

	// Start claimer for stale messages
	wg.Add(1)
	go p.claimLoop(ctx, &wg)

	// Wait for stop signal
	go func() {
		<-p.stopCh
		close(p.jobsCh)
		wg.Wait()
		close(p.stoppedC)
	}()

	p.logger.WithContext(ctx).Info("Job processor started")
	return nil
}

// Stop stops the processor gracefully
func (p *Processor) Stop(ctx context.Context) error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	p.running = false
	p.mu.Unlock()

	p.logger.WithContext(ctx).Info("Stopping job processor...")

	close(p.stopCh)

	// Wait for graceful shutdown with timeout
	select {
	case <-p.stoppedC:
		p.logger.WithContext(ctx).Info("Job processor stopped gracefully")
	case <-ctx.Done():
		p.logger.WithContext(ctx).Warn("Job processor shutdown timed out")
		return ctx.Err()
	}

	return nil
}

// IsRunning returns whether the processor is running
func (p *Processor) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// consumeLoop continuously consumes messages from the stream
func (p *Processor) consumeLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	p.logger.WithContext(ctx).Debug("Consumer loop started")

	for {
		select {
		case <-p.stopCh:
			p.logger.WithContext(ctx).Debug("Consumer loop stopping")
			return
		default:
		}

		// Consume messages
		messages, err := p.streams.Consume(
			ctx,
			p.config.Stream,
			p.config.ConsumerGroup,
			p.config.ConsumerName,
			p.config.BatchSize,
			p.config.BlockTimeout,
		)

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			p.logger.WithContext(ctx).WithError(err).Warn("Failed to consume messages")
			time.Sleep(time.Second) // Back off on error
			continue
		}

		// Send messages to workers
		for _, msg := range messages {
			job, err := p.parseJobMessage(msg)
			if err != nil {
				p.logger.WithContext(ctx).WithError(err).Warnf("Failed to parse job message %s", msg.ID)
				// Acknowledge invalid messages to prevent reprocessing
				if ackErr := p.streams.Ack(ctx, p.config.Stream, p.config.ConsumerGroup, msg.ID); ackErr != nil {
					p.logger.WithContext(ctx).WithError(ackErr).Warnf("Failed to ack invalid message %s", msg.ID)
				}
				continue
			}

			select {
			case p.jobsCh <- jobItem{message: msg, job: job}:
			case <-p.stopCh:
				return
			}
		}
	}
}

// claimLoop periodically claims stale pending messages
func (p *Processor) claimLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(p.config.ClaimInterval)
	defer ticker.Stop()

	p.logger.WithContext(ctx).Debug("Claim loop started")

	for {
		select {
		case <-p.stopCh:
			p.logger.WithContext(ctx).Debug("Claim loop stopping")
			return
		case <-ticker.C:
			p.claimPendingMessages(ctx)
		}
	}
}

// claimPendingMessages claims stale pending messages from other consumers
func (p *Processor) claimPendingMessages(ctx context.Context) {
	ctx, span := tracing.StartSpan(ctx, "Processor.claimPendingMessages")
	defer span.End()

	// Get pending messages
	pending, err := p.streams.Pending(ctx, p.config.Stream, p.config.ConsumerGroup, p.config.BatchSize)
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Warn("Failed to get pending messages")
		return
	}

	if len(pending) == 0 {
		return
	}

	// Filter messages that have been idle long enough
	var staleIDs []string
	for _, msg := range pending {
		if msg.Idle >= p.config.ClaimMinIdle {
			// Check retry count
			if msg.RetryCount <= int64(p.config.MaxRetries) {
				staleIDs = append(staleIDs, msg.ID)
			} else {
				p.logger.WithContext(ctx).Warnf("Message %s exceeded max retries (%d), moving to DLQ", msg.ID, msg.RetryCount)
				// Move to dead letter queue
				p.moveToDLQ(ctx, msg.ID, int(msg.RetryCount), models.DLQReasonMaxRetries, "exceeded maximum retry count")
			}
		}
	}

	if len(staleIDs) == 0 {
		return
	}

	p.logger.WithContext(ctx).Infof("Claiming %d stale pending messages", len(staleIDs))

	// Claim the messages
	claimed, err := p.streams.Claim(ctx, p.config.Stream, p.config.ConsumerGroup, p.config.ConsumerName, p.config.ClaimMinIdle, staleIDs...)
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Warn("Failed to claim pending messages")
		return
	}

	// Send claimed messages to workers
	for _, msg := range claimed {
		job, err := p.parseJobMessage(msg)
		if err != nil {
			p.logger.WithContext(ctx).WithError(err).Warnf("Failed to parse claimed job message %s", msg.ID)
			continue
		}

		select {
		case p.jobsCh <- jobItem{message: msg, job: job}:
		case <-p.stopCh:
			return
		default:
			// Channel full, skip for now
		}
	}
}

// worker processes jobs from the channel
func (p *Processor) worker(ctx context.Context, wg *sync.WaitGroup, id int) {
	defer wg.Done()

	p.logger.WithContext(ctx).Debugf("Worker %d started", id)

	for item := range p.jobsCh {
		result := p.processJob(ctx, item)

		if result.Success {
			// Acknowledge successful job
			if err := p.streams.Ack(ctx, p.config.Stream, p.config.ConsumerGroup, item.message.ID); err != nil {
				p.logger.WithContext(ctx).WithError(err).Warnf("Failed to ack message %s", item.message.ID)
			}
		} else {
			// Log failure - message will be reclaimed after ClaimMinIdle
			p.logger.WithContext(ctx).WithError(result.Error).Warnf("Job %s failed, will be retried", result.JobID)
		}
	}

	p.logger.WithContext(ctx).Debugf("Worker %d stopped", id)
}

// processJob processes a single job
func (p *Processor) processJob(ctx context.Context, item jobItem) *JobResult {
	ctx, span := tracing.StartSpan(ctx, "Processor.processJob")
	defer span.End()

	start := time.Now()
	result := &JobResult{
		JobID:     item.job.ID,
		MessageID: item.message.ID,
	}

	// Set tenant context
	ctx = appctx.SetTenantID(ctx, item.job.TenantID)
	ctx = appctx.SetRequestID(ctx, item.job.ID)

	p.logger.WithContext(ctx).Infof("Processing job %s: type=%s tenant=%s", item.job.ID, item.job.Type, item.job.TenantID)

	switch item.job.Type {
	case JobTypePlanExecution:
		err := p.processPlanExecution(ctx, item.job, result)
		if err != nil {
			result.Error = err
			result.Success = false
		} else {
			result.Success = true
		}

	default:
		result.Error = fmt.Errorf("unknown job type: %s", item.job.Type)
		result.Success = false
	}

	result.Duration = time.Since(start)

	if result.Success {
		p.logger.WithContext(ctx).Infof("Job %s completed successfully in %s", item.job.ID, result.Duration)
	} else {
		p.logger.WithContext(ctx).WithError(result.Error).Warnf("Job %s failed after %s", item.job.ID, result.Duration)
	}

	return result
}

// processPlanExecution processes a plan execution job
func (p *Processor) processPlanExecution(ctx context.Context, job *redis.JobMessage, result *JobResult) error {
	ctx, span := tracing.StartSpan(ctx, "Processor.processPlanExecution")
	defer span.End()

	// Parse job payload
	payloadBytes, err := json.Marshal(job.Payload)
	if err != nil {
		return httperror.NewHTTPErrorf(http.StatusBadRequest, "failed to marshal job payload: %v", err)
	}

	var execJob PlanExecutionJob
	if err := json.Unmarshal(payloadBytes, &execJob); err != nil {
		return httperror.NewHTTPErrorf(http.StatusBadRequest, "failed to unmarshal plan execution job: %v", err)
	}

	// Validate required fields
	if execJob.PlanKey == "" || execJob.ConfigID == "" || execJob.TenantID == "" {
		return httperror.NewHTTPErrorf(http.StatusBadRequest, "%v: missing plan_key, config_id, or tenant_id", ErrInvalidJobMessage)
	}

	// Parse UUIDs
	planKey := execJob.PlanKey
	if planKey == "" {
		return httperror.NewHTTPErrorf(http.StatusBadRequest, "missing plan_key")
	}

	configID, err := uuid.Parse(execJob.ConfigID)
	if err != nil {
		return httperror.NewHTTPErrorf(http.StatusBadRequest, "invalid config_id: %v", err)
	}

	tenantID, err := uuid.Parse(execJob.TenantID)
	if err != nil {
		return httperror.NewHTTPErrorf(http.StatusBadRequest, "invalid tenant_id: %v", err)
	}

	// Build execution input
	input := execution.PlanExecutionInput{
		PlanKey:         planKey,
		Integration:     execJob.Integration,
		ConfigID:        configID,
		TenantID:        tenantID,
		ContextOverride: execJob.ContextOverride,
	}

	if execJob.ParentExecutionID != "" {
		parentID, err := uuid.Parse(execJob.ParentExecutionID)
		if err == nil {
			input.ParentExecutionID = &parentID
		}
	}

	// Execute the plan
	output, err := p.planExecutor.Execute(ctx, input)
	if err != nil {
		return err
	}

	result.ExecutionID = output.ExecutionID
	return nil
}

// parseJobMessage parses a stream message into a JobMessage
func (p *Processor) parseJobMessage(msg redis.StreamMessage) (*redis.JobMessage, error) {
	// The payload should already be the job structure
	jobBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message payload: %w", err)
	}

	var job redis.JobMessage
	if err := json.Unmarshal(jobBytes, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job message: %w", err)
	}

	return &job, nil
}

// PublishPlanExecution publishes a plan execution job to the queue
func PublishPlanExecution(ctx context.Context, streams *redis.Streams, stream string, job PlanExecutionJob) (string, error) {
	msg := &redis.JobMessage{
		ID:        uuid.New().String(),
		TenantID:  job.TenantID,
		Type:      JobTypePlanExecution,
		CreatedAt: time.Now(),
		Payload: map[string]interface{}{
			"plan_key":            job.PlanKey,
			"integration":         job.Integration,
			"config_id":           job.ConfigID,
			"tenant_id":           job.TenantID,
			"context_override":    job.ContextOverride,
			"parent_execution_id": job.ParentExecutionID,
			"scheduled_at":        job.ScheduledAt,
		},
	}

	return streams.Publish(ctx, stream, msg)
}

// moveToDLQ moves a failed job to the dead letter queue
func (p *Processor) moveToDLQ(ctx context.Context, messageID string, retryCount int, reason models.DeadLetterReason, errorMsg string) {
	ctx, span := tracing.StartSpan(ctx, "Processor.moveToDLQ")
	defer span.End()

	// Get the original message to store in DLQ
	messages, err := p.streams.Range(ctx, p.config.Stream, messageID, messageID)
	if err != nil || len(messages) == 0 {
		p.logger.WithContext(ctx).WithError(err).Warnf("Failed to get message %s for DLQ", messageID)
		// Still ack the message to prevent infinite retries
		if ackErr := p.streams.Ack(ctx, p.config.Stream, p.config.ConsumerGroup, messageID); ackErr != nil {
			p.logger.WithContext(ctx).WithError(ackErr).Warnf("Failed to ack failed message %s", messageID)
		}
		return
	}

	msg := messages[0]
	job, err := p.parseJobMessage(msg)
	if err != nil {
		p.logger.WithContext(ctx).WithError(err).Warnf("Failed to parse message %s for DLQ", messageID)
		if ackErr := p.streams.Ack(ctx, p.config.Stream, p.config.ConsumerGroup, messageID); ackErr != nil {
			p.logger.WithContext(ctx).WithError(ackErr).Warnf("Failed to ack failed message %s", messageID)
		}
		return
	}

	// Extract plan and config IDs from the job
	planKey := ""
	configID := ""
	if payload := job.Payload; payload != nil {
		if pid, ok := payload["plan_key"].(string); ok {
			planKey = pid
		}
		if cid, ok := payload["config_id"].(string); ok {
			configID = cid
		}
	}

	// Add to DLQ if available
	if p.dlq != nil {
		entry := &redis.DLQEntry{
			TenantID:     job.TenantID,
			PlanKey:      planKey,
			ConfigID:     configID,
			OriginalJob:  job,
			Reason:       reason,
			ErrorMessage: errorMsg,
			RetryCount:   retryCount,
		}

		if _, dlqErr := p.dlq.Add(ctx, entry); dlqErr != nil {
			p.logger.WithContext(ctx).WithError(dlqErr).Errorf("Failed to add job %s to DLQ", job.ID)
		} else {
			// Record metric
			metrics.RecordDLQJob(job.TenantID, string(reason))
		}
	}

	// Ack the original message
	if ackErr := p.streams.Ack(ctx, p.config.Stream, p.config.ConsumerGroup, messageID); ackErr != nil {
		p.logger.WithContext(ctx).WithError(ackErr).Warnf("Failed to ack message %s after DLQ", messageID)
	}
}
