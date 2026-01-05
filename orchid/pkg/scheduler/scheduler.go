package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"

	"github.com/Ramsey-B/orchid/pkg/queue"
	"github.com/Ramsey-B/orchid/pkg/redis"
	appctx "github.com/Ramsey-B/stem/pkg/context"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

var (
	// ErrSchedulerStopped is returned when the scheduler is stopped
	ErrSchedulerStopped = errors.New("scheduler stopped")

	// ErrSchedulerAlreadyRunning is returned when trying to start an already running scheduler
	ErrSchedulerAlreadyRunning = errors.New("scheduler already running")
)

const (
	// DefaultPollInterval is the default interval between scheduling runs
	DefaultPollInterval = 30 * time.Second

	// DefaultLockTTL is the default TTL for distributed locks
	DefaultLockTTL = 60 * time.Second

	// DefaultBatchSize is the number of plan/config pairs to fetch per poll
	DefaultBatchSize = 100

	// LockKeyPrefix is the prefix for scheduler locks
	LockKeyPrefix = "scheduler:plan:"
)

// SchedulablePlan represents a plan+config combination that can be scheduled
type SchedulablePlan struct {
	TenantID        uuid.UUID
	Integration     string
	PlanKey         string
	ConfigID        uuid.UUID
	IntegrationID   uuid.UUID
	WaitSeconds     int
	LastExecutionAt *time.Time
}

// SchedulerRepository defines the interface for scheduler data access
// This is separate from tenant-scoped repositories as it needs cross-tenant access
type SchedulerRepository interface {
	// ListSchedulablePlans returns all enabled plan+config combinations that are due for execution
	ListSchedulablePlans(ctx context.Context, limit int) ([]SchedulablePlan, error)
}

// Config holds configuration for the scheduler
type Config struct {
	// PollInterval is how often to check for schedulable plans
	PollInterval time.Duration

	// LockTTL is how long to hold a lock for a plan+config
	LockTTL time.Duration

	// BatchSize is the maximum number of plans to schedule per poll
	BatchSize int

	// JobQueue is the Redis Streams queue name
	JobQueue string
}

// DefaultConfig returns the default scheduler configuration
func DefaultConfig() Config {
	return Config{
		PollInterval: DefaultPollInterval,
		LockTTL:      DefaultLockTTL,
		BatchSize:    DefaultBatchSize,
		JobQueue:     "orchid:jobs",
	}
}

// Scheduler polls for and schedules plan executions
type Scheduler struct {
	repo    SchedulerRepository
	streams *redis.Streams
	locker  *redis.Locker
	config  Config
	logger  ectologger.Logger

	// Coordination
	stopCh   chan struct{}
	stoppedC chan struct{}
	running  bool
	mu       sync.RWMutex
}

