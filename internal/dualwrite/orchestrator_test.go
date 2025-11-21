package dualwrite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock database for testing
type mockDB struct {
	queries []string
}

func (m *mockDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	m.queries = append(m.queries, query)
	return &mockResult{rowsAffected: 1}, nil
}

func (m *mockDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return nil, nil
}

type mockResult struct {
	rowsAffected int64
}

func (m *mockResult) LastInsertId() (int64, error) {
	return 1, nil
}

func (m *mockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

func getTestConfig() *config.Config {
	return &config.Config{
		Conversion: config.ConversionConfig{
			Ratio:            1000,
			Precision:        4,
			RoundingStrategy: "BANKERS_ROUND",
		},
		Tables: config.TablesConfig{
			"orders": {
				Enabled: true,
				Columns: map[string]config.ColumnConfig{
					"total_amount": {
						SourceColumn:     "total_amount",
						TargetColumn:     "total_amount_idn",
						SourceType:       "BIGINT",
						TargetType:       "DECIMAL(19,4)",
						RoundingStrategy: "BANKERS_ROUND",
						Precision:        4,
					},
					"shipping_fee": {
						SourceColumn:     "shipping_fee",
						TargetColumn:     "shipping_fee_idn",
						SourceType:       "INT",
						TargetType:       "DECIMAL(12,4)",
						RoundingStrategy: "BANKERS_ROUND",
						Precision:        4,
					},
				},
			},
		},
	}
}

func TestInterceptAndRewrite_Insert(t *testing.T) {
	cfg := getTestConfig()

	// Note: We can't easily mock sql.DB, so we'll test the public interface
	// For full integration tests, use testcontainers

	tests := []struct {
		name         string
		query        string
		wantContains []string
		wantErr      bool
	}{
		{
			name:  "INSERT with currency columns",
			query: "INSERT INTO orders (customer_id, total_amount, shipping_fee) VALUES (123, 500000, 25000)",
			wantContains: []string{
				"total_amount_idn",
				"shipping_fee_idn",
				"500.0000",
				"25.0000",
			},
			wantErr: false,
		},
		{
			name:  "INSERT without currency columns",
			query: "INSERT INTO orders (customer_id, status) VALUES (123, 'pending')",
			wantContains: []string{
				"customer_id",
				"status",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We'll test the rewrite logic without actual DB
			// Create orchestrator with nil db (won't execute, just rewrite)
			orch := &Orchestrator{
				db:        nil, // nil for rewrite-only testing
				parser:    nil, // will be set below
				converter: nil, // will be set below
				config:    cfg,
			}

			// Initialize components
			orch.parser = NewOrchestrator(nil, cfg).parser
			orch.converter = NewOrchestrator(nil, cfg).converter

			rewritten, err := orch.InterceptAndRewrite(tt.query)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, rewritten, want)
			}

			t.Logf("Original: %s", tt.query)
			t.Logf("Rewritten: %s", rewritten)
		})
	}
}

func TestInterceptAndRewrite_Update(t *testing.T) {
	cfg := getTestConfig()

	orch := &Orchestrator{
		db:        nil,
		parser:    nil,
		converter: nil,
		config:    cfg,
	}
	orch.parser = NewOrchestrator(nil, cfg).parser
	orch.converter = NewOrchestrator(nil, cfg).converter

	query := "UPDATE orders SET total_amount = 750000 WHERE id = 123"
	rewritten, err := orch.InterceptAndRewrite(query)

	require.NoError(t, err)
	assert.Contains(t, rewritten, "total_amount_idn")
	assert.Contains(t, rewritten, "750.0000")

	t.Logf("Original: %s", query)
	t.Logf("Rewritten: %s", rewritten)
}

func TestConvertCurrencyValues(t *testing.T) {
	cfg := getTestConfig()
	orch := NewOrchestrator(nil, cfg)

	// This is tested indirectly through InterceptAndRewrite
	// Direct testing would require accessing private method
	// We'll verify the results through integration

	query := "INSERT INTO orders (customer_id, total_amount) VALUES (123, 1234567)"
	rewritten, err := orch.InterceptAndRewrite(query)

	require.NoError(t, err)
	// 1234567 / 1000 = 1234.567 → rounds to 1234.5670 with Banker's Round
	assert.Contains(t, rewritten, "1234.5670")
}

func TestInterceptAndRewrite_NoTransform(t *testing.T) {
	cfg := getTestConfig()
	orch := NewOrchestrator(nil, cfg)

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "SELECT query",
			query: "SELECT * FROM orders WHERE id = 123",
		},
		{
			name:  "DELETE query",
			query: "DELETE FROM orders WHERE id = 123",
		},
		{
			name:  "INSERT to unconfigured table",
			query: "INSERT INTO users (name, email) VALUES ('John', 'john@test.com')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewritten, err := orch.InterceptAndRewrite(tt.query)

			require.NoError(t, err)
			// For queries that don't need transform, should return same query
			assert.Equal(t, tt.query, rewritten)
		})
	}
}
