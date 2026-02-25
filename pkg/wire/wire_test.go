package wire

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/timbrinded/hlgo/pkg/output"
)

func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic("bad test decimal: " + s)
	}
	return v
}

func TestCountSigFigs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"zero", "0", 1},
		{"one", "1", 1},
		{"integer_small", "42", 2},
		{"integer_btc", "95000", 5},
		{"integer_100k", "100000", 6},
		{"decimal_eth", "3412.1", 5},
		{"decimal_small", "0.00001234", 4},
		{"decimal_five_sigfig", "95123.5", 6},
		{"decimal_one_sigfig", "0.003", 1},
		{"decimal_two_sigfig", "0.0012", 2},
		{"negative_value", "-3412.1", 5},
		// shopspring/decimal normalizes 10.00 to 10, so we see 2 sigfigs.
		{"trailing_decimal_zero", "10.00", 2},
		{"exact_five", "12345", 5},
		{"single_nonzero_frac", "0.5", 1},
		{"large_integer", "1000000", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountSigFigs(d(tt.input))
			if got != tt.want {
				t.Errorf("CountSigFigs(%s) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidatePrice(t *testing.T) {
	tests := []struct {
		name       string
		price      string
		szDecimals int
		isSpot     bool
		wantErr    bool
		wantCode   output.ErrorCode
	}{
		// Valid perp prices.
		{
			name:       "valid_eth_price",
			price:      "3412.1",
			szDecimals: 2,
			isSpot:     false,
			wantErr:    false,
		},
		{
			name:       "valid_btc_integer",
			price:      "95000",
			szDecimals: 3,
			isSpot:     false,
			wantErr:    false,
		},
		{
			name:       "valid_large_integer_100k",
			price:      "100000",
			szDecimals: 3,
			isSpot:     false,
			wantErr:    false,
		},
		{
			name:       "valid_exact_5_sigfigs",
			price:      "12345",
			szDecimals: 1,
			isSpot:     false,
			wantErr:    false,
		},
		{
			name:       "valid_meme_coin",
			price:      "0.00012",
			szDecimals: 0,
			isSpot:     false,
			wantErr:    false,
		},
		{
			name:       "valid_5_sigfig_decimal",
			price:      "0.012345",
			szDecimals: 0,
			isSpot:     false,
			wantErr:    false,
		},
		// Invalid: too many sigfigs (non-integer).
		{
			name:       "invalid_6_sigfigs",
			price:      "95123.5",
			szDecimals: 2,
			isSpot:     false,
			wantErr:    true,
			wantCode:   output.ErrValidation,
		},
		{
			name:       "invalid_7_sigfigs_small",
			price:      "0.0001234567",
			szDecimals: 0,
			isSpot:     false,
			wantErr:    true,
			wantCode:   output.ErrValidation,
		},
		// Invalid: too many decimal places.
		{
			name:       "invalid_perp_too_many_decimals",
			price:      "3412.12345",
			szDecimals: 2,
			isSpot:     false,
			wantErr:    true,
			wantCode:   output.ErrValidation,
		},
		{
			name:       "valid_spot_more_decimals",
			price:      "0.012345",
			szDecimals: 2,
			isSpot:     true,
			wantErr:    false,
		},
		{
			name:       "invalid_spot_too_many_decimals",
			price:      "1.1234567",
			szDecimals: 2,
			isSpot:     true,
			wantErr:    true,
			wantCode:   output.ErrValidation,
		},
		// Invalid: non-positive.
		{
			name:       "invalid_zero",
			price:      "0",
			szDecimals: 2,
			isSpot:     false,
			wantErr:    true,
			wantCode:   output.ErrValidation,
		},
		{
			name:       "invalid_negative",
			price:      "-100",
			szDecimals: 2,
			isSpot:     false,
			wantErr:    true,
			wantCode:   output.ErrValidation,
		},
		// Integer always passes sigfig validation.
		{
			name:       "integer_many_digits_valid",
			price:      "1234567890",
			szDecimals: 0,
			isSpot:     false,
			wantErr:    false,
		},
		// Edge: szDecimals consumes all allowed decimals.
		{
			name:       "szDecimals_equals_max_perp",
			price:      "100",
			szDecimals: 6,
			isSpot:     false,
			wantErr:    false,
		},
		{
			name:       "szDecimals_equals_max_perp_decimal_rejected",
			price:      "100.1",
			szDecimals: 6,
			isSpot:     false,
			wantErr:    true,
			wantCode:   output.ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePrice(d(tt.price), tt.szDecimals, tt.isSpot)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePrice(%s, %d, spot=%v) error = %v, wantErr %v",
					tt.price, tt.szDecimals, tt.isSpot, err, tt.wantErr)
			}
			if tt.wantErr && tt.wantCode != "" {
				var cliErr *output.CLIError
				if !errors.As(err, &cliErr) {
					t.Fatalf("expected *output.CLIError, got %T", err)
				}
				if cliErr.Code != tt.wantCode {
					t.Errorf("error code = %s, want %s", cliErr.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestPriceToWire(t *testing.T) {
	tests := []struct {
		name       string
		price      string
		szDecimals int
		isSpot     bool
		want       string
		wantErr    bool
	}{
		{"eth_price", "3412.1", 2, false, "3412.1", false},
		{"btc_integer", "95000", 3, false, "95000", false},
		{"meme_coin", "0.00012", 0, false, "0.00012", false},
		{"invalid_rejected", "95123.5", 2, false, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PriceToWire(d(tt.price), tt.szDecimals, tt.isSpot)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PriceToWire(%s) error = %v, wantErr %v", tt.price, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("PriceToWire(%s) = %q, want %q", tt.price, got, tt.want)
			}
		})
	}
}

func TestSizeToWire(t *testing.T) {
	tests := []struct {
		name       string
		size       string
		szDecimals int
		want       string
		wantErr    bool
	}{
		{"round_to_2", "1.23456", 2, "1.23", false},
		{"round_to_3", "0.12345", 3, "0.123", false},
		{"round_up", "1.005", 2, "1.01", false},
		{"strip_trailing_zeros", "1.10000", 2, "1.1", false},
		{"integer_result", "5.001", 0, "5", false},
		{"large_size", "1000.123456", 4, "1000.1235", false},
		{"no_rounding_needed", "0.5", 4, "0.5", false},
		{"round_to_8_spot", "0.123456789", 8, "0.12345679", false},
		// Validation errors.
		{"zero_size", "0", 2, "", true},
		{"negative_size", "-1.5", 2, "", true},
		{"negative_szDecimals", "1.5", -1, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SizeToWire(d(tt.size), tt.szDecimals)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SizeToWire(%s, %d) error = %v, wantErr %v", tt.size, tt.szDecimals, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SizeToWire(%s, %d) = %q, want %q", tt.size, tt.szDecimals, got, tt.want)
			}
		})
	}
}

func TestValidatePrice_NegativeSzDecimals(t *testing.T) {
	err := ValidatePrice(d("100"), -1, false)
	if err == nil {
		t.Fatal("expected error for negative szDecimals")
	}
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrValidation)
	}
}

