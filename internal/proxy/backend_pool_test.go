package proxy

import (
	"testing"
	"time"

	"github.com/kafitramarna/TransisiDB/internal/config"
)

func TestBackendPool_CreateAndAcquire(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:              "localhost",
			Port:              3307,
			ConnectionTimeout: 5 * time.Second,
		},
	}

	pool, err := NewBackendPool(cfg, 5)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Note: This test requires a running MySQL instance on localhost:3307
	// In a real scenario, you'd use a mock or test MySQL container
	t.Skip("Requires running MySQL instance")

	conn, err := pool.Acquire()
	if err != nil {
		t.Fatalf("Failed to acquire connection: %v", err)
	}

	if conn == nil {
		t.Fatal("Expected non-nil connection")
	}

	// Release connection back to pool
	pool.Release(conn)

	stats := pool.Stats()
	t.Logf("Pool stats: %+v", stats)
}

func TestBackendPool_ReleaseAndReuse(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:              "localhost",
			Port:              3307,
			ConnectionTimeout: 5 * time.Second,
		},
	}

	pool, err := NewBackendPool(cfg, 5)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	t.Skip("Requires running MySQL instance")

	// Acquire and release multiple connections
	for i := 0; i < 3; i++ {
		conn, err := pool.Acquire()
		if err != nil {
			t.Fatalf("Failed to acquire connection %d: %v", i, err)
		}
		pool.Release(conn)
	}

	stats := pool.Stats()
	if stats["total_created"].(uint64) > 3 {
		t.Errorf("Expected at most 3 connections created, got %d", stats["total_created"])
	}
}

func TestBackendConn_TransactionState(t *testing.T) {
	// Mock connection (nil is fine for this test)
	conn := NewBackendConn(nil, 1)

	if conn.IsInTransaction() {
		t.Error("Expected connection to not be in transaction initially")
	}

	conn.SetInTransaction(true)
	if !conn.IsInTransaction() {
		t.Error("Expected connection to be in transaction after setting")
	}

	conn.SetInTransaction(false)
	if conn.IsInTransaction() {
		t.Error("Expected connection to not be in transaction after clearing")
	}
}

func TestBackendConn_DatabaseTracking(t *testing.T) {
	conn := NewBackendConn(nil, 1)

	if db := conn.GetDatabase(); db != "" {
		t.Errorf("Expected empty database, got %s", db)
	}

	conn.SetDatabase("test_db")
	if db := conn.GetDatabase(); db != "test_db" {
		t.Errorf("Expected database 'test_db', got %s", db)
	}
}

func TestBackendConn_Reset(t *testing.T) {
	conn := NewBackendConn(nil, 1)

	conn.SetInTransaction(true)
	conn.SetDatabase("test_db")

	err := conn.Reset()
	if err != nil {
		t.Errorf("Reset failed: %v", err)
	}

	if conn.IsInTransaction() {
		t.Error("Expected transaction state to be reset")
	}

	if db := conn.GetDatabase(); db != "" {
		t.Errorf("Expected database to be reset, got %s", db)
	}
}
