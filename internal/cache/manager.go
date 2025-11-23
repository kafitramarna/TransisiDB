package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Config holds cache configuration
type Config struct {
	Enabled        bool                        `yaml:"enabled"`
	RedisAddr      string                      `yaml:"redis_addr"`
	RedisPassword  string                      `yaml:"redis_password"`
	RedisDB        int                         `yaml:"redis_db"`
	DefaultTTL     time.Duration               `yaml:"default_ttl"`
	MaxMemory      string                      `yaml:"max_memory"`
	EvictionPolicy string                      `yaml:"eviction_policy"` // allkeys-lru, volatile-lru, etc.
	TableConfigs   map[string]TableCacheConfig `yaml:"table_configs"`
}

// TableCacheConfig holds per-table cache configuration
type TableCacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
}

// Manager manages query result caching
type Manager struct {
	client  *redis.Client
	config  *Config
	ctx     context.Context
	enabled bool
	stats   *Stats
}

// Stats tracks cache performance metrics
type Stats struct {
	Hits          int64
	Misses        int64
	Writes        int64
	Invalidations int64
	Errors        int64
}

// CacheEntry represents a cached query result
type CacheEntry struct {
	Query     string                   `json:"query"`
	Results   []map[string]interface{} `json:"results"`
	CachedAt  time.Time                `json:"cached_at"`
	ExpiresAt time.Time                `json:"expires_at"`
	TableName string                   `json:"table_name"`
}

// NewManager creates a new cache manager
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil || !cfg.Enabled {
		return &Manager{enabled: false}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Set max memory policy if specified
	if cfg.MaxMemory != "" && cfg.EvictionPolicy != "" {
		client.ConfigSet(ctx, "maxmemory", cfg.MaxMemory)
		client.ConfigSet(ctx, "maxmemory-policy", cfg.EvictionPolicy)
	}

	return &Manager{
		client:  client,
		config:  cfg,
		ctx:     ctx,
		enabled: true,
		stats:   &Stats{},
	}, nil
}

// Get retrieves cached query result
func (m *Manager) Get(query string, tableName string) (*CacheEntry, error) {
	if !m.enabled {
		return nil, fmt.Errorf("cache disabled")
	}

	// Check if table caching is enabled
	if !m.isTableCachingEnabled(tableName) {
		m.stats.Misses++
		return nil, fmt.Errorf("caching disabled for table: %s", tableName)
	}

	key := m.generateKey(query, tableName)

	data, err := m.client.Get(m.ctx, key).Bytes()
	if err == redis.Nil {
		m.stats.Misses++
		return nil, fmt.Errorf("cache miss")
	} else if err != nil {
		m.stats.Errors++
		return nil, fmt.Errorf("cache get error: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		m.stats.Errors++
		return nil, fmt.Errorf("cache unmarshal error: %w", err)
	}

	// Check if expired (double-check)
	if time.Now().After(entry.ExpiresAt) {
		m.stats.Misses++
		m.client.Del(m.ctx, key) // Clean up
		return nil, fmt.Errorf("cache expired")
	}

	m.stats.Hits++
	return &entry, nil
}

// Set stores query result in cache
func (m *Manager) Set(query string, tableName string, results []map[string]interface{}) error {
	if !m.enabled {
		return fmt.Errorf("cache disabled")
	}

	// Check if table caching is enabled
	if !m.isTableCachingEnabled(tableName) {
		return nil // Skip silently
	}

	ttl := m.getTTL(tableName)
	now := time.Now()

	entry := CacheEntry{
		Query:     query,
		Results:   results,
		CachedAt:  now,
		ExpiresAt: now.Add(ttl),
		TableName: tableName,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		m.stats.Errors++
		return fmt.Errorf("cache marshal error: %w", err)
	}

	key := m.generateKey(query, tableName)

	if err := m.client.Set(m.ctx, key, data, ttl).Err(); err != nil {
		m.stats.Errors++
		return fmt.Errorf("cache set error: %w", err)
	}

	m.stats.Writes++
	return nil
}

// Invalidate removes cached entries for a table
func (m *Manager) Invalidate(tableName string) error {
	if !m.enabled {
		return nil
	}

	// Pattern: cache:table:<tableName>:*
	pattern := fmt.Sprintf("cache:table:%s:*", tableName)

	keys, err := m.client.Keys(m.ctx, pattern).Result()
	if err != nil {
		m.stats.Errors++
		return fmt.Errorf("cache invalidate error: %w", err)
	}

	if len(keys) > 0 {
		if err := m.client.Del(m.ctx, keys...).Err(); err != nil {
			m.stats.Errors++
			return fmt.Errorf("cache delete error: %w", err)
		}
		m.stats.Invalidations += int64(len(keys))
	}

	return nil
}

// InvalidateAll clears entire cache
func (m *Manager) InvalidateAll() error {
	if !m.enabled {
		return nil
	}

	if err := m.client.FlushDB(m.ctx).Err(); err != nil {
		m.stats.Errors++
		return fmt.Errorf("cache flush error: %w", err)
	}

	return nil
}

// generateKey creates cache key from query and table
func (m *Manager) generateKey(query string, tableName string) string {
	// Use MD5 hash of query for consistent key length
	hash := md5.Sum([]byte(query))
	queryHash := hex.EncodeToString(hash[:])

	return fmt.Sprintf("cache:table:%s:query:%s", tableName, queryHash)
}

// isTableCachingEnabled checks if caching is enabled for table
func (m *Manager) isTableCachingEnabled(tableName string) bool {
	if tableConfig, exists := m.config.TableConfigs[tableName]; exists {
		return tableConfig.Enabled
	}
	// Default: enabled if global cache is enabled
	return m.enabled
}

// getTTL returns TTL for table
func (m *Manager) getTTL(tableName string) time.Duration {
	if tableConfig, exists := m.config.TableConfigs[tableName]; exists {
		if tableConfig.TTL > 0 {
			return tableConfig.TTL
		}
	}
	return m.config.DefaultTTL
}

// GetStats returns cache statistics
func (m *Manager) GetStats() Stats {
	return *m.stats
}

// GetHitRate returns cache hit rate percentage
func (m *Manager) GetHitRate() float64 {
	total := m.stats.Hits + m.stats.Misses
	if total == 0 {
		return 0.0
	}
	return float64(m.stats.Hits) / float64(total) * 100.0
}

// Close closes Redis connection
func (m *Manager) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// IsEnabled returns whether cache is enabled
func (m *Manager) IsEnabled() bool {
	return m.enabled
}
