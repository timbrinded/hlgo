package cmd

import (
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

const (
	mainnetURL = "https://api.hyperliquid.xyz"
	testnetURL = "https://api.hyperliquid-testnet.xyz"
)

// baseURL returns the API base URL based on the testnet config flag.
// HLGO_API_URL overrides for testing.
func baseURL(cfg *config.Config) string {
	if u := os.Getenv("HLGO_API_URL"); u != "" {
		return u
	}
	if cfg.Testnet {
		return testnetURL
	}
	return mainnetURL
}

// buildInfoClient creates an InfoClient configured from the current config.
func buildInfoClient(cfg *config.Config) *info.InfoClient {
	return info.NewInfoClient(client.NewClient(baseURL(cfg)))
}

// parseTimeFlag parses a time string as either Unix milliseconds or ISO 8601.
func parseTimeFlag(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}

	// Try Unix millisecond timestamp first.
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return ms, nil
	}

	// Try ISO 8601 formats.
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UnixMilli(), nil
		}
	}

	return 0, output.NewCLIError(output.ErrValidation, "invalid time format: "+s).
		WithDetails("value", s).
		WithDetails("hint", "use Unix milliseconds or ISO 8601 (e.g. 2024-01-15T10:30:00Z)")
}

// printResult outputs the result in the configured format.
// For JSON format, raw API response is printed directly (preserving precision).
// For table/CSV, the Tabular implementation is used. If tab is nil, falls back to JSON.
func printResult(cmd *cobra.Command, cfg *config.Config, raw json.RawMessage, tab output.Tabular) error {
	f := output.NewFormatter(cfg.Format, cfg.Quiet)
	if cfg.Format == "json" || tab == nil {
		return f.Format(cmd.OutOrStdout(), raw)
	}
	return f.Format(cmd.OutOrStdout(), tab)
}

// mustMarshal marshals v to JSON, panicking on error (for request dry-run output).
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic("mustMarshal: " + err.Error())
	}
	return data
}
