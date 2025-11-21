package dualwrite

import (
	"fmt"

	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/detector"
	"github.com/kafitramarna/TransisiDB/internal/parser"
	"github.com/kafitramarna/TransisiDB/internal/rounding"
)

// Direction represents the conversion direction
type Direction int

const (
	DirectionIDRtoIDN Direction = iota // IDR → IDN (existing v1.0 behavior)
	DirectionIDNtoIDR                  // IDN → IDR (new v2.0 reverse)
	DirectionNone                      // No conversion needed
)

// Converter handles bidirectional currency conversion
type Converter struct {
	roundingEngine *rounding.Engine
	detector       *detector.CurrencyDetector
	config         *config.Config
}

// NewConverter creates a new bidirectional converter
func NewConverter(cfg *config.Config) *Converter {
	// Create detector based on configuration
	detectorCfg := &detector.Config{
		Method:         detector.DetectionAuto, // Default to auto
		ThresholdValue: 1000000,                // 1 million threshold
		CurrencyField:  "currency",             // Default currency field name
	}

	// Override with config if available
	if cfg.DetectionStrategy.Method != "" {
		detectorCfg.Method = detector.DetectionMethod(cfg.DetectionStrategy.Method)
	}
	if cfg.DetectionStrategy.ThresholdValue > 0 {
		detectorCfg.ThresholdValue = cfg.DetectionStrategy.ThresholdValue
	}
	if cfg.DetectionStrategy.ExplicitField != "" {
		detectorCfg.CurrencyField = cfg.DetectionStrategy.ExplicitField
	}

	return &Converter{
		roundingEngine: rounding.NewEngine(
			rounding.Strategy(cfg.Conversion.RoundingStrategy),
			cfg.Conversion.Precision,
		),
		detector: detector.NewDetector(detectorCfg),
		config:   cfg,
	}
}

// DetectDirection analyzes the parsed query to determine conversion direction
func (c *Converter) DetectDirection(pq *parser.ParsedQuery) (Direction, error) {
	// If no currency columns, no conversion needed
	if len(pq.CurrencyColumns) == 0 {
		return DirectionNone, nil
	}

	// Use detector to determine currency type
	result, err := c.detector.Detect(pq.Values)
	if err != nil {
		return DirectionNone, fmt.Errorf("failed to detect currency: %w", err)
	}

	// Log ambiguity warning if present
	if result.AmbiguityWarning {
		// TODO: Add proper logger
		// logger.Warn("Currency detection ambiguous", "confidence", result.Confidence)
	}

	// Determine direction based on detected currency
	switch result.Currency {
	case detector.CurrencyIDR:
		return DirectionIDRtoIDN, nil
	case detector.CurrencyIDN:
		return DirectionIDNtoIDR, nil
	default:
		return DirectionNone, fmt.Errorf("unknown currency type: %s", result.Currency)
	}
}

// ConvertValues converts currency values based on direction
func (c *Converter) ConvertValues(pq *parser.ParsedQuery, direction Direction) (map[string]float64, error) {
	if direction == DirectionNone {
		return nil, nil
	}

	switch direction {
	case DirectionIDRtoIDN:
		return c.convertIDRtoIDN(pq)
	case DirectionIDNtoIDR:
		return c.convertIDNtoIDR(pq)
	default:
		return nil, fmt.Errorf("unsupported conversion direction: %d", direction)
	}
}

// convertIDRtoIDN converts IDR values to IDN (existing v1.0 logic)
func (c *Converter) convertIDRtoIDN(pq *parser.ParsedQuery) (map[string]float64, error) {
	converted := make(map[string]float64)

	for _, colName := range pq.CurrencyColumns {
		value, exists := pq.Values[colName]
		if !exists {
			continue
		}

		// Convert to int64
		intValue, err := toInt64(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for column %s: %w", colName, err)
		}

		// Convert IDR → IDN (divide by ratio)
		convertedValue := c.roundingEngine.ConvertIDRtoIDN(intValue, c.config.Conversion.Ratio)
		converted[colName] = convertedValue
	}

	return converted, nil
}

// convertIDNtoIDR converts IDN values to IDR (new v2.0 reverse logic)
func (c *Converter) convertIDNtoIDR(pq *parser.ParsedQuery) (map[string]float64, error) {
	converted := make(map[string]float64)

	for _, colName := range pq.CurrencyColumns {
		value, exists := pq.Values[colName]
		if !exists {
			continue
		}

		// Convert to float64
		floatValue, err := toFloat64(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for column %s: %w", colName, err)
		}

		// Convert IDN → IDR (multiply by ratio)
		// Apply rounding to ensure integer result
		idrValue := floatValue * float64(c.config.Conversion.Ratio)
		converted[colName] = c.roundingEngine.Round(idrValue)
	}

	return converted, nil
}

// toInt64 converts various types to int64
func toInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		// Try parsing string as integer
		var i int64
		_, err := fmt.Sscanf(v, "%d", &i)
		return i, err
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", value)
	}
}

// toFloat64 converts various types to float64
func toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		// Try parsing string as float
		var f float64
		_, err := fmt.Sscanf(v, "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}
