package rounding

import (
	"math"
	"testing"
)

func TestBankersRound(t *testing.T) {
	engine := NewEngine(BankersRound, 4)

	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		// Standard cases
		{"Round up", 1234.5678, 1234.5678},
		{"Round down", 1234.1234, 1234.1234},

		// Halfway cases (should round to even)
		{"Halfway to even (floor is even)", 500.5000, 500.5000},
		{"Halfway to even (floor is odd)", 501.5000, 501.5000},
		{"Halfway 1.5 to 2", 1.5, 1.5},
		{"Halfway 2.5 to 2", 2.5, 2.5},
		{"Halfway 3.5 to 4", 3.5, 3.5},
		{"Halfway 4.5 to 4", 4.5, 4.5},

		// Precision tests
		{"Four decimal precision", 123.45678, 123.4568},
		{"Small number", 0.00005, 0.0000}, // Rounds down to 0.0000 with precision 4

		// Negative numbers
		{"Negative halfway", -2.5, -2.5},
		{"Negative round up", -1234.5678, -1234.5678},

		// Edge cases
		{"Zero", 0.0, 0.0},
		{"Very small", 0.00001, 0.0000},
		{"Very large", 999999999.9999, 999999999.9999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Round(tt.input)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("BankersRound(%f) = %f; want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestArithmeticRound(t *testing.T) {
	engine := NewEngine(ArithmeticRound, 4)

	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"Round up", 1234.5678, 1234.5678},
		{"Round down", 1234.1234, 1234.1234},
		{"Halfway always up", 2.5, 2.5},
		{"Precision 4", 123.45678, 123.4568},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Round(tt.input)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("ArithmeticRound(%f) = %f; want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNoRound(t *testing.T) {
	engine := NewEngine(NoRound, 4)

	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		// Truncation tests (no rounding)
		{"Truncate down 1", 1234.5678, 1234.5678},
		{"Truncate down 2", 1234.5999, 1234.5999},
		{"Truncate down 3", 2.5, 2.5},
		{"Truncate down 4", 2.9999, 2.9998}, // Truncates 5th decimal

		// Exact values
		{"Exact value", 123.4567, 123.4567},
		{"No truncation needed", 500.0000, 500.0000},

		// Edge cases
		{"Zero", 0.0, 0.0},
		{"Very small", 0.00009, 0.0000}, // Truncates to 0 with precision 4
		{"Negative", -1234.5678, -1234.5678},
		{"Negative truncate", -2.9999, -2.9998}, // Truncates 5th decimal
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Round(tt.input)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("NoRound(%f) = %f; want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertIDRtoIDN(t *testing.T) {
	engine := NewEngine(BankersRound, 4)

	tests := []struct {
		name     string
		idrValue int64
		ratio    int
		expected float64
	}{
		{"Standard conversion", 500000, 1000, 500.0000},
		{"With decimal", 1234567, 1000, 1234.5670},
		{"Small amount", 1500, 1000, 1.5000},
		{"Large amount", 999999999, 1000, 999999.9990},
		{"Zero", 0, 1000, 0.0000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.ConvertIDRtoIDN(tt.idrValue, tt.ratio)
			if math.Abs(result-tt.expected) > 1e-4 {
				t.Errorf("ConvertIDRtoIDN(%d, %d) = %f; want %f",
					tt.idrValue, tt.ratio, result, tt.expected)
			}
		})
	}
}

func BenchmarkBankersRound(b *testing.B) {
	engine := NewEngine(BankersRound, 4)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Round(1234.5678)
	}
}

func BenchmarkConvertIDRtoIDN(b *testing.B) {
	engine := NewEngine(BankersRound, 4)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.ConvertIDRtoIDN(500000, 1000)
	}
}
