package ratelimit

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"

	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/redis"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// Manager handles rate limiting for API requests
type Manager struct {
	limiter *redis.RateLimiter
	logger  ectologger.Logger
	locker  *redis.Locker

	// Cache compiled regexes for endpoint matching
	regexCache map[string]*regexp.Regexp
	regexMu    sync.RWMutex
}

// NewManager creates a new rate limit manager
func NewManager(redisClient *redis.Client, logger ectologger.Logger) *Manager {
	return &Manager{
		limiter:    redis.NewRateLimiter(redisClient, "orchid:ratelimit:"),
		locker:     redis.NewLocker(redisClient, "orchid:concurrency:"),
		logger:     logger,
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// CheckRequest represents a rate limit check request
type CheckRequest struct {
	TenantID      uuid.UUID
	IntegrationID uuid.UUID
	ConfigID      uuid.UUID
	URL           string
	RateLimits    []models.RateLimitConfig
}

// CheckResult represents the result of a rate limit check
type CheckResult struct {
	Allowed     bool
	RetryAfter  time.Duration
	LimitName   string
	Remaining   int64
	WaitAndRetry bool // If true, caller should wait RetryAfter and retry

	// Release should be called by the caller when the request completes to release any acquired concurrency slot.
	Release func()
}

// Check checks if a request is allowed under all applicable rate limits
func (m *Manager) Check(ctx context.Context, req CheckRequest) (*CheckResult, error) {
	ctx, span := tracing.StartSpan(ctx, "RateLimitManager.Check")
	defer span.End()

	if len(req.RateLimits) == 0 {
		return &CheckResult{Allowed: true}, nil
	}

	// Find matching rate limits for this URL
	matchingLimits := m.findMatchingLimits(req.URL, req.RateLimits)
	if len(matchingLimits) == 0 {
		return &CheckResult{Allowed: true}, nil
	}

	// Track any acquired concurrency slots so we can release on deny.
	releases := make([]func(), 0, 2)

	// Check each matching limit
	for _, limit := range matchingLimits {
		key := m.buildKey(req, limit)
		window := time.Duration(limit.WindowSecs) * time.Second

		// Concurrency limiting (max in-flight) for this bucket
		if limit.MaxConcurrent > 0 && m.locker != nil {
			// Spread across N sub-keys by trying to acquire any of them.
			var acquired *redis.Lock
			for i := 0; i < limit.MaxConcurrent; i++ {
				lockKey := fmt.Sprintf("%s:slot:%d", key, i)
				lock, err := m.locker.Acquire(ctx, lockKey, 2*time.Minute)
				if err == nil {
					acquired = lock
					break
				}
				if err != nil && err != redis.ErrLockNotAcquired {
					return nil, err
				}
			}
			if acquired == nil {
				// No concurrency slot available; ask caller to wait briefly and retry.
				for _, rel := range releases {
					rel()
				}
				return &CheckResult{
					Allowed:      false,
					RetryAfter:   200 * time.Millisecond,
					LimitName:    limit.Name,
					Remaining:    0,
					WaitAndRetry: true,
				}, nil
			}
			releases = append(releases, func() { _ = acquired.Release(ctx) })
		}

		// Dynamic block (e.g. Retry-After) takes precedence over the sliding window.
		if blocked, ttl, err := m.limiter.IsBlocked(ctx, key); err == nil && blocked {
			for _, rel := range releases {
				rel()
			}
			return &CheckResult{
				Allowed:      false,
				RetryAfter:   ttl,
				LimitName:    limit.Name,
				Remaining:    0,
				WaitAndRetry: true,
			}, nil
		}

		result, err := m.limiter.Allow(ctx, key, int64(limit.Requests), window)
		if err != nil {
			m.logger.WithContext(ctx).WithError(err).Errorf("Rate limit check failed for %s", limit.Name)
			// On error, allow the request (fail open)
			continue
		}

		if !result.Allowed {
			m.logger.WithContext(ctx).Warnf("Rate limit exceeded for %s: retry in %v", limit.Name, result.RetryIn)
			for _, rel := range releases {
				rel()
			}
			return &CheckResult{
				Allowed:      false,
				RetryAfter:   result.RetryIn,
				LimitName:    limit.Name,
				Remaining:    0,
				WaitAndRetry: true,
			}, nil
		}

		m.logger.WithContext(ctx).Debugf("Rate limit %s: %d remaining", limit.Name, result.Remaining)
	}

	// Success: return a composed release function (may be nil).
	var releaseFn func()
	if len(releases) > 0 {
		releaseFn = func() {
			for _, rel := range releases {
				rel()
			}
		}
	}
	return &CheckResult{Allowed: true, Release: releaseFn}, nil
}

// WaitForLimit waits until the rate limit allows the request
// Returns an error if the context is cancelled
func (m *Manager) WaitForLimit(ctx context.Context, req CheckRequest, maxWait time.Duration) (func(), error) {
	ctx, span := tracing.StartSpan(ctx, "RateLimitManager.WaitForLimit")
	defer span.End()

	deadline := time.Now().Add(maxWait)

	for {
		result, err := m.Check(ctx, req)
		if err != nil {
			return nil, err
		}

		if result.Allowed {
			return result.Release, nil
		}

		// Check if we'd exceed max wait
		if time.Now().Add(result.RetryAfter).After(deadline) {
			return nil, fmt.Errorf("rate limit %s would exceed max wait time of %v", result.LimitName, maxWait)
		}

		m.logger.WithContext(ctx).Infof("Rate limited by %s, waiting %v", result.LimitName, result.RetryAfter)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(result.RetryAfter):
			// Continue and check again
		}
	}
}

// UpdateFromResponse updates rate limits based on response headers
func (m *Manager) UpdateFromResponse(ctx context.Context, req CheckRequest, headers map[string]string) {
	ctx, span := tracing.StartSpan(ctx, "RateLimitManager.UpdateFromResponse")
	defer span.End()

	for _, limit := range req.RateLimits {
		if limit.Dynamic == nil {
			continue
		}

		m.updateDynamicLimit(ctx, req, limit, headers)
	}
}

// updateDynamicLimit updates a rate limit based on response headers
func (m *Manager) updateDynamicLimit(ctx context.Context, req CheckRequest, limit models.RateLimitConfig, headers map[string]string) {
	dynamic := limit.Dynamic

	// Extract remaining from header
	if dynamic.RemainingHeader != "" {
		if remaining, ok := headers[dynamic.RemainingHeader]; ok {
			if remainingInt, err := strconv.ParseInt(remaining, 10, 64); err == nil {
				m.logger.WithContext(ctx).Debugf("Dynamic rate limit %s: %d remaining from header", limit.Name, remainingInt)
				// If remaining hits 0 and we have a reset header, block until reset.
				if remainingInt == 0 && dynamic.ResetHeader != "" {
					if reset, ok := headers[dynamic.ResetHeader]; ok {
						if resetTime, err := strconv.ParseInt(reset, 10, 64); err == nil {
							resetAt := time.Unix(resetTime, 0)
							d := time.Until(resetAt)
							if d > 0 {
								_ = m.limiter.BlockFor(ctx, m.buildKey(req, limit), d)
							}
						}
					}
				}
			}
		}
	}

	// Extract limit from header
	if dynamic.LimitHeader != "" {
		if limitVal, ok := headers[dynamic.LimitHeader]; ok {
			if limitInt, err := strconv.ParseInt(limitVal, 10, 64); err == nil {
				m.logger.WithContext(ctx).Debugf("Dynamic rate limit %s: limit is %d from header", limit.Name, limitInt)
			}
		}
	}

	// Extract reset time from header
	if dynamic.ResetHeader != "" {
		if reset, ok := headers[dynamic.ResetHeader]; ok {
			// Could be epoch seconds or a duration
			if resetTime, err := strconv.ParseInt(reset, 10, 64); err == nil {
				resetAt := time.Unix(resetTime, 0)
				m.logger.WithContext(ctx).Debugf("Dynamic rate limit %s: resets at %v", limit.Name, resetAt)
			}
		}
	}
}

// ParseRetryAfter parses a Retry-After header value
// Returns the duration to wait before retrying
func ParseRetryAfter(value string) (time.Duration, error) {
	// Try parsing as seconds
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}

	// Try parsing as HTTP date (RFC 1123)
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		return time.Until(t), nil
	}

	return 0, fmt.Errorf("invalid Retry-After value: %s", value)
}

