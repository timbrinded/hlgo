package cmd

import (
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newAgentBracketCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bracket",
		Short: "Place entry + TP + SL in one grouped order action",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")      //nolint:errcheck // known flag
			sideFlag, _ := cmd.Flags().GetString("side")  //nolint:errcheck // known flag
			priceStr, _ := cmd.Flags().GetString("price") //nolint:errcheck // known flag
			sizeStr, _ := cmd.Flags().GetString("size")   //nolint:errcheck // known flag
			tpStr, _ := cmd.Flags().GetString("tp")       //nolint:errcheck // known flag
			slStr, _ := cmd.Flags().GetString("sl")       //nolint:errcheck // known flag
			tifFlag, _ := cmd.Flags().GetString("tif")    //nolint:errcheck // known flag
			cloidStr, _ := cmd.Flags().GetString("cloid") //nolint:errcheck // known flag

			input, err := buildPlaceOrderInput(cmd, coin, sideFlag, priceStr, sizeStr, tifFlag, false, cloidStr, cfg.DryRun)
			if err != nil {
				return err
			}
			input.TpTrigger, input.SlTrigger, err = parseBracketTriggers(input.Side, input.Price, tpStr, slStr)
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
	cmd.Flags().String("price", "", "entry limit price")
	cmd.Flags().String("size", "", "order size")
	cmd.Flags().String("tp", "", "take-profit trigger price")
	cmd.Flags().String("sl", "", "stop-loss trigger price")
	cmd.Flags().String("tif", "gtc", "time in force: gtc, ioc, alo")
	cmd.Flags().String("cloid", "", "client order ID")
	cmd.Flags().String("builder", "", "builder address for optional builder fee routing")
	cmd.Flags().Int("builder-fee-tenths-bp", 0, "builder fee in tenths of a basis point (requires --builder)")
	cmd.Flags().String("expires-after", "", "expiry timestamp (Unix ms or ISO 8601)")

	mustMarkRequiredFlags(cmd, "coin", "side", "price", "size", "tp", "sl")

	return cmd
}

func parseBracketTriggers(side string, price decimal.Decimal, tpStr, slStr string) (*string, *string, error) {
	tp, err := parseDecimalField("tp", tpStr)
	if err != nil {
		return nil, nil, err
	}
	sl, err := parseDecimalField("sl", slStr)
	if err != nil {
		return nil, nil, err
	}
	if side == "buy" {
		if !tp.GreaterThan(price) {
			return nil, nil, output.NewCLIError(output.ErrValidation, "for buy brackets, tp must be greater than entry price").WithDetails("price", price.String()).WithDetails("tp", tp.String())
		}
		if !sl.LessThan(price) {
			return nil, nil, output.NewCLIError(output.ErrValidation, "for buy brackets, sl must be less than entry price").WithDetails("price", price.String()).WithDetails("sl", sl.String())
		}
	} else {
		if !tp.LessThan(price) {
			return nil, nil, output.NewCLIError(output.ErrValidation, "for sell brackets, tp must be less than entry price").WithDetails("price", price.String()).WithDetails("tp", tp.String())
		}
		if !sl.GreaterThan(price) {
			return nil, nil, output.NewCLIError(output.ErrValidation, "for sell brackets, sl must be greater than entry price").WithDetails("price", price.String()).WithDetails("sl", sl.String())
		}
	}
	return stringPointer(tp.String()), stringPointer(sl.String()), nil
}
