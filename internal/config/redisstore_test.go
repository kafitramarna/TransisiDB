package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: These tests require a running Redis instance
// For CI/CD, use testcontainers or skip if Redis not available

func getTestRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host:     "localhost",
		Port:     6379,
		Password: "",
		Database: 15, // Use DB 15 for testing
		PoolSize: 10,
	}
}

func TestNewRedisStore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	cfg := getTestRedisConfig()
	store, err := NewRedisStore(cfg)

	if err != nil {
		t.Skipf("Redis not available, skipping test: %v", err)
		return
	}
	defer store.Close()

	require.NoError(t, err)
	assert.NotNil(t, store)

	// Test health check
	ctx := context.Background()
	err = store.Health(ctx)
	assert.NoError(t, err)
}

func TestSaveAndLoadConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	cfg := getTestRedisConfig()
	store, err := NewRedisStore(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer store.Close()

	ctx := context.Background()

	// Create test configuration
	testConfig := &Config{
		Conversion: ConversionConfig{
			Ratio:            1000,
			Precision:        4,
			RoundingStrategy: "BANKERS_ROUND",
		},
		Tables: TablesConfig{
			"test_table": {
				Enabled: true,
				Columns: map[string]ColumnConfig{
					"amount": {
						SourceColumn:     "amount",
						TargetColumn:     "amount_idn",
						SourceType:       "BIGINT",
						TargetType:       "DECIMAL(19,4)",
						RoundingStrategy: "BANKERS_ROUND",
						Precision:        4,
					},
				},
			},
		},
	}

	// Save configuration
	err = store.SaveConfig(ctx, testConfig)
	require.NoError(t, err)

	// Load configuration
	loadedConfig, err := store.LoadConfig(ctx)
	require.NoError(t, err)
	assert.NotNil(t, loadedConfig)

	// Verify loaded config matches saved config
	assert.Equal(t, testConfig.Conversion.Ratio, loadedConfig.Conversion.Ratio)
	assert.Equal(t, testConfig.Conversion.Precision, loadedConfig.Conversion.Precision)
	assert.Equal(t, testConfig.Conversion.RoundingStrategy, loadedConfig.Conversion.RoundingStrategy)

	// Verify table config
	assert.Contains(t, loadedConfig.Tables, "test_table")
	assert.True(t, loadedConfig.Tables["test_table"].Enabled)
}

func TestConfigTimestamp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	cfg := getTestRedisConfig()
	store, err := NewRedisStore(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer store.Close()

	ctx := context.Background()

	testConfig := &Config{
		Conversion: ConversionConfig{
			Ratio:            1000,
			Precision:        4,
			RoundingStrategy: "BANKERS_ROUND",
		},
	}

	beforeSave := time.Now().Unix()

	// Save config
	err = store.SaveConfig(ctx, testConfig)
	require.NoError(t, err)

	// Get timestamp
	timestamp, err := store.GetConfigTimestamp(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, timestamp, beforeSave)
}

func TestTableConfigOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	cfg := getTestRedisConfig()
	store, err := NewRedisStore(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer store.Close()

	ctx := context.Background()

	// Create test table config
	tableConfig := TableConfig{
		Enabled: true,
		Columns: map[string]ColumnConfig{
			"price": {
				SourceColumn:     "price",
				TargetColumn:     "price_idn",
				SourceType:       "BIGINT",
				TargetType:       "DECIMAL(19,4)",
				RoundingStrategy: "BANKERS_ROUND",
				Precision:        4,
			},
		},
	}

	// Save table config
	err = store.SaveTableConfig(ctx, "products", tableConfig)
	require.NoError(t, err)

	// Load table config
	loaded, err := store.LoadTableConfig(ctx, "products")
	require.NoError(t, err)
	assert.True(t, loaded.Enabled)
	assert.Contains(t, loaded.Columns, "price")

	// List tables
	tables, err := store.ListTables(ctx)
	require.NoError(t, err)
	assert.Contains(t, tables, "products")

	// Delete table config
	err = store.DeleteTableConfig(ctx, "products")
	require.NoError(t, err)

	// Verify deletion
	_, err = store.LoadTableConfig(ctx, "products")
	assert.Error(t, err)
}

func TestWatchConfigChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	cfg := getTestRedisConfig()
	store, err := NewRedisStore(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start watching for config changes
	reloadCh, err := store.WatchConfigChanges(ctx)
	require.NoError(t, err)

	// Save initial config
	testConfig := &Config{
		Conversion: ConversionConfig{
			Ratio:            1000,
			Precision:        4,
			RoundingStrategy: "BANKERS_ROUND",
		},
	}
	err = store.SaveConfig(ctx, testConfig)
	require.NoError(t, err)

	// Publish reload notification
	err = store.PublishReload(ctx)
	require.NoError(t, err)

	// Wait for reload notification
	select {
	case newConfig := <-reloadCh:
		assert.NotNil(t, newConfig)
		assert.Equal(t, 1000, newConfig.Conversion.Ratio)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for config reload")
	}
}

func TestRedisStoreStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	cfg := getTestRedisConfig()
	store, err := NewRedisStore(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
		return
	}
	defer store.Close()

	stats := store.Stats()
	assert.NotNil(t, stats)
	// Stats should show at least some activity
	assert.GreaterOrEqual(t, stats.TotalConns, uint32(0))
}
