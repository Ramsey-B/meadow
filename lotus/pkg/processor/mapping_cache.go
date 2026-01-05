package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Ramsey-B/lotus/pkg/mapping"
)

// MappingRepository loads mapping definitions from storage
type MappingRepository interface {
	GetByID(ctx context.Context, tenantID, mappingID string) (*mapping.MappingDefinition, error)
}

// MappingCache caches compiled mapping definitions
type MappingCache struct {
	cache      map[string]*cacheEntry
	mu         sync.RWMutex
	repo       MappingRepository
	maxSize    int
	ttl        time.Duration
	hits       int64
	misses     int64
}

type cacheEntry struct {
	mapping   *mapping.MappingDefinition
	expiresAt time.Time
}

// MappingCacheConfig configures the mapping cache
type MappingCacheConfig struct {
	MaxSize int
	TTL     time.Duration
}

// DefaultMappingCacheConfig returns sensible defaults
func DefaultMappingCacheConfig() MappingCacheConfig {
	return MappingCacheConfig{
		MaxSize: 1000,
		TTL:     5 * time.Minute,
	}
}

// NewMappingCache creates a new mapping cache
func NewMappingCache(repo MappingRepository, config MappingCacheConfig) *MappingCache {
	return &MappingCache{
		cache:   make(map[string]*cacheEntry),
		repo:    repo,
		maxSize: config.MaxSize,
		ttl:     config.TTL,
	}
}

// cacheKey generates a cache key for a mapping
func cacheKey(tenantID, mappingID string) string {
	return fmt.Sprintf("%s:%s", tenantID, mappingID)
}

// GetCompiledMapping returns a compiled mapping definition
func (c *MappingCache) GetCompiledMapping(ctx context.Context, tenantID, mappingID string) (*mapping.MappingDefinition, error) {
	key := cacheKey(tenantID, mappingID)

	// Check cache first
	c.mu.RLock()
	entry, exists := c.cache[key]
	c.mu.RUnlock()

	if exists && time.Now().Before(entry.expiresAt) {
		c.mu.Lock()
		c.hits++
		c.mu.Unlock()
		return entry.mapping, nil
	}

	// Cache miss or expired - load from repository
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()

	mappingDef, err := c.repo.GetByID(ctx, tenantID, mappingID)
	if err != nil {
		return nil, err
	}

	// Compile the mapping
	if !mappingDef.IsCompiled() {
		if err := mappingDef.Compile(); err != nil {
			return nil, fmt.Errorf("failed to compile mapping: %w", err)
		}
	}

	// Store in cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity (simple LRU - just clear half)
	if len(c.cache) >= c.maxSize {
		c.evictHalf()
	}

	c.cache[key] = &cacheEntry{
		mapping:   mappingDef,
		expiresAt: time.Now().Add(c.ttl),
	}

	return mappingDef, nil
}

// evictHalf removes half the cache entries (must be called with lock held)
func (c *MappingCache) evictHalf() {
	count := 0
	target := len(c.cache) / 2
	for key := range c.cache {
		delete(c.cache, key)
		count++
		if count >= target {
			break
		}
	}
}

// Invalidate removes a specific mapping from the cache
func (c *MappingCache) Invalidate(tenantID, mappingID string) {
	key := cacheKey(tenantID, mappingID)
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

// InvalidateTenant removes all mappings for a tenant from the cache
func (c *MappingCache) InvalidateTenant(tenantID string) {
	prefix := tenantID + ":"
	c.mu.Lock()
	for key := range c.cache {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			delete(c.cache, key)
		}
	}
	c.mu.Unlock()
}

// Clear removes all entries from the cache
func (c *MappingCache) Clear() {
	c.mu.Lock()
	c.cache = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

// CacheStats returns cache statistics
type CacheStats struct {
	Size   int
	Hits   int64
	Misses int64
}

func (c *MappingCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Size:   len(c.cache),
		Hits:   c.hits,
		Misses: c.misses,
	}
}

