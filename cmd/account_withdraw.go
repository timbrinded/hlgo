package cmd

import (
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
)

func newAccountWithdrawCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw",
		Short: "Withdraw USDC to an Arbitrum address",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			destination, _ := cmd.Flags().GetString("destination") //nolint:errcheck // known flag
			amountStr, _ := cmd.Flags().GetString("amount")        //nolint:errcheck // known flag

			if err := requireConfirm("withdraw", confirmationAccepted(cmd), cfg.DryRun); err != nil {
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

			input := exchange.Withdraw3Input{Destination: destination, Amount: amount, DryRun: cfg.DryRun}
			raw, err := exec.Withdraw3(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, raw, nil)
		},
	}

	cmd.Flags().String("destination", "", "destination EVM address")
	cmd.Flags().String("amount", "", "USDC amount")
	cmd.Flags().Bool("confirm", false, "confirm execution for withdrawal")
	cmd.Flags().Bool("yes", false, "alias for --confirm")

	mustMarkRequiredFlags(cmd, "destination", "amount")

	return cmd
}
