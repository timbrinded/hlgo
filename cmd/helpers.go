package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/signer"
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

// buildHTTPClient creates a raw HTTP client for the current config.
func buildHTTPClient(cfg *config.Config) *client.Client {
	return client.NewClient(baseURL(cfg))
}

// buildInfoClient creates an InfoClient configured from the current config.
func buildInfoClient(cfg *config.Config) *info.InfoClient {
	return info.NewInfoClient(buildHTTPClient(cfg))
}

func buildResolver(cfg *config.Config) resolver.Resolver {
	return resolver.NewResolver(
		buildHTTPClient(cfg),
		resolveCacheDir(cfg),
		time.Duration(cfg.MetadataTTL)*time.Second,
	)
}

// buildExecutor constructs an exchange.Executor from the current config.
func buildExecutor(cfg *config.Config) (*exchange.Executor, error) {
	keyHex := os.Getenv(cfg.PrivateKeyEnv)
	if keyHex == "" {
		return nil, output.NewCLIError(output.ErrConfig, "private key not set").
			WithDetails("env_var", cfg.PrivateKeyEnv)
	}

	s, err := signer.NewSigner(keyHex)
	if err != nil {
		return nil, err
	}

	httpClient := buildHTTPClient(cfg)
	assetResolver := resolver.NewResolver(httpClient, resolveCacheDir(cfg), time.Duration(cfg.MetadataTTL)*time.Second)
	return exchange.NewExecutor(s, httpClient, assetResolver, !cfg.Testnet), nil
}

// resolveCacheDir returns the cache directory path for the current network.
func resolveCacheDir(cfg *config.Config) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if cfg.Testnet {
		return filepath.Join(home, ".hlgo", "cache", "testnet")
	}
	return filepath.Join(home, ".hlgo", "cache", "mainnet")
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

func parseDecimalField(field, value string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Decimal{}, output.NewCLIError(output.ErrValidation, "invalid "+field).
			WithDetails("value", value)
	}
	return parsed, nil
}

func parseOrderSide(side string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(side))
	if normalized == "buy" || normalized == "sell" {
		return normalized, nil
	}
	return "", output.NewCLIError(output.ErrValidation, "side must be 'buy' or 'sell'").
		WithDetails("value", side)
}

func resolveOrderTIF(tif string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(tif)) {
	case "gtc":
		return "Gtc", true
	case "ioc":
		return "Ioc", true
	case "alo":
		return "Alo", true
	default:
		return "", false
	}
}

func parseOrderTIF(tif string) (string, error) {
	wireTIF, ok := resolveOrderTIF(tif)
	if ok {
		return wireTIF, nil
	}
	return "", output.NewCLIError(output.ErrValidation, "invalid tif: "+tif).
		WithDetails("value", tif).
		WithDetails("valid", "gtc, ioc, alo")
}

func parseOptionalExpiresAfter(cmd *cobra.Command) (*int64, error) {
	expiresAfterText, _ := cmd.Flags().GetString("expires-after") //nolint:errcheck // known flag
	if expiresAfterText == "" {
		return nil, nil
	}

	ms, err := parseTimeFlag(expiresAfterText)
	if err != nil {
		return nil, err
	} else if ms <= 0 {
		return nil, output.NewCLIError(output.ErrValidation, "expires-after must be a positive Unix ms timestamp")
	}
	return &ms, nil
}

func parseOptionalBuilder(cmd *cobra.Command) (*exchange.BuilderInfo, error) {
	hasBuilder, hasBuilderFee := cmd.Flags().Changed("builder"), cmd.Flags().Changed("builder-fee-tenths-bp")
	if hasBuilder != hasBuilderFee {
		return nil, output.NewCLIError(output.ErrValidation, "--builder and --builder-fee-tenths-bp must be provided together")
	}
	if !hasBuilder {
		return nil, nil
	}

	builderAddress, _ := cmd.Flags().GetString("builder")                //nolint:errcheck // known flag
	builderFeeTenthsBP, _ := cmd.Flags().GetInt("builder-fee-tenths-bp") //nolint:errcheck // known flag
	builderAddress = strings.ToLower(strings.TrimSpace(builderAddress))
	if !common.IsHexAddress(builderAddress) {
		return nil, output.NewCLIError(output.ErrValidation, "invalid builder address").
			WithDetails("builder", builderAddress)
	} else if builderFeeTenthsBP < 0 {
		return nil, output.NewCLIError(output.ErrValidation, "builder fee must be non-negative").
			WithDetails("builder_fee_tenths_bp", builderFeeTenthsBP)
	}
	return &exchange.BuilderInfo{B: builderAddress, F: builderFeeTenthsBP}, nil
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func confirmationAccepted(cmd *cobra.Command) bool {
	confirm, _ := cmd.Flags().GetBool("confirm") //nolint:errcheck // known flag
	yes, _ := cmd.Flags().GetBool("yes")         //nolint:errcheck // known flag
	return confirm || yes
}

func mustMarkRequiredFlags(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic("mustMarkRequiredFlags: " + err.Error())
		}
	}
}

func newHelpCommandGroup(use, short, long string, subcommands ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(subcommands...)
	return cmd
}

func resolveAddressFlagUser(cmd *cobra.Command, cfg *config.Config) (string, error) {
	address, _ := cmd.Flags().GetString("address") //nolint:errcheck // known flag
	return info.ResolveUserAddress(address, cfg)
}

func fetchOpenOrders(cmd *cobra.Command, cfg *config.Config, user, dex string) (json.RawMessage, info.OpenOrdersResult, error) {
	raw, err := buildInfoClient(cfg).FrontendOpenOrders(cmd.Context(), user, dex)
	if err != nil {
		return nil, nil, err
	}
	orders, err := info.ParseOpenOrdersResult(raw)
	return raw, orders, err
}

func fetchMids(ctx context.Context, cfg *config.Config, dex string) (json.RawMessage, info.MidsResult, error) {
	raw, err := buildInfoClient(cfg).AllMids(ctx, dex)
	if err != nil {
		return nil, nil, err
	}
	mids, err := info.ParseMidsResult(raw)
	return raw, mids, err
}

func fetchPerpState(ctx context.Context, cfg *config.Config, user, dex string) (json.RawMessage, *info.StateResult, error) {
	raw, err := buildInfoClient(cfg).ClearinghouseState(ctx, user, dex)
	if err != nil {
		return nil, nil, err
	}
	state, err := info.ParseStateResult(raw)
	return raw, state, err
}

func printOKMessage(cmd *cobra.Command, cfg *config.Config, message string) error {
	return printResult(cmd, cfg, mustMarshal(map[string]string{"status": "ok", "message": message}), nil)
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
