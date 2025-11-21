package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Database          DatabaseConfig    `yaml:"database"`
	Proxy             ProxyConfig       `yaml:"proxy"`
	TLS               TLSConfig         `yaml:"tls"`     // v2.0: TLS/SSL configuration
	Replica           ReplicaConfig     `yaml:"replica"` // v2.0: Read replica routing
	Redis             RedisConfig       `yaml:"redis"`
	API               APIConfig         `yaml:"api"`
	Conversion        ConversionConfig  `yaml:"conversion"`
	DetectionStrategy DetectionStrategy `yaml:"detection_strategy"` // v2.0: Currency detection
	Backfill          BackfillConfig    `yaml:"backfill"`
	Simulation        SimulationConfig  `yaml:"simulation"`
	Monitoring        MonitoringConfig  `yaml:"monitoring"`
	Logging           LoggingConfig     `yaml:"logging"`
	Tables            TablesConfig      `yaml:"tables"`
}

type DatabaseConfig struct {
	Host              string        `yaml:"host"`
	Port              int           `yaml:"port"`
	Type              string        `yaml:"type"`
	User              string        `yaml:"user"`
	Password          string        `yaml:"password"`
	Database          string        `yaml:"database"`
	MaxConnections    int           `yaml:"max_connections"`
	IdleConnections   int           `yaml:"idle_connections"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout"`
}

type ProxyConfig struct {
	Host                  string        `yaml:"host"`
	Port                  int           `yaml:"port"`
	PoolSize              int           `yaml:"pool_size"`
	MaxConnectionsPerHost int           `yaml:"max_connections_per_host"`
	ReadTimeout           time.Duration `yaml:"read_timeout"`
	WriteTimeout          time.Duration `yaml:"write_timeout"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	Database int    `yaml:"database"`
	PoolSize int    `yaml:"pool_size"`
}

type APIConfig struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	APIKey string `yaml:"api_key"`
}

type ConversionConfig struct {
	Ratio            int    `yaml:"ratio"`
	Precision        int    `yaml:"precision"`
	RoundingStrategy string `yaml:"rounding_strategy"`
}

// DetectionStrategy configures currency detection (v2.0)
type DetectionStrategy struct {
	Method         string `yaml:"method"`          // AUTO, EXPLICIT, FIELD_NAME, VALUE_RANGE
	ExplicitField  string `yaml:"explicit_field"`  // Field name for explicit detection
	ThresholdValue int64  `yaml:"threshold_value"` // Value threshold for range detection
}

// TLSConfig configures TLS/SSL for secure connections (v2.0)
type TLSConfig struct {
	Client  TLSEndpointConfig `yaml:"client"`  // TLS for client-facing connections (app → proxy)
	Backend TLSEndpointConfig `yaml:"backend"` // TLS for backend connections (proxy → MySQL)
}

// TLSEndpointConfig configures TLS for a specific endpoint
type TLSEndpointConfig struct {
	Enabled    bool   `yaml:"enabled"`     // Enable TLS for this endpoint
	CertFile   string `yaml:"cert_file"`   // Path to certificate file
	KeyFile    string `yaml:"key_file"`    // Path to private key file
	CAFile     string `yaml:"ca_file"`     // Path to CA certificate
	ServerName string `yaml:"server_name"` // Expected server name for verification
	SkipVerify bool   `yaml:"skip_verify"` // Skip certificate verification (dev only!)
}

// ReplicaConfig configures read replica routing (v2.0)
type ReplicaConfig struct {
	Enabled  bool                    `yaml:"enabled"`  // Enable replica routing
	Strategy string                  `yaml:"strategy"` // ROUND_ROBIN, RANDOM, LEAST_CONNECTIONS
	Replicas []ReplicaDatabaseConfig `yaml:"replicas"` // List of replica databases
}

// ReplicaDatabaseConfig represents a replica database connection
type ReplicaDatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type BackfillConfig struct {
	Enabled         bool `yaml:"enabled"`
	BatchSize       int  `yaml:"batch_size"`
	SleepIntervalMs int  `yaml:"sleep_interval_ms"`
	MaxCPUPercent   int  `yaml:"max_cpu_percent"`
	RetryAttempts   int  `yaml:"retry_attempts"`
	RetryBackoffMs  int  `yaml:"retry_backoff_ms"`
}

type SimulationConfig struct {
	Enabled    bool     `yaml:"enabled"`
	AllowedIPs []string `yaml:"allowed_ips"`
}

type MonitoringConfig struct {
	PrometheusEnabled bool   `yaml:"prometheus_enabled"`
	PrometheusPort    int    `yaml:"prometheus_port"`
	MetricsPath       string `yaml:"metrics_path"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

type TablesConfig map[string]TableConfig

type TableConfig struct {
	Enabled bool                    `yaml:"enabled"`
	Columns map[string]ColumnConfig `yaml:"columns"`
}

type ColumnConfig struct {
	SourceColumn     string `yaml:"source_column"`
	TargetColumn     string `yaml:"target_column"`
	SourceType       string `yaml:"source_type"`
	TargetType       string `yaml:"target_type"`
	RoundingStrategy string `yaml:"rounding_strategy"`
	Precision        int    `yaml:"precision"`
}

// Load loads configuration from a YAML file
func Load(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.Port == 0 {
		return fmt.Errorf("database port is required")
	}
	if c.Proxy.Port == 0 {
		return fmt.Errorf("proxy port is required")
	}
	if c.Conversion.Ratio <= 0 {
		return fmt.Errorf("conversion ratio must be positive")
	}
	if c.Conversion.Precision < 0 || c.Conversion.Precision > 10 {
		return fmt.Errorf("conversion precision must be between 0 and 10")
	}

	// Validate rounding strategy
	validStrategies := map[string]bool{
		"BANKERS_ROUND":    true,
		"ARITHMETIC_ROUND": true,
	}
	if !validStrategies[c.Conversion.RoundingStrategy] {
		return fmt.Errorf("invalid rounding strategy: %s", c.Conversion.RoundingStrategy)
	}

	return nil
}

// GetDatabaseDSN returns the database connection string
func (c *Config) GetDatabaseDSN() string {
	switch c.Database.Type {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Asia%%2FJakarta",
			c.Database.User,
			c.Database.Password,
			c.Database.Host,
			c.Database.Port,
			c.Database.Database,
		)
	case "postgresql":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Jakarta",
			c.Database.Host,
			c.Database.Port,
			c.Database.User,
			c.Database.Password,
			c.Database.Database,
		)
	default:
		return ""
	}
}
