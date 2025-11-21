package detector

import (
	"fmt"
	"strings"
)

// DetectionMethod defines how currency is detected
type DetectionMethod string

const (
	DetectionAuto       DetectionMethod = "AUTO"        // Multi-strategy auto-detection
	DetectionExplicit   DetectionMethod = "EXPLICIT"    // Via currency field
	DetectionFieldName  DetectionMethod = "FIELD_NAME"  // Column name suffix (_idn)
	DetectionValueRange DetectionMethod = "VALUE_RANGE" // Numeric threshold
)

// CurrencyType represents the detected currency
type CurrencyType string

const (
	CurrencyIDR CurrencyType = "IDR" // Indonesian Rupiah (old)
	CurrencyIDN CurrencyType = "IDN" // Indonesian Rupiah Denominated (new)
)

// DetectionResult holds the outcome of currency detection
type DetectionResult struct {
	Currency         CurrencyType // Detected currency
	Confidence       float64      // Confidence level (0.0 - 1.0)
	DetectedBy       string       // Method that made the detection
	AmbiguityWarning bool         // True if multiple methods disagree
}

// Config holds detector configuration
type Config struct {
	Method         DetectionMethod
	ThresholdValue int64  // Value threshold for range detection (default: 1000000)
	CurrencyField  string // Field name for explicit detection (default: "currency")
}

// CurrencyDetector detects currency format from query columns
type CurrencyDetector struct {
	method         DetectionMethod
	thresholdValue int64
	currencyField  string
}

// NewDetector creates a new currency detector
func NewDetector(cfg *Config) *CurrencyDetector {
	if cfg == nil {
		cfg = &Config{
			Method:         DetectionAuto,
			ThresholdValue: 1000000,
			CurrencyField:  "currency",
		}
	}

	// Set defaults
	if cfg.ThresholdValue == 0 {
		cfg.ThresholdValue = 1000000
	}
	if cfg.CurrencyField == "" {
		cfg.CurrencyField = "currency"
	}

	return &CurrencyDetector{
		method:         cfg.Method,
		thresholdValue: cfg.ThresholdValue,
		currencyField:  cfg.CurrencyField,
	}
}

// Detect analyzes query columns and returns detected currency
func (d *CurrencyDetector) Detect(columns map[string]interface{}) (*DetectionResult, error) {
	switch d.method {
	case DetectionExplicit:
		return d.detectExplicit(columns)
	case DetectionFieldName:
		return d.detectByFieldName(columns)
	case DetectionValueRange:
		return d.detectByValueRange(columns)
	case DetectionAuto:
		return d.detectAuto(columns)
	default:
		return nil, fmt.Errorf("unknown detection method: %s", d.method)
	}
}

// detectExplicit looks for explicit currency field in columns
func (d *CurrencyDetector) detectExplicit(columns map[string]interface{}) (*DetectionResult, error) {
	currency, exists := columns[d.currencyField]
	if !exists {
		return nil, fmt.Errorf("currency field '%s' not found in columns", d.currencyField)
	}

	currencyStr, ok := currency.(string)
	if !ok {
		return nil, fmt.Errorf("currency field is not a string")
	}

	currencyStr = strings.ToUpper(strings.TrimSpace(currencyStr))

	switch currencyStr {
	case "IDR":
		return &DetectionResult{
			Currency:   CurrencyIDR,
			Confidence: 1.0,
			DetectedBy: "EXPLICIT",
		}, nil
	case "IDN":
		return &DetectionResult{
			Currency:   CurrencyIDN,
			Confidence: 1.0,
			DetectedBy: "EXPLICIT",
		}, nil
	default:
		return nil, fmt.Errorf("invalid currency value: %s (expected IDR or IDN)", currencyStr)
	}
}

