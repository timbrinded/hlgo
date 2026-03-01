package cmd

import (
	"strings"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newPositionMarginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "margin",
		Short: "Adjust isolated margin for a position",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")        //nolint:errcheck // known flag
			side, _ := cmd.Flags().GetString("side")        //nolint:errcheck // known flag
			amountStr, _ := cmd.Flags().GetString("amount") //nolint:errcheck // known flag
			vault, _ := cmd.Flags().GetString("vault")      //nolint:errcheck // known flag

			side = strings.ToLower(side)
			if side != "buy" && side != "sell" {
				return output.NewCLIError(output.ErrValidation, "side must be 'buy' or 'sell'").
					WithDetails("value", side)
			}

			amount, err := decimal.NewFromString(amountStr)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid amount").
					WithDetails("value", amountStr)
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.UpdateIsolatedMargin(cmd.Context(), exchange.UpdateIsolatedMarginInput{
				Coin:      coin,
				IsBuy:     side == "buy",
				Amount:    amount,
				VaultAddr: vault,
				DryRun:    cfg.DryRun,
			})
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, result, nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().String("side", "", "position side: buy or sell")
	cmd.Flags().String("amount", "", "margin amount (positive to add, negative to remove)")
	cmd.Flags().String("vault", "", "vault address")

	for _, required := range []string{"coin", "side", "amount"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}
