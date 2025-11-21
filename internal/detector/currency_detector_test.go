package detector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetector_ExplicitDetection_IDR(t *testing.T) {
	detector := NewDetector(&Config{
		Method:        DetectionExplicit,
		CurrencyField: "currency",
	})

	columns := map[string]interface{}{
		"total_amount": 50000000,
		"currency":     "IDR",
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDR, result.Currency)
	assert.Equal(t, 1.0, result.Confidence)
	assert.Equal(t, "EXPLICIT", result.DetectedBy)
	assert.False(t, result.AmbiguityWarning)
}

func TestDetector_ExplicitDetection_IDN(t *testing.T) {
	detector := NewDetector(&Config{
		Method:        DetectionExplicit,
		CurrencyField: "currency",
	})

	columns := map[string]interface{}{
		"total_amount_idn": 50000.00,
		"currency":         "IDN",
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDN, result.Currency)
	assert.Equal(t, 1.0, result.Confidence)
	assert.Equal(t, "EXPLICIT", result.DetectedBy)
}

func TestDetector_ExplicitDetection_CaseInsensitive(t *testing.T) {
	detector := NewDetector(&Config{
		Method:        DetectionExplicit,
		CurrencyField: "currency",
	})

	testCases := []string{"idr", "IDR", "Idr", "  idr  "}

	for _, currencyValue := range testCases {
		columns := map[string]interface{}{
			"total_amount": 50000000,
			"currency":     currencyValue,
		}

		result, err := detector.Detect(columns)
		require.NoError(t, err, "Failed for currency value: %s", currencyValue)
		assert.Equal(t, CurrencyIDR, result.Currency)
	}
}

func TestDetector_ExplicitDetection_MissingField(t *testing.T) {
	detector := NewDetector(&Config{
		Method:        DetectionExplicit,
		CurrencyField: "currency",
	})

	columns := map[string]interface{}{
		"total_amount": 50000000,
		// Missing currency field
	}

	_, err := detector.Detect(columns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDetector_ExplicitDetection_InvalidCurrency(t *testing.T) {
	detector := NewDetector(&Config{
		Method:        DetectionExplicit,
		CurrencyField: "currency",
	})

	columns := map[string]interface{}{
		"total_amount": 50000000,
		"currency":     "USD", // Invalid currency
	}

	_, err := detector.Detect(columns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid currency")
}

func TestDetector_FieldNameDetection_IDR(t *testing.T) {
	detector := NewDetector(&Config{
		Method: DetectionFieldName,
	})

	columns := map[string]interface{}{
		"customer_id":  1001,
		"total_amount": 50000000,
		"shipping_fee": 15000,
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDR, result.Currency)
	assert.Equal(t, 0.9, result.Confidence)
	assert.Equal(t, "FIELD_NAME", result.DetectedBy)
	assert.False(t, result.AmbiguityWarning)
}

func TestDetector_FieldNameDetection_IDN(t *testing.T) {
	detector := NewDetector(&Config{
		Method: DetectionFieldName,
	})

	columns := map[string]interface{}{
		"customer_id":      1001,
		"total_amount_idn": 50000.00,
		"shipping_fee_idn": 15.00,
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDN, result.Currency)
	assert.Equal(t, 0.9, result.Confidence)
	assert.Equal(t, "FIELD_NAME", result.DetectedBy)
	assert.False(t, result.AmbiguityWarning)
}

func TestDetector_FieldNameDetection_Mixed(t *testing.T) {
	detector := NewDetector(&Config{
		Method: DetectionFieldName,
	})

	// Both IDR and IDN columns present (ambiguous)
	columns := map[string]interface{}{
		"total_amount":     50000000,
		"total_amount_idn": 50000.00,
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDR, result.Currency) // Default to IDR
	assert.Equal(t, 0.3, result.Confidence)       // Low confidence
	assert.True(t, result.AmbiguityWarning)
}

func TestDetector_ValueRangeDetection_IDR(t *testing.T) {
	detector := NewDetector(&Config{
		Method:         DetectionValueRange,
		ThresholdValue: 1000000,
	})

	columns := map[string]interface{}{
		"customer_id":  1001,
		"total_amount": 50000000, // Large value = IDR
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDR, result.Currency)
	assert.Equal(t, 0.7, result.Confidence)
	assert.Equal(t, "VALUE_RANGE", result.DetectedBy)
}

func TestDetector_ValueRangeDetection_IDN(t *testing.T) {
	detector := NewDetector(&Config{
		Method:         DetectionValueRange,
		ThresholdValue: 1000000,
	})

	columns := map[string]interface{}{
		"customer_id":  1001,
		"total_amount": 50000, // Small value = IDN
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDN, result.Currency)
	assert.Equal(t, 0.7, result.Confidence)
	assert.Equal(t, "VALUE_RANGE", result.DetectedBy)
}

func TestDetector_ValueRangeDetection_EdgeCases(t *testing.T) {
	detector := NewDetector(&Config{
		Method:         DetectionValueRange,
		ThresholdValue: 1000000,
	})

	testCases := []struct {
		name     string
		value    interface{}
		expected CurrencyType
	}{
		{"Exactly threshold", 1000000, CurrencyIDR},
		{"Just below threshold", 999999, CurrencyIDN},
		{"Zero value", 0, CurrencyIDR}, // Default when ambiguous
		{"Float IDN", 50000.50, CurrencyIDN},
		{"Float IDR", 50000000.00, CurrencyIDR},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			columns := map[string]interface{}{
				"total_amount": tc.value,
			}

			result, err := detector.Detect(columns)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result.Currency, "Failed for value: %v", tc.value)
		})
	}
}

