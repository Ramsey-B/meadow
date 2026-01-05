package execution

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"

	"github.com/Ramsey-B/orchid/pkg/expressions"
	"github.com/Ramsey-B/orchid/pkg/httpclient"
	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/ratelimit"
)

// StepResult holds the result of a step execution
type StepResult struct {
	Response      *httpclient.Response
	Context       *ExecutionContext
	RequestURL    string
	RequestMethod string
	ShouldAbort   bool
	ShouldBreak   bool
	ShouldRetry   bool
	ShouldIgnore  bool
	Error         error
	RetryCount    int
	ExecutionTime time.Duration
	RateLimited   bool          // True if request was rate limited
	WaitedFor     time.Duration // Time spent waiting for rate limit
}

// ExecuteOptions provides optional configuration for step execution
type ExecuteOptions struct {
	TenantID      uuid.UUID
	IntegrationID uuid.UUID
	ConfigID      uuid.UUID
	RateLimits    []models.RateLimitConfig
	MaxRateWait   time.Duration // Max time to wait for rate limit (default: 60s)
}

// StepExecutor executes individual steps
type StepExecutor struct {
	client          *httpclient.Client
	requestBuilder  *httpclient.RequestBuilder
	evaluator       *expressions.Evaluator
	rateLimiter     *ratelimit.Manager
	logger          ectologger.Logger
}

type releaseFunc = func()

// NewStepExecutor creates a new step executor
func NewStepExecutor(
	client *httpclient.Client,
	evaluator *expressions.Evaluator,
	rateLimiter *ratelimit.Manager,
	logger ectologger.Logger,
) *StepExecutor {
	return &StepExecutor{
		client:         client,
		requestBuilder: httpclient.NewRequestBuilder(evaluator),
		evaluator:      evaluator,
		rateLimiter:    rateLimiter,
		logger:         logger,
	}
}

// Execute executes a single step
func (e *StepExecutor) Execute(ctx context.Context, step *models.Step, execCtx *ExecutionContext) (*StepResult, error) {
	return e.ExecuteWithOptions(ctx, step, execCtx, nil)
}

