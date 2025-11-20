package simulation

import (
	"database/sql"
	"fmt"

	"github.com/transisidb/transisidb/internal/config"
	"github.com/transisidb/transisidb/internal/rounding"
)

// Simulator handles simulation mode for testing
type Simulator struct {
	config         *config.Config
	roundingEngine *rounding.Engine
}

// NewSimulator creates a new simulator
func NewSimulator(cfg *config.Config) *Simulator {
	return &Simulator{
		config: cfg,
		roundingEngine: rounding.NewEngine(
			rounding.Strategy(cfg.Conversion.RoundingStrategy),
			cfg.Conversion.Precision,
		),
	}
}

// TransformResponse transforms database response to simulation format
func (s *Simulator) TransformResponse(rows *sql.Rows, tableName string) (*SimulatedResponse, error) {
	tableConfig, exists := s.config.Tables[tableName]
	if !exists {
		return nil, fmt.Errorf("table not configured: %s", tableName)
	}

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var data []map[string]interface{}

	for rows.Next() {
		// Create slice for values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Build row map with transformation
		row := make(map[string]interface{})
		for i, col := range columns {
			// Check if this is a currency column
			if _, isCurrency := tableConfig.Columns[col]; isCurrency {
				// Transform to IDN
				if intVal, ok := values[i].(int64); ok {
					converted := s.roundingEngine.ConvertIDRtoIDN(intVal, s.config.Conversion.Ratio)
					row[col] = converted
				} else {
					row[col] = values[i]
				}
			} else {
				row[col] = values[i]
			}
		}

		data = append(data, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return &SimulatedResponse{
		Data: data,
		Metadata: ResponseMetadata{
			Simulated: true,
			Currency:  "IDN",
			Ratio:     s.config.Conversion.Ratio,
		},
	}, nil
}

// ShouldSimulate checks if simulation mode is enabled for this request
func (s *Simulator) ShouldSimulate(simulateHeader string, clientIP string) bool {
	if !s.config.Simulation.Enabled {
		return false
	}

	if simulateHeader != "SIMULATE_IDN" {
		return false
	}

	// Check IP whitelist
	if len(s.config.Simulation.AllowedIPs) > 0 {
		allowed := false
		for _, ip := range s.config.Simulation.AllowedIPs {
			// Simple IP matching (in production, use proper CIDR matching)
			if clientIP == ip || ip == "0.0.0.0/0" {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	return true
}

// SimulatedResponse represents a simulated API response
type SimulatedResponse struct {
	Data     []map[string]interface{} `json:"data"`
	Metadata ResponseMetadata         `json:"_metadata"`
}

// ResponseMetadata contains simulation metadata
type ResponseMetadata struct {
	Simulated bool   `json:"simulated"`
	Currency  string `json:"currency"`
	Ratio     int    `json:"conversion_ratio"`
}