// NewScheduler creates a new scheduler
func NewScheduler(
	repo SchedulerRepository,
	streams *redis.Streams,
	locker *redis.Locker,
	config Config,
	logger ectologger.Logger,
) *Scheduler {
	// Apply defaults
	if config.PollInterval <= 0 {
		config.PollInterval = DefaultPollInterval
	}
	if config.LockTTL <= 0 {
		config.LockTTL = DefaultLockTTL
	}
	if config.BatchSize <= 0 {
		config.BatchSize = DefaultBatchSize
	}
	if config.JobQueue == "" {
		config.JobQueue = "orchid:jobs"
	}

	return &Scheduler{
		repo:     repo,
		streams:  streams,
		locker:   locker,
		config:   config,
		logger:   logger,
		stopCh:   make(chan struct{}),
		stoppedC: make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrSchedulerAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	ctx, span := tracing.StartSpan(ctx, "Scheduler.Start")
	defer span.End()

	s.logger.WithContext(ctx).Infof("Starting scheduler: poll_interval=%s batch_size=%d",
		s.config.PollInterval, s.config.BatchSize)

	// Start the polling loop
	go s.pollLoop(ctx)

	s.logger.WithContext(ctx).Info("Scheduler started")
	return nil
}

// Stop stops the scheduler gracefully
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	s.logger.WithContext(ctx).Info("Stopping scheduler...")

	close(s.stopCh)

	// Wait for graceful shutdown with timeout
	select {
	case <-s.stoppedC:
		s.logger.WithContext(ctx).Info("Scheduler stopped gracefully")
	case <-ctx.Done():
		s.logger.WithContext(ctx).Warn("Scheduler shutdown timed out")
		return ctx.Err()
	}

	return nil
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// pollLoop continuously polls for schedulable plans
func (s *Scheduler) pollLoop(ctx context.Context) {
	defer close(s.stoppedC)

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	// Run immediately on start
	s.runSchedulingCycle(ctx)

	for {
		select {
		case <-s.stopCh:
			s.logger.WithContext(ctx).Debug("Scheduler poll loop stopping")
			return
		case <-ticker.C:
			s.runSchedulingCycle(ctx)
		}
	}
}

// runSchedulingCycle runs a single scheduling cycle
func (s *Scheduler) runSchedulingCycle(ctx context.Context) {
	ctx, span := tracing.StartSpan(ctx, "Scheduler.runSchedulingCycle")
	defer span.End()

	start := time.Now()
	s.logger.WithContext(ctx).Debug("Running scheduling cycle")

	// Fetch schedulable plans
	plans, err := s.repo.ListSchedulablePlans(ctx, s.config.BatchSize)
	if err != nil {
		s.logger.WithContext(ctx).WithError(err).Error("Failed to list schedulable plans")
		return
	}

	if len(plans) == 0 {
		s.logger.WithContext(ctx).Debug("No plans to schedule")
		return
	}

	s.logger.WithContext(ctx).Infof("Found %d plans to schedule", len(plans))

	// Schedule each plan
	scheduled := 0
	skipped := 0
	for _, plan := range plans {
		if err := s.schedulePlan(ctx, plan); err != nil {
			if errors.Is(err, redis.ErrLockNotAcquired) {
				skipped++
				continue
			}
			s.logger.WithContext(ctx).WithError(err).Warnf("Failed to schedule plan %s with config %s",
				plan.PlanKey, plan.ConfigID)
			continue
		}
		scheduled++
	}

	duration := time.Since(start)
	s.logger.WithContext(ctx).Infof("Scheduling cycle completed: scheduled=%d skipped=%d duration=%s",
		scheduled, skipped, duration)
}

// schedulePlan schedules a single plan+config combination
func (s *Scheduler) schedulePlan(ctx context.Context, plan SchedulablePlan) error {
	ctx, span := tracing.StartSpan(ctx, "Scheduler.schedulePlan")
	defer span.End()

	// Create a lock key for this plan+config combination
	lockKey := s.lockKey(plan.PlanKey, plan.ConfigID)

	// Try to acquire the lock
	lock, err := s.locker.Acquire(ctx, lockKey, s.config.LockTTL)
	if err != nil {
		return err
	}
	// Release the lock after publishing (the processor will handle its own locking if needed)
	defer lock.Release(ctx)

	// Set tenant context for logging
	ctx = appctx.SetTenantID(ctx, plan.TenantID.String())

	s.logger.WithContext(ctx).Debugf("Scheduling plan %s with config %s", plan.PlanKey, plan.ConfigID)

	// Create the job
	job := queue.PlanExecutionJob{
		TenantID:    plan.TenantID.String(),
		Integration: plan.Integration,
		PlanKey:     plan.PlanKey,
		ConfigID:    plan.ConfigID.String(),
		ScheduledAt: time.Now(),
	}

	// Publish to the queue
	messageID, err := queue.PublishPlanExecution(ctx, s.streams, s.config.JobQueue, job)
	if err != nil {
		s.logger.WithContext(ctx).WithError(err).Errorf("Failed to publish job for plan %s config %s",
			plan.PlanKey, plan.ConfigID)
		return err
	}

	s.logger.WithContext(ctx).Infof("Scheduled plan %s with config %s (message_id=%s)",
		plan.PlanKey, plan.ConfigID, messageID)

	return nil
}

// lockKey generates a lock key for a plan+config combination
func (s *Scheduler) lockKey(planKey string, configID uuid.UUID) string {
	return LockKeyPrefix + planKey + ":" + configID.String()
}
