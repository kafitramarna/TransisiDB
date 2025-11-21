package dualwrite

import (
	"fmt"
	"testing"

	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestConfigForConverter() *config.Config {
	return &config.Config{
		Conversion: config.ConversionConfig{
			Ratio:            1000,
			Precision:        4,
			RoundingStrategy: "BANKERS_ROUND",
		},
		DetectionStrategy: config.DetectionStrategy{
			Method:         "AUTO",
			ExplicitField:  "currency",
			ThresholdValue: 1000000,
		},
	}
}

func TestConverter_DetectDirection_IDR(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount"},
		Values: map[string]interface{}{
			"total_amount": 50000000, // Large value indicates IDR
		},
	}

	direction, err := converter.DetectDirection(pq)
	require.NoError(t, err)
	assert.Equal(t, DirectionIDRtoIDN, direction)
}

func TestConverter_DetectDirection_IDN(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount_idn"},
		Values: map[string]interface{}{
			"total_amount_idn": 50000.00, // Small value + _idn suffix indicates IDN
		},
	}

	direction, err := converter.DetectDirection(pq)
	require.NoError(t, err)
	assert.Equal(t, DirectionIDNtoIDR, direction)
}

func TestConverter_DetectDirection_Explicit(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount"},
		Values: map[string]interface{}{
			"total_amount": 50000000,
			"currency":     "IDR", // Explicit field
		},
	}

	direction, err := converter.DetectDirection(pq)
	require.NoError(t, err)
	assert.Equal(t, DirectionIDRtoIDN, direction)
}

func TestConverter_DetectDirection_None(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{}, // No currency columns
		Values: map[string]interface{}{
			"customer_id": 1001,
		},
	}

	direction, err := converter.DetectDirection(pq)
	require.NoError(t, err)
	assert.Equal(t, DirectionNone, direction)
}

func TestConverter_ConvertIDRtoIDN(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount", "shipping_fee"},
		Values: map[string]interface{}{
			"total_amount": 50000000, // 50 million IDR
			"shipping_fee": 15000,    // 15 thousand IDR
		},
	}

	converted, err := converter.convertIDRtoIDN(pq)
	require.NoError(t, err)

	// 50,000,000 / 1000 = 50,000.0000
	assert.Equal(t, 50000.0, converted["total_amount"])

	// 15,000 / 1000 = 15.0000
	assert.Equal(t, 15.0, converted["shipping_fee"])
}

func TestConverter_ConvertIDNtoIDR(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount_idn", "shipping_fee_idn"},
		Values: map[string]interface{}{
			"total_amount_idn": 50000.0, // 50 thousand IDN
			"shipping_fee_idn": 15.0,    // 15 IDN
		},
	}

	converted, err := converter.convertIDNtoIDR(pq)
	require.NoError(t, err)

	// 50,000.0 * 1000 = 50,000,000
	assert.Equal(t, 50000000.0, converted["total_amount_idn"])

	// 15.0 * 1000 = 15,000
	assert.Equal(t, 15000.0, converted["shipping_fee_idn"])
}

func TestConverter_ConvertValues_IDRtoIDN(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount"},
		Values: map[string]interface{}{
			"total_amount": 50000000,
		},
	}

	converted, err := converter.ConvertValues(pq, DirectionIDRtoIDN)
	require.NoError(t, err)
	assert.Equal(t, 50000.0, converted["total_amount"])
}

func TestConverter_ConvertValues_IDNtoIDR(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount_idn"},
		Values: map[string]interface{}{
			"total_amount_idn": 50000.0,
		},
	}

	converted, err := converter.ConvertValues(pq, DirectionIDNtoIDR)
	require.NoError(t, err)
	assert.Equal(t, 50000000.0, converted["total_amount_idn"])
}

func TestConverter_ConvertValues_None(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{},
		Values:          map[string]interface{}{},
	}

	converted, err := converter.ConvertValues(pq, DirectionNone)
	require.NoError(t, err)
	assert.Nil(t, converted)
}

