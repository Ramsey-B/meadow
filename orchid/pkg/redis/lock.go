package redis

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrLockNotAcquired is returned when a lock cannot be acquired
	ErrLockNotAcquired = errors.New("lock not acquired")
	// ErrLockNotHeld is returned when trying to release a lock not held
	ErrLockNotHeld = errors.New("lock not held")
)

// Lock represents a distributed lock
type Lock struct {
	client *Client
	key    string
	value  string
	ttl    time.Duration
}

// Locker provides distributed locking operations
type Locker struct {
	client    *Client
	keyPrefix string
}

// NewLocker creates a new Locker
func NewLocker(client *Client, keyPrefix string) *Locker {
	if keyPrefix == "" {
		keyPrefix = "lock:"
	}
	return &Locker{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// Acquire attempts to acquire a lock
func (l *Locker) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	lockKey := l.keyPrefix + key
	lockValue := uuid.New().String()

	// Try to set the lock using SET NX (only if not exists)
	ok, err := l.client.rdb.SetNX(ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrLockNotAcquired
	}

	l.client.logger.WithContext(ctx).Debugf("Acquired lock: %s", key)

	return &Lock{
		client: l.client,
		key:    lockKey,
		value:  lockValue,
		ttl:    ttl,
	}, nil
}

// TryAcquire attempts to acquire a lock, retrying with backoff
func (l *Locker) TryAcquire(ctx context.Context, key string, ttl time.Duration, timeout time.Duration) (*Lock, error) {
	deadline := time.Now().Add(timeout)
	backoff := 10 * time.Millisecond

	for time.Now().Before(deadline) {
		lock, err := l.Acquire(ctx, key, ttl)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, ErrLockNotAcquired) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			// Exponential backoff with cap
			backoff = backoff * 2
			if backoff > 500*time.Millisecond {
				backoff = 500 * time.Millisecond
			}
		}
	}

	return nil, ErrLockNotAcquired
}

// Release releases the lock
func (lock *Lock) Release(ctx context.Context) error {
	// Use Lua script to ensure we only delete if we own the lock
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, lock.client.rdb, []string{lock.key}, lock.value).Int64()
	if err != nil {
		return err
	}

	if result == 0 {
		return ErrLockNotHeld
	}

	lock.client.logger.WithContext(ctx).Debugf("Released lock: %s", lock.key)
	return nil
}

// Extend extends the lock's TTL
func (lock *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	// Use Lua script to ensure we only extend if we own the lock
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, lock.client.rdb, []string{lock.key}, lock.value, ttl.Milliseconds()).Int64()
	if err != nil {
		return err
	}

	if result == 0 {
		return ErrLockNotHeld
	}

	lock.ttl = ttl
	return nil
}

// WithLock executes a function while holding a lock
func (l *Locker) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	lock, err := l.Acquire(ctx, key, ttl)
	if err != nil {
		return err
	}
	defer lock.Release(ctx)

	return fn()
}

