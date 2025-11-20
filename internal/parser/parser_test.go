package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/transisidb/transisidb/internal/config"
)

func getTestConfig() config.TablesConfig {
	return config.TablesConfig{
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
		"invoices": {
			Enabled: true,
			Columns: map[string]config.ColumnConfig{
				"grand_total": {
					SourceColumn:     "grand_total",
					TargetColumn:     "grand_total_idn",
					SourceType:       "BIGINT",
					TargetType:       "DECIMAL(19,4)",
					RoundingStrategy: "BANKERS_ROUND",
					Precision:        4,
				},
			},
		},
	}
}

func TestParseInsert(t *testing.T) {
	parser := NewParser(getTestConfig())

	tests := []struct {
		name               string
		query              string
		wantType           QueryType
		wantTable          string
		wantCurrencyCols   []string
		wantNeedsTransform bool
		wantErr            bool
	}{
		{
			name:               "Simple INSERT with currency columns",
			query:              "INSERT INTO orders (customer_id, total_amount, shipping_fee) VALUES (123, 500000, 25000)",
			wantType:           QueryTypeInsert,
			wantTable:          "orders",
			wantCurrencyCols:   []string{"total_amount", "shipping_fee"},
			wantNeedsTransform: true,
			wantErr:            false,
		},
		{
			name:               "INSERT without currency columns",
			query:              "INSERT INTO orders (customer_id, status) VALUES (123, 'pending')",
			wantType:           QueryTypeInsert,
			wantTable:          "orders",
			wantCurrencyCols:   []string{},
			wantNeedsTransform: false,
			wantErr:            false,
		},
		{
			name:               "INSERT to unconfigured table",
			query:              "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')",
			wantType:           QueryTypeInsert,
			wantTable:          "users",
			wantCurrencyCols:   []string{},
			wantNeedsTransform: false,
			wantErr:            false,
		},
		{
			name:               "INSERT with partial currency columns",
			query:              "INSERT INTO orders (customer_id, total_amount, status) VALUES (456, 1000000, 'completed')",
			wantType:           QueryTypeInsert,
			wantTable:          "orders",
			wantCurrencyCols:   []string{"total_amount"},
			wantNeedsTransform: true,
			wantErr:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pq, err := parser.Parse(tt.query)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantType, pq.Type)
			assert.Equal(t, tt.wantTable, pq.TableName)
			assert.Equal(t, tt.wantNeedsTransform, pq.NeedsTransform)

			if len(tt.wantCurrencyCols) > 0 {
				assert.ElementsMatch(t, tt.wantCurrencyCols, pq.CurrencyColumns)
			}
		})
	}
}

func TestParseUpdate(t *testing.T) {
	parser := NewParser(getTestConfig())

	tests := []struct {
		name               string
		query              string
		wantType           QueryType
		wantTable          string
		wantCurrencyCols   []string
		wantNeedsTransform bool
		wantErr            bool
	}{
		{
			name:               "Simple UPDATE with currency column",
			query:              "UPDATE orders SET total_amount = 750000 WHERE id = 123",
			wantType:           QueryTypeUpdate,
			wantTable:          "orders",
			wantCurrencyCols:   []string{"total_amount"},
			wantNeedsTransform: true,
			wantErr:            false,
		},
		{
			name:               "UPDATE without currency columns",
			query:              "UPDATE orders SET status = 'shipped' WHERE id = 123",
			wantType:           QueryTypeUpdate,
			wantTable:          "orders",
			wantCurrencyCols:   []string{},
			wantNeedsTransform: false,
			wantErr:            false,
		},
		{
			name:               "UPDATE multiple columns including currency",
			query:              "UPDATE orders SET total_amount = 900000, shipping_fee = 30000, status = 'completed' WHERE id = 456",
			wantType:           QueryTypeUpdate,
			wantTable:          "orders",
			wantCurrencyCols:   []string{"total_amount", "shipping_fee"},
			wantNeedsTransform: true,
			wantErr:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pq, err := parser.Parse(tt.query)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantType, pq.Type)
			assert.Equal(t, tt.wantTable, pq.TableName)
			assert.Equal(t, tt.wantNeedsTransform, pq.NeedsTransform)

			if len(tt.wantCurrencyCols) > 0 {
				assert.ElementsMatch(t, tt.wantCurrencyCols, pq.CurrencyColumns)
			}
		})
	}
}