func TestNearestValidPrice(t *testing.T) {
	tests := []struct {
		name       string
		price      string
		szDecimals int
		isSpot     bool
		want       string
	}{
		{
			name:       "already_valid",
			price:      "3412.1",
			szDecimals: 2,
			isSpot:     false,
			want:       "3412.1",
		},
		{
			name:       "too_many_decimals_and_sigfigs_perp",
			price:      "3412.12345",
			szDecimals: 2,
			isSpot:     false,
			// Truncate to 4 decimals → 3412.1234 (8 sigfigs), then round to 5 sigfigs → 3412.1.
			want: "3412.1",
		},
		{
			name:       "too_many_sigfigs_snap",
			price:      "95123.5",
			szDecimals: 2,
			isSpot:     false,
			want:       "95124",
		},
		{
			name:       "meme_coin_too_precise",
			price:      "0.00001234567",
			szDecimals: 0,
			isSpot:     false,
			want:       "0.000012",
		},
		{
			name:       "spot_allows_more_decimals",
			price:      "1.12345678",
			szDecimals: 2,
			isSpot:     true,
			want:       "1.1235",
		},
		{
			name:       "integer_unchanged",
			price:      "100000",
			szDecimals: 3,
			isSpot:     false,
			want:       "100000",
		},
		{
			name:       "zero_unchanged",
			price:      "0",
			szDecimals: 2,
			isSpot:     false,
			want:       "0",
		},
		{
			name:       "negative_unchanged",
			price:      "-100",
			szDecimals: 2,
			isSpot:     false,
			want:       "-100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NearestValidPrice(d(tt.price), tt.szDecimals, tt.isSpot)
			want := d(tt.want)
			if !got.Equal(want) {
				t.Errorf("NearestValidPrice(%s, %d, spot=%v) = %s, want %s",
					tt.price, tt.szDecimals, tt.isSpot, got, tt.want)
			}
		})
	}
}