func TestConverter_Rounding_BankersRound(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	testCases := []struct {
		name     string
		idnValue float64
		expected float64
	}{
		{"Exact conversion", 50000.0, 50000000.0},
		{"Half to even - 15.5", 15.5, 15000.0 + 500.0}, // 15.5 * 1000 = 15500.0
		{"Half to even - 16.5", 16.5, 16000.0 + 500.0}, // 16.5 * 1000 = 16500.0
		{"Decimal precision", 1234.5678, 1234567.8},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pq := &parser.ParsedQuery{
				CurrencyColumns: []string{"amount"},
				Values: map[string]interface{}{
					"amount": tc.idnValue,
				},
			}

			converted, err := converter.convertIDNtoIDR(pq)
			require.NoError(t, err)
			assert.InDelta(t, tc.expected, converted["amount"], 0.1)
		})
	}
}

func TestConverter_TypeConversion_Int(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount"},
		Values: map[string]interface{}{
			"total_amount": int(50000000),
		},
	}

	converted, err := converter.convertIDRtoIDN(pq)
	require.NoError(t, err)
	assert.Equal(t, 50000.0, converted["total_amount"])
}

func TestConverter_TypeConversion_Int64(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount"},
		Values: map[string]interface{}{
			"total_amount": int64(50000000),
		},
	}

	converted, err := converter.convertIDRtoIDN(pq)
	require.NoError(t, err)
	assert.Equal(t, 50000.0, converted["total_amount"])
}

func TestConverter_TypeConversion_String(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount"},
		Values: map[string]interface{}{
			"total_amount": "50000000",
		},
	}

	converted, err := converter.convertIDRtoIDN(pq)
	require.NoError(t, err)
	assert.Equal(t, 50000.0, converted["total_amount"])
}

func TestConverter_TypeConversion_InvalidString(t *testing.T) {
	cfg := getTestConfigForConverter()
	converter := NewConverter(cfg)

	pq := &parser.ParsedQuery{
		CurrencyColumns: []string{"total_amount"},
		Values: map[string]interface{}{
			"total_amount": "invalid",
		},
	}

	_, err := converter.convertIDRtoIDN(pq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert")
}

func TestToInt64_Conversions(t *testing.T) {
	testCases := []struct {
		name     string
		value    interface{}
		expected int64
		hasError bool
	}{
		{"int", 123, 123, false},
		{"int32", int32(456), 456, false},
		{"int64", int64(789), 789, false},
		{"float32", float32(100.5), 100, false},
		{"float64", 200.9, 200, false},
		{"string valid", "999", 999, false},
		{"string invalid", "abc", 0, true},
		{"unsupported type", true, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := toInt64(tc.value)
			if tc.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestToFloat64_Conversions(t *testing.T) {
	testCases := []struct {
		name     string
		value    interface{}
		expected float64
		hasError bool
	}{
		{"int", 123, 123.0, false},
		{"int32", int32(456), 456.0, false},
		{"int64", int64(789), 789.0, false},
		{"float32", float32(100.5), 100.5, false},
		{"float64", 200.9, 200.9, false},
		{"string valid", "123.456", 123.456, false},
		{"string invalid", "abc", 0.0, true},
		{"unsupported type", true, 0.0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := toFloat64(tc.value)
			if tc.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tc.expected, result, 0.001)
			}
		})
	}
}

// Test with different ratios
func TestConverter_DifferentRatios(t *testing.T) {
	testRatios := []int{100, 1000, 10000}

	for _, ratio := range testRatios {
		t.Run(fmt.Sprintf("Ratio_%d", ratio), func(t *testing.T) {
			cfg := getTestConfigForConverter()
			cfg.Conversion.Ratio = ratio
			converter := NewConverter(cfg)

			pq := &parser.ParsedQuery{
				CurrencyColumns: []string{"amount"},
				Values: map[string]interface{}{
					"amount": int64(50000000),
				},
			}

			// IDR → IDN
			converted, err := converter.convertIDRtoIDN(pq)
			require.NoError(t, err)
			expected := 50000000.0 / float64(ratio)
			assert.Equal(t, expected, converted["amount"])

			// IDN → IDR (reverse)
			pqReverse := &parser.ParsedQuery{
				CurrencyColumns: []string{"amount"},
				Values: map[string]interface{}{
					"amount": expected,
				},
			}
			convertedBack, err := converter.convertIDNtoIDR(pqReverse)
			require.NoError(t, err)
			assert.InDelta(t, 50000000.0, convertedBack["amount"], 1.0)
		})
	}
}
