package tls

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create temporary test certificates
func createTestCerts(t *testing.T) (certFile, keyFile, caFile string, cleanup func()) {
	tmpDir := t.TempDir()

	certFile = filepath.Join(tmpDir, "server.crt")
	keyFile = filepath.Join(tmpDir, "server.key")
	caFile = filepath.Join(tmpDir, "ca.crt")

	// Create dummy cert files (these are not real certs, just for file existence tests)
	// For real TLS tests, you'd need proper certificates
	require.NoError(t, os.WriteFile(certFile, []byte("DUMMY CERT"), 0644))
	require.NoError(t, os.WriteFile(keyFile, []byte("DUMMY KEY"), 0644))
	require.NoError(t, os.WriteFile(caFile, []byte("DUMMY CA"), 0644))

	cleanup = func() {
		// TempDir auto-cleans
	}

	return
}

func TestNewManager_Disabled(t *testing.T) {
	clientCfg := &Config{Enabled: false}
	backendCfg := &Config{Enabled: false}

	manager, err := NewManager(clientCfg, backendCfg)
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Nil(t, manager.clientConfig)
	assert.Nil(t, manager.backendConfig)
	assert.False(t, manager.IsClientTLSEnabled())
	assert.False(t, manager.IsBackendTLSEnabled())
}

func TestNewManager_NilConfigs(t *testing.T) {
	manager, err := NewManager(nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.False(t, manager.IsClientTLSEnabled())
	assert.False(t, manager.IsBackendTLSEnabled())
}

func TestValidateCertificates_FilesExist(t *testing.T) {
	certFile, keyFile, caFile, cleanup := createTestCerts(t)
	defer cleanup()

	cfg := &Config{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   caFile,
	}

	// This will fail because files are not real certificates
	// But it should pass the file existence check
	err := ValidateCertificates(cfg)
	// Expect error because dummy files aren't valid certs
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load certificate pair")
}

func TestValidateCertificates_MissingFiles(t *testing.T) {
	cfg := &Config{
		Enabled:  true,
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}

	err := ValidateCertificates(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "certificate file not found")
}

func TestValidateCertificates_MissingKey(t *testing.T) {
	certFile, _, _, cleanup := createTestCerts(t)
	defer cleanup()

	cfg := &Config{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  "/nonexistent/key.pem",
	}

	err := ValidateCertificates(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key file not found")
}

func TestValidateCertificates_MissingCA(t *testing.T) {
	certFile, keyFile, _, cleanup := createTestCerts(t)
	defer cleanup()

	cfg := &Config{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   "/nonexistent/ca.pem",
	}

	err := ValidateCertificates(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CA file not found")
}

func TestCreateTLSConfig_MinVersion(t *testing.T) {
	cfg := &Config{
		Enabled:    true,
		SkipVerify: true, // Skip verify for test
	}

	tlsConfig, err := createTLSConfig(cfg, false)
	require.NoError(t, err)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
}

func TestCreateTLSConfig_ServerName(t *testing.T) {
	cfg := &Config{
		Enabled:    true,
		ServerName: "mysql.example.com",
		SkipVerify: true,
	}

	tlsConfig, err := createTLSConfig(cfg, false)
	require.NoError(t, err)
	assert.Equal(t, "mysql.example.com", tlsConfig.ServerName)
}

func TestCreateTLSConfig_SkipVerify(t *testing.T) {
	testCases := []struct {
		name       string
		skipVerify bool
	}{
		{"Skip verification enabled", true},
		{"Skip verification disabled", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Enabled:    true,
				SkipVerify: tc.skipVerify,
			}

			tlsConfig, err := createTLSConfig(cfg, false)
			require.NoError(t, err)
			assert.Equal(t, tc.skipVerify, tlsConfig.InsecureSkipVerify)
		})
	}
}

func TestCreateTLSConfig_ServerMode_MissingCerts(t *testing.T) {
	cfg := &Config{
		Enabled: true,
		// Missing CertFile and KeyFile
	}

	_, err := createTLSConfig(cfg, true) // isServer = true
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cert_file and key_file are required")
}

func TestConfig_Structure(t *testing.T) {
	cfg := &Config{
		Enabled:    true,
		CertFile:   "/path/to/cert.pem",
		KeyFile:    "/path/to/key.pem",
		CAFile:     "/path/to/ca.pem",
		ServerName: "example.com",
		SkipVerify: false,
	}

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "/path/to/cert.pem", cfg.CertFile)
	assert.Equal(t, "/path/to/key.pem", cfg.KeyFile)
	assert.Equal(t, "/path/to/ca.pem", cfg.CAFile)
	assert.Equal(t, "example.com", cfg.ServerName)
	assert.False(t, cfg.SkipVerify)
}

func TestManager_GetConfigs(t *testing.T) {
	manager := &Manager{
		clientConfig:  &tls.Config{ServerName: "client"},
		backendConfig: &tls.Config{ServerName: "backend"},
	}

	assert.Equal(t, "client", manager.GetClientConfig().ServerName)
	assert.Equal(t, "backend", manager.GetBackendConfig().ServerName)
}

func TestManager_NilConfigs(t *testing.T) {
	manager := &Manager{}

	assert.Nil(t, manager.GetClientConfig())
	assert.Nil(t, manager.GetBackendConfig())
	assert.False(t, manager.IsClientTLSEnabled())
	assert.False(t, manager.IsBackendTLSEnabled())
}