func TestNearestValidPrice_PassesValidation(t *testing.T) {
	// Verify that NearestValidPrice output always passes ValidatePrice
	// for positive values.
	cases := []struct {
		price      string
		szDecimals int
		isSpot     bool
	}{
		{"95123.5", 2, false},
		{"3412.12345", 2, false},
		{"0.00001234567", 0, false},
		{"1.12345678", 2, true},
		{"99999.99999", 1, false},
		{"0.123456789", 0, true},
	}

	for _, c := range cases {
		t.Run(c.price, func(t *testing.T) {
			snapped := NearestValidPrice(d(c.price), c.szDecimals, c.isSpot)
			if snapped.LessThanOrEqual(decimal.Zero) {
				t.Skip("non-positive snapped value")
			}
			if err := ValidatePrice(snapped, c.szDecimals, c.isSpot); err != nil {
				t.Errorf("NearestValidPrice(%s) = %s, but ValidatePrice fails: %v",
					c.price, snapped, err)
			}
		})
	}
}

func TestSpotVsPerpMaxDecimals(t *testing.T) {
	// A price with 7 decimal places but only 5 sigfigs: valid for spot, invalid for perp.
	// 0.0012345 has 5 sigfigs and 7 decimal places.
	// Perp: maxDecimals=6, szDecimals=0 → 6 allowed decimals → 7 > 6, fails.
	// Spot: maxDecimals=8, szDecimals=0 → 8 allowed decimals → 7 <= 8, passes.
	price := d("0.0012345")

	errPerp := ValidatePrice(price, 0, false)
	if errPerp == nil {
		t.Error("expected perp validation to fail for 7-decimal price")
	}

	errSpot := ValidatePrice(price, 0, true)
	if errSpot != nil {
		t.Errorf("expected spot validation to pass for 0.0012345 with szDecimals=0: %v", errSpot)
	}

	// A price with 6 decimal places and 5 sigfigs: valid for both perp and spot.
	price2 := d("0.012345")

	errPerp2 := ValidatePrice(price2, 0, false)
	if errPerp2 != nil {
		t.Errorf("expected perp validation to pass for 0.012345: %v", errPerp2)
	}

	errSpot2 := ValidatePrice(price2, 0, true)
	if errSpot2 != nil {
		t.Errorf("expected spot validation to pass for 0.012345: %v", errSpot2)
	}
}

func TestValidatePrice_ErrorDetails(t *testing.T) {
	err := ValidatePrice(d("95123.5"), 2, false)
	if err == nil {
		t.Fatal("expected error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}

	// Verify actionable details are present per SOUL.md.
	if cliErr.Details["value"] != "95123.5" {
		t.Errorf("details.value = %v, want %q", cliErr.Details["value"], "95123.5")
	}
	if _, ok := cliErr.Details["nearest_valid"]; !ok {
		t.Error("details.nearest_valid is missing")
	}
}
