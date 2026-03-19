package cmd

import (
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
)

func newPositionMarginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "margin",
		Short: "Adjust isolated margin for a position",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")        //nolint:errcheck // known flag
			sideFlag, _ := cmd.Flags().GetString("side")    //nolint:errcheck // known flag
			amountStr, _ := cmd.Flags().GetString("amount") //nolint:errcheck // known flag

			side, err := parseOrderSide(sideFlag)
			if err != nil {
				return err
			}

			amount, err := parseDecimalField("amount", amountStr)
			if err != nil {
				return err
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			input := exchange.UpdateIsolatedMarginInput{Coin: coin, IsBuy: side == "buy", Amount: amount, DryRun: cfg.DryRun}
			result, err := exec.UpdateIsolatedMargin(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, result, nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("side", "", "position side: buy or sell")
	cmd.Flags().String("amount", "", "margin amount (positive to add, negative to remove)")

	mustMarkRequiredFlags(cmd, "coin", "side", "amount")

	return cmd
}
