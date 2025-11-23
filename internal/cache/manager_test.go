package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager_Disabled(t *testing.T) {
	cfg := &Config{
		Enabled: false,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	assert.False(t, manager.IsEnabled())
}

func TestNewManager_NilConfig(t *testing.T) {
	manager, err := NewManager(nil)
	require.NoError(t, err)
	assert.False(t, manager.IsEnabled())
}

func TestGenerateKey(t *testing.T) {
	cfg := &Config{Enabled: false}
	manager, _ := NewManager(cfg)

	key1 := manager.generateKey("SELECT * FROM orders", "orders")
	key2 := manager.generateKey("SELECT * FROM orders", "orders")
	key3 := manager.generateKey("SELECT * FROM users", "users")

	// Same query + table = same key
	assert.Equal(t, key1, key2)

	// Different table = different key
	assert.NotEqual(t, key1, key3)

	// Key format check
	assert.Contains(t, key1, "cache:table:orders:query:")
}

func TestIsTableCachingEnabled_Default(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		TableConfigs: map[string]TableCacheConfig{
			"orders": {Enabled: true},
			"users":  {Enabled: false},
		},
	}

	manager := &Manager{
		config:  cfg,
		enabled: true,
	}

	assert.True(t, manager.isTableCachingEnabled("orders"))
	assert.False(t, manager.isTableCachingEnabled("users"))
	assert.True(t, manager.isTableCachingEnabled("products")) // Default to enabled
}

func TestGetTTL(t *testing.T) {
	defaultTTL := 60 * time.Second
	customTTL := 30 * time.Second

	cfg := &Config{
		Enabled:    true,
		DefaultTTL: defaultTTL,
		TableConfigs: map[string]TableCacheConfig{
			"orders": {
				Enabled: true,
				TTL:     customTTL,
			},
		},
	}

	manager := &Manager{
		config:  cfg,
		enabled: true,
	}

	assert.Equal(t, customTTL, manager.getTTL("orders"))
	assert.Equal(t, defaultTTL, manager.getTTL("products"))
}

func TestGetStats(t *testing.T) {
	manager := &Manager{
		enabled: false,
		stats: &Stats{
			Hits:          100,
			Misses:        50,
			Writes:        75,
			Invalidations: 10,
			Errors:        5,
		},
	}

	stats := manager.GetStats()
	assert.Equal(t, int64(100), stats.Hits)
	assert.Equal(t, int64(50), stats.Misses)
	assert.Equal(t, int64(75), stats.Writes)
	assert.Equal(t, int64(10), stats.Invalidations)
	assert.Equal(t, int64(5), stats.Errors)
}

func TestGetHitRate(t *testing.T) {
	tests := []struct {
		name     string
		hits     int64
		misses   int64
		expected float64
	}{
		{"100% hit rate", 100, 0, 100.0},
		{"0% hit rate", 0, 100, 0.0},
		{"75% hit rate", 75, 25, 75.0},
		{"50% hit rate", 50, 50, 50.0},
		{"No data", 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				stats: &Stats{
					Hits:   tt.hits,
					Misses: tt.misses,
				},
			}

			hitRate := manager.GetHitRate()
			assert.InDelta(t, tt.expected, hitRate, 0.01)
		})
	}
}

func TestCacheEntry_Struct(t *testing.T) {
	now := time.Now()
	entry := CacheEntry{
		Query: "SELECT * FROM orders",
		Results: []map[string]interface{}{
			{"id": 1, "total": 50000},
			{"id": 2, "total": 75000},
		},
		CachedAt:  now,
		ExpiresAt: now.Add(60 * time.Second),
		TableName: "orders",
	}

	assert.Equal(t, "SELECT * FROM orders", entry.Query)
	assert.Equal(t, 2, len(entry.Results))
	assert.Equal(t, "orders", entry.TableName)
	assert.True(t, entry.ExpiresAt.After(entry.CachedAt))
}

func TestConfig_Structure(t *testing.T) {
	cfg := Config{
		Enabled:        true,
		RedisAddr:      "localhost:6379",
		RedisPassword:  "secret",
		RedisDB:        0,
		DefaultTTL:     60 * time.Second,
		MaxMemory:      "1GB",
		EvictionPolicy: "allkeys-lru",
		TableConfigs: map[string]TableCacheConfig{
			"orders": {
				Enabled: true,
				TTL:     30 * time.Second,
			},
		},
	}

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "localhost:6379", cfg.RedisAddr)
	assert.Equal(t, "1GB", cfg.MaxMemory)
	assert.Equal(t, 1, len(cfg.TableConfigs))
}

// Integration tests (require Redis)
func TestManager_Integration(t *testing.T) {
	// Skip if Redis not available
	t.Skip("Requires Redis connection")

	cfg := &Config{
		Enabled:    true,
		RedisAddr:  "localhost:6379",
		DefaultTTL: 60 * time.Second,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	defer manager.Close()

	// Test Set
	results := []map[string]interface{}{
		{"id": 1, "name": "Product 1"},
		{"id": 2, "name": "Product 2"},
	}

	err = manager.Set("SELECT * FROM products", "products", results)
	assert.NoError(t, err)

	// Test Get (hit)
	entry, err := manager.Get("SELECT * FROM products", "products")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(entry.Results))

	// Test Get (miss)
	_, err = manager.Get("SELECT * FROM orders", "orders")
	assert.Error(t, err)

	// Test Invalidate
	err = manager.Invalidate("products")
	assert.NoError(t, err)

	// Verify invalidation
	_, err = manager.Get("SELECT * FROM products", "products")
	assert.Error(t, err) // Should be cache miss now
}

func BenchmarkGenerateKey(b *testing.B) {
	cfg := &Config{Enabled: false}
	manager, _ := NewManager(cfg)
	query := "SELECT * FROM orders WHERE id = 123 AND status = 'active'"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.generateKey(query, "orders")
	}
}

func BenchmarkGetHitRate(b *testing.B) {
	manager := &Manager{
		stats: &Stats{
			Hits:   1000000,
			Misses: 500000,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetHitRate()
	}
}
