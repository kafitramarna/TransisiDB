package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// Config holds TLS configuration for client or backend connections
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	CertFile   string `yaml:"cert_file"`   // Path to certificate file
	KeyFile    string `yaml:"key_file"`    // Path to private key file
	CAFile     string `yaml:"ca_file"`     // Path to CA certificate
	ServerName string `yaml:"server_name"` // Expected server name (for verification)
	SkipVerify bool   `yaml:"skip_verify"` // Skip certificate verification (dev only!)
}

// Manager handles TLS certificates and configuration
type Manager struct {
	clientConfig  *tls.Config // TLS config for client-facing connections
	backendConfig *tls.Config // TLS config for backend MySQL connections
}

// NewManager creates a new TLS manager
func NewManager(clientCfg, backendCfg *Config) (*Manager, error) {
	manager := &Manager{}

	// Initialize client TLS config if enabled
	if clientCfg != nil && clientCfg.Enabled {
		config, err := createTLSConfig(clientCfg, true)
		if err != nil {
			return nil, fmt.Errorf("failed to create client TLS config: %w", err)
		}
		manager.clientConfig = config
	}

	// Initialize backend TLS config if enabled
	if backendCfg != nil && backendCfg.Enabled {
		config, err := createTLSConfig(backendCfg, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create backend TLS config: %w", err)
		}
		manager.backendConfig = config
	}

	return manager, nil
}

// createTLSConfig builds a tls.Config from our Config structure
func createTLSConfig(cfg *Config, isServer bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		ServerName:         cfg.ServerName,
		InsecureSkipVerify: cfg.SkipVerify,
		MinVersion:         tls.VersionTLS12, // Enforce TLS 1.2 minimum
	}

	// For server mode (client-facing), we need server certificates
	if isServer {
		if cfg.CertFile == "" || cfg.KeyFile == "" {
			return nil, fmt.Errorf("cert_file and key_file are required for server TLS")
		}

		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load server certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}

		// If CA file provided, use it for client cert verification
		if cfg.CAFile != "" {
			caCert, err := os.ReadFile(cfg.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA file: %w", err)
			}

			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}
			tlsConfig.ClientCAs = caCertPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
	} else {
		// For client mode (backend MySQL), we need client certificates
		if cfg.CertFile != "" && cfg.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		// If CA file provided, use it to verify server
		if cfg.CAFile != "" {
			caCert, err := os.ReadFile(cfg.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA file: %w", err)
			}

			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}
			tlsConfig.RootCAs = caCertPool
		}
	}

	return tlsConfig, nil
}

// GetClientConfig returns TLS config for client-facing connections
func (m *Manager) GetClientConfig() *tls.Config {
	return m.clientConfig
}

// GetBackendConfig returns TLS config for backend MySQL connections
func (m *Manager) GetBackendConfig() *tls.Config {
	return m.backendConfig
}

// IsClientTLSEnabled returns true if client TLS is configured
func (m *Manager) IsClientTLSEnabled() bool {
	return m.clientConfig != nil
}

// IsBackendTLSEnabled returns true if backend TLS is configured
func (m *Manager) IsBackendTLSEnabled() bool {
	return m.backendConfig != nil
}

// ValidateCertificates validates that certificates are readable and valid
func ValidateCertificates(cfg *Config) error {
	// Check certificate file exists
	if cfg.CertFile != "" {
		if _, err := os.Stat(cfg.CertFile); err != nil {
			return fmt.Errorf("certificate file not found: %w", err)
		}
	}

	// Check key file exists
	if cfg.KeyFile != "" {
		if _, err := os.Stat(cfg.KeyFile); err != nil {
			return fmt.Errorf("key file not found: %w", err)
		}
	}

	// Check CA file exists
	if cfg.CAFile != "" {
		if _, err := os.Stat(cfg.CAFile); err != nil {
			return fmt.Errorf("CA file not found: %w", err)
		}
	}

	// Try loading the certificate pair
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		if _, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile); err != nil {
			return fmt.Errorf("failed to load certificate pair: %w", err)
		}
	}

	return nil
}