func TestDetector_AutoDetection_ExplicitWins(t *testing.T) {
	detector := NewDetector(&Config{
		Method:         DetectionAuto,
		CurrencyField:  "currency",
		ThresholdValue: 1000000,
	})

	// Explicit field present - should override all other methods
	columns := map[string]interface{}{
		"total_amount": 999999, // Value says IDN
		"currency":     "IDR",  // Explicit says IDR
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDR, result.Currency)
	assert.Equal(t, 1.0, result.Confidence)
	assert.Equal(t, "EXPLICIT", result.DetectedBy)
}

func TestDetector_AutoDetection_FieldNameHighConfidence(t *testing.T) {
	detector := NewDetector(&Config{
		Method: DetectionAuto,
	})

	// No explicit field, but clear field name signal
	columns := map[string]interface{}{
		"total_amount_idn": 50000.00,
		"shipping_fee_idn": 15.00,
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDN, result.Currency)
	assert.Equal(t, "AUTO", result.DetectedBy) // Always AUTO when using auto-detection
}

func TestDetector_AutoDetection_Agreement(t *testing.T) {
	detector := NewDetector(&Config{
		Method:         DetectionAuto,
		ThresholdValue: 1000000,
	})

	// Both field name and value range agree on IDN
	columns := map[string]interface{}{
		"total_amount_idn": 50000, // Field name says IDN, value says IDN
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDN, result.Currency)
	assert.False(t, result.AmbiguityWarning)
}

func TestDetector_AutoDetection_Disagreement(t *testing.T) {
	detector := NewDetector(&Config{
		Method:         DetectionAuto,
		ThresholdValue: 1000000,
	})

	// Field name says IDR, but value says IDN (conflicting signals)
	columns := map[string]interface{}{
		"total_amount": 50000, // No _idn suffix (IDR), but small value (IDN)
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	// Field name has higher confidence (0.9) than value range (0.7)
	// So currency should be IDR, but with ambiguity warning
	assert.Equal(t, CurrencyIDR, result.Currency)
	assert.True(t, result.AmbiguityWarning, "Expected ambiguity warning when field name and value range disagree")
}

func TestDetector_DefaultConfig(t *testing.T) {
	// Test that NewDetector works with nil config
	detector := NewDetector(nil)

	columns := map[string]interface{}{
		"total_amount": 50000000,
		"currency":     "IDR",
	}

	result, err := detector.Detect(columns)
	require.NoError(t, err)
	assert.Equal(t, CurrencyIDR, result.Currency)
}

func TestIsMonetaryColumn(t *testing.T) {
	testCases := []struct {
		column   string
		expected bool
	}{
		{"total_amount", true},
		{"shipping_fee", true},
		{"grand_total", true},
		{"price", true},
		{"subtotal", true},
		{"tax_amount", true},
		{"discount_rate", true},
		{"customer_id", false},
		{"order_date", false},
		{"status", false},
		{"notes", false},
	}

	for _, tc := range testCases {
		t.Run(tc.column, func(t *testing.T) {
			result := isMonetaryColumn(tc.column)
			assert.Equal(t, tc.expected, result)
		})
	}
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
		{"bool", true, 0, true},
		{"nil", nil, 0, true},
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

// Benchmark tests
func BenchmarkDetector_Explicit(b *testing.B) {
	detector := NewDetector(&Config{
		Method:        DetectionExplicit,
		CurrencyField: "currency",
	})

	columns := map[string]interface{}{
		"total_amount": 50000000,
		"currency":     "IDR",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.Detect(columns)
	}
}

func BenchmarkDetector_FieldName(b *testing.B) {
	detector := NewDetector(&Config{
		Method: DetectionFieldName,
	})

	columns := map[string]interface{}{
		"total_amount_idn": 50000.00,
		"shipping_fee_idn": 15.00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.Detect(columns)
	}
}

func BenchmarkDetector_Auto(b *testing.B) {
	detector := NewDetector(&Config{
		Method: DetectionAuto,
	})

	columns := map[string]interface{}{
		"total_amount": 50000000,
		"currency":     "IDR",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.Detect(columns)
	}
}
