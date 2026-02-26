package cmd

import (
	"strings"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newOrderMarketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "Place a market order (IOC with slippage)",
		Long: `Place a market order by fetching the current mid price and applying slippage.
The order is placed as an IOC (immediate-or-cancel) at the slippage-adjusted price.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")             //nolint:errcheck // known flag
			side, _ := cmd.Flags().GetString("side")             //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")          //nolint:errcheck // known flag
			slippagePct, _ := cmd.Flags().GetFloat64("slippage") //nolint:errcheck // known flag
			vault, _ := cmd.Flags().GetString("vault")           //nolint:errcheck // known flag

			side = strings.ToLower(side)
			if side != "buy" && side != "sell" {
				return output.NewCLIError(output.ErrValidation, "side must be 'buy' or 'sell'").
					WithDetails("value", side)
			}

			size, err := decimal.NewFromString(sizeStr)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid size").
					WithDetails("value", sizeStr)
			}

			// Fetch mid price.
			ic := buildInfoClient(cfg)
			raw, err := ic.AllMids(cmd.Context(), cfg.Dex)
			if err != nil {
				return err
			}

			mids, err := info.ParseMidsResult(raw)
			if err != nil {
				return err
			}

			midStr, ok := mids[coin]
			if !ok {
				return output.NewCLIError(output.ErrValidation, "no mid price found for coin: "+coin).
					WithDetails("coin", coin)
			}

			mid, err := decimal.NewFromString(midStr)
			if err != nil {
				return output.NewCLIError(output.ErrAPI, "invalid mid price from API").
					WithDetails("coin", coin).
					WithDetails("value", midStr)
			}

			// Apply slippage: buy → mid * (1 + slippage/100), sell → mid * (1 - slippage/100).
			slippageDecimal := decimal.NewFromFloat(slippagePct).Div(decimal.NewFromInt(100))
			var price decimal.Decimal
			if side == "buy" {
				price = mid.Mul(decimal.NewFromInt(1).Add(slippageDecimal))
			} else {
				price = mid.Mul(decimal.NewFromInt(1).Sub(slippageDecimal))
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.PlaceOrder(cmd.Context(), exchange.PlaceOrderInput{
				Coin:      coin,
				Side:      side,
				Price:     price,
				Size:      size,
				Tif:       "Ioc",
				VaultAddr: vault,
				DryRun:    cfg.DryRun,
			})
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, mustMarshal(result), nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("side", "", "buy or sell")
	cmd.Flags().String("size", "", "order size")
	cmd.Flags().Float64("slippage", 0.5, "slippage percentage (default 0.5%)")
	cmd.Flags().String("vault", "", "vault address")

	for _, required := range []string{"coin", "side", "size"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}
