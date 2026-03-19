package cmd

import (
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
)

func buildPlaceOrderInput(cmd *cobra.Command, coin, sideFlag, priceStr, sizeStr, tifFlag string, reduce bool, cloidStr string, dryRun bool) (exchange.PlaceOrderInput, error) {
	var err error
	input := exchange.PlaceOrderInput{Coin: coin, ReduceOnly: reduce, Cloid: stringPointer(cloidStr), DryRun: dryRun}
	input.Side, err = parseOrderSide(sideFlag)
	if err != nil {
		return exchange.PlaceOrderInput{}, err
	}
	input.Tif, err = parseOrderTIF(tifFlag)
	if err != nil {
		return exchange.PlaceOrderInput{}, err
	}
	input.Price, err = parseDecimalField("price", priceStr)
	if err != nil {
		return exchange.PlaceOrderInput{}, err
	}
	input.Size, err = parseDecimalField("size", sizeStr)
	if err != nil {
		return exchange.PlaceOrderInput{}, err
	}
	input.Builder, err = parseOptionalBuilder(cmd)
	if err != nil {
		return exchange.PlaceOrderInput{}, err
	}
	input.ExpiresAfter, err = parseOptionalExpiresAfter(cmd)
	if err != nil {
		return exchange.PlaceOrderInput{}, err
	}
	return input, nil
}

func newOrderPlaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place a limit order",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")      //nolint:errcheck // known flag
			sideFlag, _ := cmd.Flags().GetString("side")  //nolint:errcheck // known flag
			priceStr, _ := cmd.Flags().GetString("price") //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")   //nolint:errcheck // known flag
			tifFlag, _ := cmd.Flags().GetString("tif")    //nolint:errcheck // known flag
			reduce, _ := cmd.Flags().GetBool("reduce")    //nolint:errcheck // known flag
			cloidStr, _ := cmd.Flags().GetString("cloid") //nolint:errcheck // known flag

			input, err := buildPlaceOrderInput(cmd, coin, sideFlag, priceStr, sizeStr, tifFlag, reduce, cloidStr, cfg.DryRun)
			if err != nil {
				return err
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.PlaceOrder(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, mustMarshal(result), nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("side", "", "buy or sell")
	cmd.Flags().String("price", "", "limit price")
	cmd.Flags().String("size", "", "order size")
	cmd.Flags().String("tif", "gtc", "time in force: gtc, ioc, alo")
	cmd.Flags().Bool("reduce", false, "reduce-only order")
	cmd.Flags().String("cloid", "", "client order ID")
	cmd.Flags().String("builder", "", "builder address for optional builder fee routing")
	cmd.Flags().Int("builder-fee-tenths-bp", 0, "builder fee in tenths of a basis point (requires --builder)")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	mustMarkRequiredFlags(cmd, "coin", "side", "price", "size")

	return cmd
}