// findMatchingLimits returns rate limits that match the given URL
func (m *Manager) findMatchingLimits(url string, limits []models.RateLimitConfig) []models.RateLimitConfig {
	var matching []models.RateLimitConfig

	for _, limit := range limits {
		if limit.Endpoint == "" {
			// No endpoint pattern means it matches all
			matching = append(matching, limit)
			continue
		}

		re := m.getOrCompileRegex(limit.Endpoint)
		if re != nil && re.MatchString(url) {
			matching = append(matching, limit)
		}
	}

	return matching
}

// getOrCompileRegex gets a cached regex or compiles and caches a new one
func (m *Manager) getOrCompileRegex(pattern string) *regexp.Regexp {
	m.regexMu.RLock()
	if re, ok := m.regexCache[pattern]; ok {
		m.regexMu.RUnlock()
		return re
	}
	m.regexMu.RUnlock()

	// Compile and cache
	m.regexMu.Lock()
	defer m.regexMu.Unlock()

	// Double check after acquiring write lock
	if re, ok := m.regexCache[pattern]; ok {
		return re
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		m.logger.Errorf("Failed to compile rate limit regex pattern %s: %v", pattern, err)
		return nil
	}

	m.regexCache[pattern] = re
	return re
}

// buildKey builds the Redis key for a rate limit bucket
func (m *Manager) buildKey(req CheckRequest, limit models.RateLimitConfig) string {
	base := fmt.Sprintf("%s:%s", req.TenantID, limit.Name)

	switch limit.Scope {
	case "per_config":
		return fmt.Sprintf("%s:config:%s", base, req.ConfigID)
	case "per_endpoint":
		return fmt.Sprintf("%s:endpoint:%s", base, req.URL)
	case "global", "":
		return base
	default:
		return base
	}
}

// Reset resets a rate limit bucket
func (m *Manager) Reset(ctx context.Context, tenantID uuid.UUID, limitName string) error {
	key := fmt.Sprintf("%s:%s", tenantID, limitName)
	return m.limiter.Reset(ctx, key)
}

// GetRemaining returns the remaining requests for a rate limit
func (m *Manager) GetRemaining(ctx context.Context, req CheckRequest, limitName string) (int64, error) {
	// Find the limit config
	var limit *models.RateLimitConfig
	for _, l := range req.RateLimits {
		if l.Name == limitName {
			limit = &l
			break
		}
	}

	if limit == nil {
		return 0, fmt.Errorf("rate limit %s not found", limitName)
	}

	key := m.buildKey(req, *limit)
	window := time.Duration(limit.WindowSecs) * time.Second

	return m.limiter.GetRemaining(ctx, key, int64(limit.Requests), window)
}

