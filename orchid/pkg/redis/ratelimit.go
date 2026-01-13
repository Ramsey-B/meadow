package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var (
	// ErrRateLimitExceeded is returned when the rate limit is exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool
	Remaining int64
	ResetAt   time.Time
	RetryIn   time.Duration
}

// RateLimiter provides rate limiting using Redis
type RateLimiter struct {
	client    *Client
	keyPrefix string
}

func toInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case float64:
		return int64(n), nil
	case string:
		// Redis Lua returns numbers as strings sometimes (e.g., zrange WITHSCORES)
		parsed, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			// Try float parse then cast
			f, ferr := strconv.ParseFloat(n, 64)
			if ferr != nil {
				return 0, err
			}
			return int64(f), nil
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected numeric type %T", v)
	}
}

func (r *RateLimiter) blockKey(key string) string {
	return r.keyPrefix + key + ":block"
}

// BlockFor blocks a rate limit key for the given duration.
// This is used for dynamic throttling when an API tells us to back off (e.g. 429 Retry-After).
func (r *RateLimiter) BlockFor(ctx context.Context, key string, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	return r.client.Set(ctx, r.blockKey(key), "1", d)
}

// IsBlocked returns whether the key is currently blocked and, if so, for how long.
func (r *RateLimiter) IsBlocked(ctx context.Context, key string) (bool, time.Duration, error) {
	exists, err := r.client.Exists(ctx, r.blockKey(key))
	if err != nil {
		return false, 0, err
	}
	if !exists {
		return false, 0, nil
	}
	ttl, err := r.client.TTL(ctx, r.blockKey(key))
	if err != nil {
		return true, 0, err
	}
	if ttl < 0 {
		ttl = 0
	}
	return true, ttl, nil
}

// NewRateLimiter creates a new RateLimiter
func NewRateLimiter(client *Client, keyPrefix string) *RateLimiter {
	if keyPrefix == "" {
		keyPrefix = "ratelimit:"
	}
	return &RateLimiter{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// Allow checks if a request is allowed under the rate limit
// Uses sliding window algorithm
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	rateKey := r.keyPrefix + key

	// If the key is dynamically blocked (e.g. Retry-After), fail closed for the duration.
	if blocked, ttl, err := r.IsBlocked(ctx, key); err == nil && blocked {
		return &RateLimitResult{
			Allowed:   false,
			Remaining: 0,
			ResetAt:   now.Add(ttl),
			RetryIn:   ttl,
		}, nil
	}

	// Use Lua script for atomic operation
	script := goredis.NewScript(`
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local window_ms = tonumber(ARGV[4])
		
		-- Remove old entries
		redis.call("zremrangebyscore", key, "-inf", window_start)
		
		-- Count current entries
		local current = redis.call("zcard", key)
		
		if current < limit then
			-- Add new entry
			redis.call("zadd", key, now, now .. "-" .. math.random())
			redis.call("pexpire", key, window_ms)
			return {1, limit - current - 1}
		else
			-- Get oldest entry to calculate retry time
			local oldest = redis.call("zrange", key, 0, 0, "WITHSCORES")
			if #oldest > 0 then
				return {0, 0, oldest[2]}
			end
			return {0, 0, 0}
		end
	`)

	result, err := script.Run(ctx, r.client.rdb, []string{rateKey},
		now.UnixMilli(),
		windowStart.UnixMilli(),
		limit,
		window.Milliseconds(),
	).Slice()

	if err != nil {
		return nil, err
	}

	allowedFlag, err := toInt64(result[0])
	if err != nil {
		return nil, err
	}
	remaining, err := toInt64(result[1])
	if err != nil {
		return nil, err
	}
	allowed := allowedFlag == 1

	res := &RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   now.Add(window),
	}

	if !allowed && len(result) > 2 {
		oldestMs, err := toInt64(result[2])
		if err != nil {
			return nil, err
		}
		if oldestMs > 0 {
			oldestTime := time.UnixMilli(oldestMs)
			res.RetryIn = oldestTime.Add(window).Sub(now)
		}
	}

	return res, nil
}

// AllowN checks if N requests are allowed under the rate limit
func (r *RateLimiter) AllowN(ctx context.Context, key string, limit int64, window time.Duration, n int64) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	rateKey := r.keyPrefix + key

	script := goredis.NewScript(`
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local window_ms = tonumber(ARGV[4])
		local n = tonumber(ARGV[5])
		
		redis.call("zremrangebyscore", key, "-inf", window_start)
		local current = redis.call("zcard", key)
		
		if current + n <= limit then
			for i = 1, n do
				redis.call("zadd", key, now, now .. "-" .. math.random() .. "-" .. i)
			end
			redis.call("pexpire", key, window_ms)
			return {1, limit - current - n}
		else
			local oldest = redis.call("zrange", key, 0, 0, "WITHSCORES")
			if #oldest > 0 then
				return {0, 0, oldest[2]}
			end
			return {0, 0, 0}
		end
	`)

	result, err := script.Run(ctx, r.client.rdb, []string{rateKey},
		now.UnixMilli(),
		windowStart.UnixMilli(),
		limit,
		window.Milliseconds(),
		n,
	).Slice()

	if err != nil {
		return nil, err
	}

	allowedFlag, err := toInt64(result[0])
	if err != nil {
		return nil, err
	}
	remaining, err := toInt64(result[1])
	if err != nil {
		return nil, err
	}
	allowed := allowedFlag == 1

	res := &RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   now.Add(window),
	}

	if !allowed && len(result) > 2 {
		oldestMs, err := toInt64(result[2])
		if err != nil {
			return nil, err
		}
		if oldestMs > 0 {
			oldestTime := time.UnixMilli(oldestMs)
			res.RetryIn = oldestTime.Add(window).Sub(now)
		}
	}

	return res, nil
}

// Reset resets the rate limit for a key
func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	return r.client.rdb.Del(ctx, r.keyPrefix+key).Err()
}

// GetRemaining returns the remaining requests for a key
func (r *RateLimiter) GetRemaining(ctx context.Context, key string, limit int64, window time.Duration) (int64, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	rateKey := r.keyPrefix + key

	// Remove old entries and count
	pipe := r.client.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, rateKey, "-inf", fmt.Sprintf("%d", windowStart.UnixMilli()))
	countCmd := pipe.ZCard(ctx, rateKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	current := countCmd.Val()
	remaining := limit - current
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