// ExecuteWithOptions executes a step with additional options (rate limiting, etc.)
func (e *StepExecutor) ExecuteWithOptions(ctx context.Context, step *models.Step, execCtx *ExecutionContext, opts *ExecuteOptions) (*StepResult, error) {
	// Apply defaults
	step = e.applyDefaults(step)

	// Set timeout
	if step.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	maxRetries := 0
	if step.Retry != nil && step.Retry.MaxRetries > 0 {
		maxRetries = step.Retry.MaxRetries
	}

	var lastResult *StepResult
	for attempt := 0; attempt <= maxRetries; attempt++ {
		start := time.Now()
		result := &StepResult{Context: execCtx, RetryCount: attempt}

		// Build the HTTP request first (need URL for rate limiting)
		data := execCtx.ToMap()
		req, err := e.requestBuilder.BuildRequest(ctx, step, data)
		if err != nil {
			result.Error = fmt.Errorf("failed to build request: %w", err)
			return result, result.Error
		}
		result.RequestURL = req.URL.String()
		result.RequestMethod = req.Method

		// Check and wait for rate limit (also acquires any concurrency slots)
		var release releaseFunc
		if opts != nil && len(opts.RateLimits) > 0 && e.rateLimiter != nil {
			waitStart := time.Now()
			checkReq := ratelimit.CheckRequest{
				TenantID:      opts.TenantID,
				IntegrationID: opts.IntegrationID,
				ConfigID:      opts.ConfigID,
				URL:           req.URL.String(),
				RateLimits:    opts.RateLimits,
			}
			rel, err := e.rateLimiter.WaitForLimit(ctx, checkReq, func() time.Duration {
				if opts.MaxRateWait == 0 {
					return 60 * time.Second
				}
				return opts.MaxRateWait
			}())
			release = rel
			if err != nil {
				result.Error = fmt.Errorf("rate limit wait failed: %w", err)
				result.RateLimited = true
				return result, result.Error
			}
			result.WaitedFor = time.Since(waitStart)
			if result.WaitedFor > time.Millisecond*100 {
				e.logger.WithContext(ctx).Debugf("Waited %v for rate limit", result.WaitedFor)
			}
		}

		e.logger.WithContext(ctx).Debugf("Executing step: %s %s", req.Method, req.URL.String())

		resp, err := e.client.Do(ctx, req)
		if release != nil {
			// Release concurrency slot as soon as the request returns.
			release()
		}
		if err != nil {
			result.Error = fmt.Errorf("request failed: %w", err)
			// Network errors: retry if retries are configured
			if attempt < maxRetries {
				delay := CalculateBackoff(step.Retry, attempt+1)
				e.logger.WithContext(ctx).Warnf("Request error, retrying in %v (attempt %d/%d): %v", delay, attempt+1, maxRetries, err)
				time.Sleep(delay)
				lastResult = result
				continue
			}
			return result, result.Error
		}

		// Parse response
		if err := httpclient.ParseResponse(resp); err != nil {
			e.logger.WithContext(ctx).WithError(err).Warn("Failed to parse response body")
		}

		result.Response = resp
		result.ExecutionTime = time.Since(start)

		// Update dynamic rate limits from response headers
		if opts != nil && len(opts.RateLimits) > 0 && e.rateLimiter != nil {
			e.updateRateLimitsFromResponse(ctx, req.URL.String(), opts, resp)
		}

		// Mark 429 as retryable; prefer Retry-After if present.
		if resp.StatusCode == 429 {
			result.RateLimited = true
			result.ShouldRetry = true
			e.logger.WithContext(ctx).Warnf("Received 429 Too Many Requests from %s", req.URL.String())
		}

		// Update context with response for condition evaluation
		execCtx.WithResponse(resp)
		data = execCtx.ToMap()

		// Evaluate conditions
		if err := e.evaluateConditions(ctx, step, data, result); err != nil {
			result.Error = err
			return result, err
		}

		// Process set_context
		if err := e.processSetContext(ctx, step, data, execCtx); err != nil {
			result.Error = err
			return result, err
		}

		// Retry logic
		if result.ShouldRetry && attempt < maxRetries {
			// If 429 and Retry-After header is present, honor it and also block the rate limiter bucket.
			if resp.StatusCode == 429 {
				if ra, ok := resp.Headers["Retry-After"]; ok && ra != "" {
					if secs, convErr := strconv.Atoi(ra); convErr == nil && secs > 0 {
						delay := time.Duration(secs) * time.Second
						// Proactively block this endpoint bucket for the Retry-After duration to reduce contention.
						if opts != nil && e.rateLimiter != nil {
							checkReq := ratelimit.CheckRequest{
								TenantID:      opts.TenantID,
								IntegrationID: opts.IntegrationID,
								ConfigID:      opts.ConfigID,
								URL:           req.URL.String(),
								RateLimits:    opts.RateLimits,
							}
							e.rateLimiter.UpdateFromResponse(ctx, checkReq, resp.Headers)
						}
						e.logger.WithContext(ctx).Warnf("Retry-After=%ds, retrying (attempt %d/%d)", secs, attempt+1, maxRetries)
						time.Sleep(delay)
						lastResult = result
						continue
					}
				}
			}

			delay := CalculateBackoff(step.Retry, attempt+1)
			e.logger.WithContext(ctx).Warnf("Retrying in %v (attempt %d/%d)", delay, attempt+1, maxRetries)
			time.Sleep(delay)
			lastResult = result
			continue
		}

		return result, nil
	}

	// Should never get here, but return the last attempt result if we do.
	if lastResult != nil {
		return lastResult, lastResult.Error
	}
	return &StepResult{Context: execCtx}, nil
}

// updateRateLimitsFromResponse updates rate limits from response headers
func (e *StepExecutor) updateRateLimitsFromResponse(ctx context.Context, url string, opts *ExecuteOptions, resp *httpclient.Response) {
	checkReq := ratelimit.CheckRequest{
		TenantID:      opts.TenantID,
		IntegrationID: opts.IntegrationID,
		ConfigID:      opts.ConfigID,
		URL:           url,
		RateLimits:    opts.RateLimits,
	}

	e.rateLimiter.UpdateFromResponse(ctx, checkReq, resp.Headers)
}

// applyDefaults applies default values to a step
func (e *StepExecutor) applyDefaults(step *models.Step) *models.Step {
	// Create a copy to avoid mutating the original
	s := *step

	if s.Method == "" {
		s.Method = "GET"
	}
	if s.TimeoutSeconds <= 0 {
		s.TimeoutSeconds = 30
	}
	if s.Concurrency <= 0 {
		s.Concurrency = 50
	}
	if s.Retry == nil {
		defaultRetry := models.DefaultRetryConfig()
		s.Retry = &defaultRetry
	} else {
		if s.Retry.MaxRetries <= 0 {
			s.Retry.MaxRetries = 3
		}
		if s.Retry.BackoffType == "" {
			s.Retry.BackoffType = "fibonacci"
		}
		if s.Retry.InitialDelay <= 0 {
			s.Retry.InitialDelay = 1000
		}
		if s.Retry.MaxDelay <= 0 {
			s.Retry.MaxDelay = 60000
		}
	}

	return &s
}