func TestParseSelect(t *testing.T) {
	parser := NewParser(getTestConfig())

	tests := []struct {
		name      string
		query     string
		wantType  QueryType
		wantTable string
		wantErr   bool
	}{
		{
			name:      "Simple SELECT",
			query:     "SELECT * FROM orders WHERE id = 123",
			wantType:  QueryTypeSelect,
			wantTable: "orders",
			wantErr:   false,
		},
		{
			name:      "SELECT with specific columns",
			query:     "SELECT customer_id, total_amount FROM orders",
			wantType:  QueryTypeSelect,
			wantTable: "orders",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pq, err := parser.Parse(tt.query)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantType, pq.Type)
			assert.Equal(t, tt.wantTable, pq.TableName)
			// SELECT queries don't need transformation by default
			assert.False(t, pq.NeedsTransform)
		})
	}
}

func TestParseDelete(t *testing.T) {
	parser := NewParser(getTestConfig())

	query := "DELETE FROM orders WHERE id = 123"
	pq, err := parser.Parse(query)

	require.NoError(t, err)
	assert.Equal(t, QueryTypeDelete, pq.Type)
	assert.Equal(t, "orders", pq.TableName)
	assert.False(t, pq.NeedsTransform)
}

func TestRewriteInsert(t *testing.T) {
	parser := NewParser(getTestConfig())

	query := "INSERT INTO orders (customer_id, total_amount, shipping_fee) VALUES (123, 500000, 25000)"

	pq, err := parser.Parse(query)
	require.NoError(t, err)
	require.True(t, pq.NeedsTransform)

	// Simulated converted values
	convertedValues := map[string]float64{
		"total_amount": 500.0000,
		"shipping_fee": 25.0000,
	}

	rewritten, err := parser.RewriteForDualWrite(pq, convertedValues)
	require.NoError(t, err)

	// Verify the rewritten query contains shadow columns
	assert.Contains(t, rewritten, "total_amount_idn")
	assert.Contains(t, rewritten, "shipping_fee_idn")
	assert.Contains(t, rewritten, "500.0000")
	assert.Contains(t, rewritten, "25.0000")

	t.Logf("Original: %s", query)
	t.Logf("Rewritten: %s", rewritten)
}

func TestRewriteUpdate(t *testing.T) {
	parser := NewParser(getTestConfig())

	query := "UPDATE orders SET total_amount = 750000 WHERE id = 123"

	pq, err := parser.Parse(query)
	require.NoError(t, err)
	require.True(t, pq.NeedsTransform)

	convertedValues := map[string]float64{
		"total_amount": 750.0000,
	}

	rewritten, err := parser.RewriteForDualWrite(pq, convertedValues)
	require.NoError(t, err)

	assert.Contains(t, rewritten, "total_amount_idn")
	assert.Contains(t, rewritten, "750.0000")

	t.Logf("Original: %s", query)
	t.Logf("Rewritten: %s", rewritten)
}

func TestQueryTypeString(t *testing.T) {
	tests := []struct {
		queryType QueryType
		want      string
	}{
		{QueryTypeSelect, "SELECT"},
		{QueryTypeInsert, "INSERT"},
		{QueryTypeUpdate, "UPDATE"},
		{QueryTypeDelete, "DELETE"},
		{QueryTypeUnknown, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.queryType.String())
		})
	}
}

func TestQueryTypeIsMutation(t *testing.T) {
	assert.False(t, QueryTypeSelect.IsMutation())
	assert.True(t, QueryTypeInsert.IsMutation())
	assert.True(t, QueryTypeUpdate.IsMutation())
	assert.True(t, QueryTypeDelete.IsMutation())
	assert.False(t, QueryTypeUnknown.IsMutation())
}

func TestNormalizeTableName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"orders", "orders"},
		{"`orders`", "orders"},
		{"\"orders\"", "orders"},
		{"`my_table`", "my_table"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeTableName(tt.input))
		})
	}
}

func TestParseInvalidSQL(t *testing.T) {
	parser := NewParser(getTestConfig())

	invalidQueries := []string{
		"THIS IS NOT SQL",
		"SELECT * FROM",
		"INSERT INTO",
		"UPDATE SET status = 'test'",
	}

	for _, query := range invalidQueries {
		t.Run(query, func(t *testing.T) {
			_, err := parser.Parse(query)
			assert.Error(t, err)
		})
	}
}