// detectByFieldName checks column name suffixes to determine currency
func (d *CurrencyDetector) detectByFieldName(columns map[string]interface{}) (*DetectionResult, error) {
	idnCount := 0
	idrCount := 0

	for colName := range columns {
		// Check if column is monetary
		if !isMonetaryColumn(colName) {
			continue
		}

		// Check for _idn suffix (indicates IDN currency)
		if strings.HasSuffix(strings.ToLower(colName), "_idn") {
			idnCount++
		} else {
			// Monetary column without _idn suffix = IDR
			idrCount++
		}
	}

	// Clear IDN signal
	if idnCount > 0 && idrCount == 0 {
		return &DetectionResult{
			Currency:   CurrencyIDN,
			Confidence: 0.9,
			DetectedBy: "FIELD_NAME",
		}, nil
	}

	// Clear IDR signal
	if idrCount > 0 && idnCount == 0 {
		return &DetectionResult{
			Currency:   CurrencyIDR,
			Confidence: 0.9,
			DetectedBy: "FIELD_NAME",
		}, nil
	}

	// Ambiguous: both IDR and IDN columns present
	// Default to IDR for backward compatibility
	return &DetectionResult{
		Currency:         CurrencyIDR,
		Confidence:       0.3,
		DetectedBy:       "FIELD_NAME",
		AmbiguityWarning: true,
	}, nil
}

// detectByValueRange analyzes numeric values to determine currency
func (d *CurrencyDetector) detectByValueRange(columns map[string]interface{}) (*DetectionResult, error) {
	for colName, value := range columns {
		// Only analyze monetary columns
		if !isMonetaryColumn(colName) {
			continue
		}

		// Convert value to int64
		numValue, err := toInt64(value)
		if err != nil {
			// Skip non-numeric values
			continue
		}

		// Skip zero values (ambiguous)
		if numValue == 0 {
			continue
		}

		// If value < threshold, likely IDN (post-division)
		// Example: 50000.00 (IDN) vs 50000000 (IDR)
		if numValue < d.thresholdValue {
			return &DetectionResult{
				Currency:   CurrencyIDN,
				Confidence: 0.7,
				DetectedBy: "VALUE_RANGE",
			}, nil
		}
	}

	// Default to IDR (values above threshold or no monetary columns)
	return &DetectionResult{
		Currency:   CurrencyIDR,
		Confidence: 0.7,
		DetectedBy: "VALUE_RANGE",
	}, nil
}

// detectAuto uses multiple strategies and combines results
func (d *CurrencyDetector) detectAuto(columns map[string]interface{}) (*DetectionResult, error) {
	// Strategy 1: Try explicit first (highest confidence)
	if result, err := d.detectExplicit(columns); err == nil {
		return result, nil
	}

	// Strategy 2 & 3: Run both field name and value range detection
	fieldResult, fieldErr := d.detectByFieldName(columns)
	valueResult, valueErr := d.detectByValueRange(columns)

	// If both methods failed, return error
	if fieldErr != nil && valueErr != nil {
		return nil, fmt.Errorf("auto-detection failed: no monetary columns found")
	}

	// If only one method succeeded, use it
	if fieldErr != nil {
		return valueResult, nil
	}
	if valueErr != nil {
		return fieldResult, nil
	}

	// Both methods succeeded: check for agreement/disagreement
	if fieldResult.Currency == valueResult.Currency {
		// Agreement: combine confidence
		return &DetectionResult{
			Currency:   fieldResult.Currency,
			Confidence: (fieldResult.Confidence + valueResult.Confidence) / 2,
			DetectedBy: "AUTO",
		}, nil
	}

	// Disagreement detected: use highest confidence, but set ambiguity warning
	if fieldResult.Confidence > valueResult.Confidence {
		return &DetectionResult{
			Currency:         fieldResult.Currency,
			Confidence:       fieldResult.Confidence,
			DetectedBy:       "AUTO",
			AmbiguityWarning: true,
		}, nil
	}

	return &DetectionResult{
		Currency:         valueResult.Currency,
		Confidence:       valueResult.Confidence,
		DetectedBy:       "AUTO",
		AmbiguityWarning: true,
	}, nil
}

// isMonetaryColumn checks if column name suggests monetary values
func isMonetaryColumn(name string) bool {
	monetaryKeywords := []string{
		"amount", "price", "fee", "total", "cost",
		"balance", "payment", "charge", "rate",
		"grand_total", "subtotal", "tax", "discount",
	}

	lowerName := strings.ToLower(name)

	for _, keyword := range monetaryKeywords {
		if strings.Contains(lowerName, keyword) {
			return true
		}
	}

	return false
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