// evaluateConditions evaluates all step conditions
func (e *StepExecutor) evaluateConditions(ctx context.Context, step *models.Step, data map[string]any, result *StepResult) error {
	// Check abort_when
	if step.AbortWhen != "" {
		abort, err := e.evaluateBoolCondition(ctx, step.AbortWhen, data)
		if err != nil {
			return fmt.Errorf("failed to evaluate abort_when: %w", err)
		}
		if abort {
			result.ShouldAbort = true
			e.logger.WithContext(ctx).Info("Step triggered abort condition")
			return nil
		}
	}

	// Check break_when (for while loops)
	if step.BreakWhen != "" {
		breakLoop, err := e.evaluateBoolCondition(ctx, step.BreakWhen, data)
		if err != nil {
			return fmt.Errorf("failed to evaluate break_when: %w", err)
		}
		if breakLoop {
			result.ShouldBreak = true
			e.logger.WithContext(ctx).Info("Step triggered break condition")
		}
	}

	// Check retry_when
	if step.RetryWhen != "" {
		retry, err := e.evaluateBoolCondition(ctx, step.RetryWhen, data)
		if err != nil {
			return fmt.Errorf("failed to evaluate retry_when: %w", err)
		}
		if retry {
			result.ShouldRetry = true
			e.logger.WithContext(ctx).Info("Step triggered retry condition")
		}
	}

	// Check ignore_when
	if step.IgnoreWhen != "" {
		ignore, err := e.evaluateBoolCondition(ctx, step.IgnoreWhen, data)
		if err != nil {
			return fmt.Errorf("failed to evaluate ignore_when: %w", err)
		}
		if ignore {
			result.ShouldIgnore = true
			e.logger.WithContext(ctx).Info("Step triggered ignore condition")
		}
	}

	return nil
}

// evaluateBoolCondition evaluates a JMESPath expression as a boolean
func (e *StepExecutor) evaluateBoolCondition(ctx context.Context, expr string, data map[string]any) (bool, error) {
	result, err := e.evaluator.EvaluateBool(expr, data)
	if err != nil {
		e.logger.WithContext(ctx).WithError(err).Warnf("Failed to evaluate condition: %s", expr)
		return false, err
	}
	return result, nil
}

// EvaluateWhile evaluates the while condition
func (e *StepExecutor) EvaluateWhile(ctx context.Context, step *models.Step, data map[string]any) (bool, error) {
	if step.While == "" {
		return false, nil
	}
	return e.evaluateBoolCondition(ctx, step.While, data)
}

// processSetContext processes set_context expressions
func (e *StepExecutor) processSetContext(ctx context.Context, step *models.Step, data map[string]any, execCtx *ExecutionContext) error {
	if step.SetContext == nil {
		return nil
	}

	for key, expr := range step.SetContext {
		value, err := e.evaluator.Evaluate(expr, data)
		if err != nil {
			e.logger.WithContext(ctx).WithError(err).Warnf("Failed to evaluate set_context[%s]: %s", key, expr)
			continue // Don't fail the step, just skip this context value
		}

		if err := execCtx.SetContextValue(key, value); err != nil {
			e.logger.WithContext(ctx).WithError(err).Warnf("Failed to set context[%s]", key)
			// Continue, don't fail the step
		}
	}

	return nil
}

// CalculateBackoff calculates the backoff delay for a retry
func CalculateBackoff(retry *models.RetryConfig, attempt int) time.Duration {
	if retry == nil {
		return time.Second
	}

	var delayMs int
	switch retry.BackoffType {
	case "fibonacci":
		delayMs = fibonacciBackoff(retry.InitialDelay, attempt)
	case "exponential":
		delayMs = exponentialBackoff(retry.InitialDelay, attempt)
	case "linear":
		delayMs = linearBackoff(retry.InitialDelay, attempt)
	default:
		delayMs = fibonacciBackoff(retry.InitialDelay, attempt)
	}

	if delayMs > retry.MaxDelay {
		delayMs = retry.MaxDelay
	}

	return time.Duration(delayMs) * time.Millisecond
}

// fibonacciBackoff calculates Fibonacci backoff delay
func fibonacciBackoff(initial int, attempt int) int {
	if attempt <= 1 {
		return initial
	}
	// Fibonacci sequence: 1, 1, 2, 3, 5, 8, 13, 21...
	a, b := 1, 1
	for i := 2; i < attempt; i++ {
		a, b = b, a+b
	}
	return initial * b
}

// exponentialBackoff calculates exponential backoff delay
func exponentialBackoff(initial int, attempt int) int {
	multiplier := 1
	for i := 1; i < attempt; i++ {
		multiplier *= 2
	}
	return initial * multiplier
}

// linearBackoff calculates linear backoff delay
func linearBackoff(initial int, attempt int) int {
	return initial * attempt
}
