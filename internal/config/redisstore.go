package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes
	ConfigKeyPrefix = "transisidb:config"
	ConfigChannel   = "transisidb:config:reload"
)

// RedisStore manages configuration in Redis with hot-reload capability
type RedisStore struct {
	client   *redis.Client
	cfg      *RedisConfig
	pubsub   *redis.PubSub
	reloadCh chan *Config
	closeCh  chan struct{}
}

// NewRedisStore creates a new Redis configuration store
func NewRedisStore(cfg *RedisConfig) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.Database,
		PoolSize: cfg.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	store := &RedisStore{
		client:   client,
		cfg:      cfg,
		reloadCh: make(chan *Config, 10),
		closeCh:  make(chan struct{}),
	}

	return store, nil
}

// SaveConfig saves configuration to Redis
func (s *RedisStore) SaveConfig(ctx context.Context, cfg *Config) error {
	// Convert config to JSON
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Save to Redis with version timestamp
	key := fmt.Sprintf("%s:main", ConfigKeyPrefix)
	if err := s.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save config to Redis: %w", err)
	}

	// Save timestamp
	timestampKey := fmt.Sprintf("%s:timestamp", ConfigKeyPrefix)
	if err := s.client.Set(ctx, timestampKey, time.Now().Unix(), 0).Err(); err != nil {
		return fmt.Errorf("failed to save timestamp: %w", err)
	}

	return nil
}

// LoadConfig loads configuration from Redis
func (s *RedisStore) LoadConfig(ctx context.Context) (*Config, error) {
	key := fmt.Sprintf("%s:main", ConfigKeyPrefix)

	data, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("config not found in Redis")
	} else if err != nil {
		return nil, fmt.Errorf("failed to load config from Redis: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// PublishReload publishes a reload notification
func (s *RedisStore) PublishReload(ctx context.Context) error {
	return s.client.Publish(ctx, ConfigChannel, "reload").Err()
}

// WatchConfigChanges watches for configuration changes via Redis Pub/Sub
func (s *RedisStore) WatchConfigChanges(ctx context.Context) (<-chan *Config, error) {
	// Subscribe to config reload channel
	s.pubsub = s.client.Subscribe(ctx, ConfigChannel)

	// Wait for subscription confirmation
	if _, err := s.pubsub.Receive(ctx); err != nil {
		return nil, fmt.Errorf("failed to subscribe to config channel: %w", err)
	}

	// Start watching in background
	go s.watchLoop(ctx)

	return s.reloadCh, nil
}

// watchLoop listens for reload messages and loads new config
func (s *RedisStore) watchLoop(ctx context.Context) {
	ch := s.pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closeCh:
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			// Load new configuration
			newCfg, err := s.LoadConfig(ctx)
			if err != nil {
				// Log error but continue watching
				fmt.Printf("Error loading config after reload notification: %v\n", err)
				continue
			}

			// Send to reload channel (non-blocking)
			select {
			case s.reloadCh <- newCfg:
			default:
				// Channel full, skip this update
			}
		}
	}
}

// GetConfigTimestamp returns the last config update timestamp
func (s *RedisStore) GetConfigTimestamp(ctx context.Context) (int64, error) {
	key := fmt.Sprintf("%s:timestamp", ConfigKeyPrefix)

	result, err := s.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to get timestamp: %w", err)
	}

	return result, nil
}

// SaveTableConfig saves individual table configuration
func (s *RedisStore) SaveTableConfig(ctx context.Context, tableName string, tableConfig TableConfig) error {
	data, err := json.Marshal(tableConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal table config: %w", err)
	}

	key := fmt.Sprintf("%s:tables:%s", ConfigKeyPrefix, tableName)
	return s.client.Set(ctx, key, data, 0).Err()
}

// LoadTableConfig loads individual table configuration
func (s *RedisStore) LoadTableConfig(ctx context.Context, tableName string) (*TableConfig, error) {
	key := fmt.Sprintf("%s:tables:%s", ConfigKeyPrefix, tableName)

	data, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("table config not found: %s", tableName)
	} else if err != nil {
		return nil, fmt.Errorf("failed to load table config: %w", err)
	}

	var tableConfig TableConfig
	if err := json.Unmarshal([]byte(data), &tableConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal table config: %w", err)
	}

	return &tableConfig, nil
}

// ListTables returns list of configured tables
func (s *RedisStore) ListTables(ctx context.Context) ([]string, error) {
	pattern := fmt.Sprintf("%s:tables:*", ConfigKeyPrefix)

	var tables []string
	iter := s.client.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		// Extract table name from key
		// Key format: transisidb:config:tables:tablename
		parts := len(ConfigKeyPrefix) + len(":tables:")
		if len(key) > parts {
			tableName := key[parts:]
			tables = append(tables, tableName)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan tables: %w", err)
	}

	return tables, nil
}

// DeleteTableConfig deletes a table configuration
func (s *RedisStore) DeleteTableConfig(ctx context.Context, tableName string) error {
	key := fmt.Sprintf("%s:tables:%s", ConfigKeyPrefix, tableName)
	return s.client.Del(ctx, key).Err()
}

// Health checks Redis connection health
func (s *RedisStore) Health(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Close closes the Redis store and cleanup resources
func (s *RedisStore) Close() error {
	close(s.closeCh)

	if s.pubsub != nil {
		if err := s.pubsub.Close(); err != nil {
			return err
		}
	}

	close(s.reloadCh)

	return s.client.Close()
}

// Stats returns Redis client statistics
func (s *RedisStore) Stats() *redis.PoolStats {
	return s.client.PoolStats()
}
