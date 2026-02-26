// Package wire handles price and size formatting to Hyperliquid wire protocol constraints
// including tick size, lot size, and significant figure validation.
package wire

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/timbrinded/hlgo/pkg/output"
)

const (
	// maxSigFigs is the maximum significant figures allowed for non-integer prices.
	maxSigFigs = 5

	// maxDecimalsPerp is the maximum total decimal places for perp markets.
	maxDecimalsPerp = 6

	// maxDecimalsSpot is the maximum total decimal places for spot markets.
	maxDecimalsSpot = 8
)

// maxDecimals returns the market-specific maximum decimal places.
func maxDecimals(isSpot bool) int {
	if isSpot {
		return maxDecimalsSpot
	}
	return maxDecimalsPerp
}

// CountSigFigs counts the significant figures in a decimal value.
// Leading zeros are excluded. For integers (no fractional part), the count
// includes all digits including trailing zeros.
//
// Examples:
//
//	3412.1    → 5
//	0.00001234 → 4
//	95000     → 5
//	100000    → 6
//	10.00     → 2 (shopspring/decimal normalizes to 10)
func CountSigFigs(d decimal.Decimal) int {
	if d.IsZero() {
		return 1
	}

	// Work with the absolute value for counting.
	d = d.Abs()

	// Get the string representation and strip the decimal point.
	// The Abs() above ensures no minus sign is present.
	s := d.String()
	s = strings.Replace(s, ".", "", 1)

	// Strip leading zeros.
	s = strings.TrimLeft(s, "0")

	if len(s) == 0 {
		return 1
	}

	return len(s)
}

// isInteger reports whether d has no fractional component.
func isInteger(d decimal.Decimal) bool {
	return d.Equal(d.Truncate(0))
}

// maxPriceDecimals returns the maximum number of decimal places allowed for a price
// given the size decimals and market type.
func maxPriceDecimals(szDecimals int, isSpot bool) int {
	allowed := maxDecimals(isSpot) - szDecimals
	if allowed < 0 {
		return 0
	}
	return allowed
}

// ValidatePrice checks that a price conforms to Hyperliquid wire protocol constraints.
// Returns nil if valid, or a structured CLIError with details on failure.
//
// Rules:
//   - Price must be positive
//   - szDecimals must be non-negative
//   - Non-integer prices: max 5 significant figures
//   - Max decimal places: maxDecimals(isSpot) - szDecimals
//   - Integer prices always pass sigfig validation
func ValidatePrice(price decimal.Decimal, szDecimals int, isSpot bool) error {
	if szDecimals < 0 {
		return output.NewCLIError(output.ErrValidation, "szDecimals must be non-negative").
			WithDetails("sz_decimals", szDecimals)
	}

	if price.LessThanOrEqual(decimal.Zero) {
		return output.NewCLIError(output.ErrValidation, "price must be positive").
			WithDetails("value", price.String())
	}

	allowedDecimals := maxPriceDecimals(szDecimals, isSpot)

	// Check decimal places: truncate to allowed decimals and compare.
	truncated := price.Truncate(int32(allowedDecimals))
	if !price.Equal(truncated) {
		return output.NewCLIError(output.ErrValidation,
			fmt.Sprintf("price exceeds maximum %d decimal places", allowedDecimals)).
			WithDetails("value", price.String()).
			WithDetails("max_decimals", allowedDecimals).
			WithDetails("nearest_valid", NearestValidPrice(price, szDecimals, isSpot).String())
	}

	// Integers always pass sigfig validation.
	if isInteger(price) {
		return nil
	}

	// Non-integer: enforce max 5 significant figures.
	sigfigs := CountSigFigs(price)
	if sigfigs > maxSigFigs {
		return output.NewCLIError(output.ErrValidation,
			fmt.Sprintf("price has %d significant figures, maximum is %d", sigfigs, maxSigFigs)).
			WithDetails("value", price.String()).
			WithDetails("sig_figs", sigfigs).
			WithDetails("max_sig_figs", maxSigFigs).
			WithDetails("nearest_valid", NearestValidPrice(price, szDecimals, isSpot).String())
	}

	return nil
}

// PriceToWire validates and formats a price for the Hyperliquid wire protocol.
// Returns the string representation suitable for JSON serialization.
func PriceToWire(price decimal.Decimal, szDecimals int, isSpot bool) (string, error) {
	if err := ValidatePrice(price, szDecimals, isSpot); err != nil {
		return "", err
	}
	return price.String(), nil
}

// SizeToWire validates and formats a size for the Hyperliquid wire protocol.
// The size is rounded to szDecimals places and trailing zeros are stripped.
func SizeToWire(size decimal.Decimal, szDecimals int) (string, error) {
	if szDecimals < 0 {
		return "", output.NewCLIError(output.ErrValidation, "szDecimals must be non-negative").
			WithDetails("sz_decimals", szDecimals)
	}
	if size.LessThanOrEqual(decimal.Zero) {
		return "", output.NewCLIError(output.ErrValidation, "size must be positive").
			WithDetails("value", size.String())
	}
	rounded := size.Round(int32(szDecimals))
	// String() from shopspring/decimal strips trailing zeros by default.
	return rounded.String(), nil
}

// NearestValidPrice snaps a price to the nearest value that passes ValidatePrice.
// It first truncates to the allowed decimal places, then rounds to maxSigFigs
// significant figures if the result is non-integer.
func NearestValidPrice(price decimal.Decimal, szDecimals int, isSpot bool) decimal.Decimal {
	if price.LessThanOrEqual(decimal.Zero) {
		return price
	}

	allowedDecimals := maxPriceDecimals(szDecimals, isSpot)

	// Step 1: Truncate to allowed decimal places.
	result := price.Truncate(int32(allowedDecimals))

	// Step 2: If result is non-integer and has too many sigfigs, round to maxSigFigs.
	if !isInteger(result) && CountSigFigs(result) > maxSigFigs {
		result = roundToSigFigs(result, maxSigFigs)
		// Re-truncate in case rounding introduced extra decimals.
		result = result.Truncate(int32(allowedDecimals))
	}

	return result
}

// roundToSigFigs rounds d to n significant figures.
func roundToSigFigs(d decimal.Decimal, n int) decimal.Decimal {
	if d.IsZero() || n <= 0 {
		return decimal.Zero
	}

	abs := d.Abs()

	// Find the order of magnitude: floor(log10(abs)).
	// We determine this by finding the number of integer digits.
	// For numbers >= 1: intDigits = len(integer part)
	// For numbers < 1: we need to count leading zeros after the decimal point.
	intPart := abs.Truncate(0)

	var magnitude int
	if intPart.IsZero() {
		// Number is less than 1 (e.g., 0.001234).
		// Find the position of the first significant digit.
		s := abs.String()
		_, fracPart, hasDot := strings.Cut(s, ".")
		if !hasDot {
			return decimal.Zero
		}
		leadingZeros := 0
		for _, c := range fracPart {
			if c != '0' {
				break
			}
			leadingZeros++
		}
		// magnitude is negative: e.g. 0.001234 has first sigfig at 10^-3
		magnitude = -(leadingZeros + 1)
	} else {
		magnitude = len(intPart.String()) - 1
	}

	// Round to n sigfigs: round at position (magnitude - n + 1) from the decimal point.
	// In terms of decimal places: -(magnitude - n + 1) = n - magnitude - 1.
	decimalPlaces := int32(n - magnitude - 1)

	result := d.Round(decimalPlaces)
	return result
}
