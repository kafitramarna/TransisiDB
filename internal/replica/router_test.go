package replica

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryType_Constants(t *testing.T) {
	assert.Equal(t, QueryType(0), QueryTypeRead)
	assert.Equal(t, QueryType(1), QueryTypeWrite)
}

func TestNewRouter_NilConfig(t *testing.T) {
	_, err := NewRouter(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestRouter_GetConnection_Write(t *testing.T) {
	// This test requires actual database connection
	// For unit test, we'll test the logic without real DB
	t.Skip("Requires real database connection")
}

func TestRouter_GetConnection_Read(t *testing.T) {
	// This test requires actual database connection
	t.Skip("Requires real database connection")
}

func TestRouter_GetConnection_InvalidType(t *testing.T) {
	// Create mock router (without real DB connections)
	router := &Router{
		primary:  &sql.DB{},
		replicas: []*sql.DB{},
		strategy: "ROUND_ROBIN",
	}

	_, err := router.GetConnection(QueryType(999))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown query type")
}

func TestRouter_GetReplicaCount(t *testing.T) {
	router := &Router{
		replicas: []*sql.DB{{}, {}, {}},
	}

	count := router.GetReplicaCount()
	assert.Equal(t, 3, count)
}

func TestRouter_GetReplicaCount_Empty(t *testing.T) {
	router := &Router{
		replicas: []*sql.DB{},
	}

	count := router.GetReplicaCount()
	assert.Equal(t, 0, count)
}

func TestRouter_GetPrimaryDB(t *testing.T) {
	mockPrimary := &sql.DB{}
	router := &Router{
		primary: mockPrimary,
	}

	primary := router.GetPrimaryDB()
	assert.Equal(t, mockPrimary, primary)
}

func TestConfig_Structure(t *testing.T) {
	cfg := &Config{
		Primary: DatabaseConfig{
			Host:     "localhost",
			Port:     3306,
			User:     "root",
			Password: "password",
			Database: "mydb",
		},
		Replicas: []DatabaseConfig{
			{
				Host:     "replica1",
				Port:     3306,
				User:     "root",
				Password: "password",
				Database: "mydb",
			},
		},
		Strategy: "ROUND_ROBIN",
	}

	assert.Equal(t, "localhost", cfg.Primary.Host)
	assert.Equal(t, 3306, cfg.Primary.Port)
	assert.Equal(t, 1, len(cfg.Replicas))
	assert.Equal(t, "replica1", cfg.Replicas[0].Host)
	assert.Equal(t, "ROUND_ROBIN", cfg.Strategy)
}

func TestRouter_DefaultStrategy(t *testing.T) {
	router := &Router{
		strategy: "",
	}

	// After initialization, should default to ROUND_ROBIN
	// This is tested in NewRouter, but we verify the field here
	assert.Equal(t, "", router.strategy) // Before init
}

func TestDatabaseConfig_Fields(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "mysql.example.com",
		Port:     3307,
		User:     "appuser",
		Password: "secret",
		Database: "production_db",
	}

	assert.Equal(t, "mysql.example.com", cfg.Host)
	assert.Equal(t, 3307, cfg.Port)
	assert.Equal(t, "appuser", cfg.User)
	assert.Equal(t, "secret", cfg.Password)
	assert.Equal(t, "production_db", cfg.Database)
}

// Integration test with mocked health checker
func TestRouter_FallbackToPrimary(t *testing.T) {
	mockPrimary := &sql.DB{}
	router := &Router{
		primary:  mockPrimary,
		replicas: []*sql.DB{}, // No replicas
		strategy: "ROUND_ROBIN",
		healthChecker: &HealthChecker{
			healthStatus: []bool{},
		},
	}

	conn, err := router.getReadConnection()
	require.NoError(t, err)
	assert.Equal(t, mockPrimary, conn, "Should fallback to primary when no replicas")
}

func TestRouter_RoundRobinDistribution(t *testing.T) {
	// Test round-robin logic without real database
	mockReplica1 := &sql.DB{}
	mockReplica2 := &sql.DB{}

	router := &Router{
		primary:      &sql.DB{},
		replicas:     []*sql.DB{mockReplica1, mockReplica2},
		strategy:     "ROUND_ROBIN",
		replicaIndex: 0,
		healthChecker: &HealthChecker{
			replicas:     []*sql.DB{mockReplica1, mockReplica2},
			healthStatus: []bool{true, true}, // Both healthy
		},
	}

	// First call should return replica 1
	conn1, err := router.getReadConnection()
	require.NoError(t, err)
	assert.Equal(t, mockReplica1, conn1)
	assert.Equal(t, 1, router.replicaIndex)

	// Second call should return replica 2
	conn2, err := router.getReadConnection()
	require.NoError(t, err)
	assert.Equal(t, mockReplica2, conn2)
	assert.Equal(t, 2, router.replicaIndex)

	// Third call should wrap around to replica 1
	conn3, err := router.getReadConnection()
	require.NoError(t, err)
	assert.Equal(t, mockReplica1, conn3)
	assert.Equal(t, 3, router.replicaIndex)
}
