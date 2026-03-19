package cmd

import (
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
)

func buildPlaceMarketOrderInput(cmd *cobra.Command, cfg *config.Config, coin, side, sizeStr, slippageStr string) (exchange.PlaceMarketOrderInput, error) {
	var err error
	input := exchange.PlaceMarketOrderInput{Coin: coin, Side: side, DryRun: cfg.DryRun}
	input.Builder, err = parseOptionalBuilder(cmd)
	if err != nil {
		return exchange.PlaceMarketOrderInput{}, err
	}
	input.ExpiresAfter, err = parseOptionalExpiresAfter(cmd)
	if err != nil {
		return exchange.PlaceMarketOrderInput{}, err
	}
	input.Size, err = parseDecimalField("size", sizeStr)
	if err != nil {
		return exchange.PlaceMarketOrderInput{}, err
	}
	input.SlippagePercent, err = parseDecimalField("slippage", slippageStr)
	if err != nil {
		return exchange.PlaceMarketOrderInput{}, err
	}
	return input, nil
}

func newOrderMarketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "Place a market order (IOC with slippage)",
		Long: `Place a market order by fetching the current mid price and applying slippage.
The order is placed as an IOC (immediate-or-cancel) at the slippage-adjusted price.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")            //nolint:errcheck // known flag
			side, _ := cmd.Flags().GetString("side")            //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")         //nolint:errcheck // known flag
			slippageStr, _ := cmd.Flags().GetString("slippage") //nolint:errcheck // known flag

			input, err := buildPlaceMarketOrderInput(cmd, cfg, coin, side, sizeStr, slippageStr)
			if err != nil {
				return err
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.PlaceMarketOrder(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, mustMarshal(result), nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("side", "", "buy or sell")
	cmd.Flags().String("size", "", "order size")
	cmd.Flags().String("slippage", "0.5", "slippage percentage (default 0.5%)")
	cmd.Flags().String("builder", "", "builder address for optional builder fee routing")
	cmd.Flags().Int("builder-fee-tenths-bp", 0, "builder fee in tenths of a basis point (requires --builder)")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	mustMarkRequiredFlags(cmd, "coin", "side", "size")

	return cmd
}
