package rounding

import (
	"math"
)

// Strategy represents the rounding strategy to use
type Strategy string

const (
	// BankersRound uses IEEE 754 Round Half to Even
	BankersRound Strategy = "BANKERS_ROUND"
	// ArithmeticRound uses standard arithmetic rounding (round half up)
	ArithmeticRound Strategy = "ARITHMETIC_ROUND"
	// NoRound returns exact decimal value without any rounding
	NoRound Strategy = "NO_ROUND"
)

// Engine handles currency value rounding
type Engine struct {
	strategy  Strategy
	precision int
}

// NewEngine creates a new rounding engine
func NewEngine(strategy Strategy, precision int) *Engine {
	return &Engine{
		strategy:  strategy,
		precision: precision,
	}
}

// Round rounds a value according to the configured strategy and precision
func (e *Engine) Round(value float64) float64 {
	switch e.strategy {
	case BankersRound:
		return e.bankersRound(value)
	case ArithmeticRound:
		return e.arithmeticRound(value)
	case NoRound:
		return e.noRound(value)
	default:
		return e.bankersRound(value) // Default to Banker's Round
	}
}

// bankersRound implements IEEE 754 Round Half to Even
// When the value is exactly halfway between two numbers, it rounds to the nearest even number
func (e *Engine) bankersRound(value float64) float64 {
	multiplier := math.Pow(10, float64(e.precision))
	adjusted := value * multiplier

	floor := math.Floor(adjusted)
	ceil := math.Ceil(adjusted)
	fraction := adjusted - floor

	// Exact comparison for halfway point
	const epsilon = 1e-9

	if fraction < 0.5-epsilon {
		// Round down
		return floor / multiplier
	} else if fraction > 0.5+epsilon {
		// Round up
		return ceil / multiplier
	} else {
		// Exactly 0.5: round to even
		if int64(floor)%2 == 0 {
			return floor / multiplier
		} else {
			return ceil / multiplier
		}
	}
}

// arithmeticRound implements standard arithmetic rounding (round half up)
func (e *Engine) arithmeticRound(value float64) float64 {
	multiplier := math.Pow(10, float64(e.precision))
	return math.Round(value*multiplier) / multiplier
}

// noRound returns the exact value without rounding
// Note: Still applies precision truncation for display purposes
func (e *Engine) noRound(value float64) float64 {
	// Simply truncate to specified precision without rounding
	multiplier := math.Pow(10, float64(e.precision))
	return math.Trunc(value*multiplier) / multiplier
}

// ConvertIDRtoIDN converts IDR (integer) to IDN (decimal) with rounding
func (e *Engine) ConvertIDRtoIDN(idrValue int64, ratio int) float64 {
	// Convert to float and divide by ratio
	idnValue := float64(idrValue) / float64(ratio)

	// Round according to strategy
	return e.Round(idnValue)
}
