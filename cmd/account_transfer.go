package cmd

import (
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newAccountTransferCmd() *cobra.Command {
	return newUSDClassTransferCmd(
		"transfer",
		"Transfer USDC between spot and perp classes",
		"USDC amount",
		"transfer from spot to perp",
		"transfer from perp to spot",
	)
}

func newUSDClassTransferCmd(use, short, amountHelp, toPerpHelp, toSpotHelp string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			amountStr, _ := cmd.Flags().GetString("amount") //nolint:errcheck // known flag
			toPerp, _ := cmd.Flags().GetBool("to-perp")     //nolint:errcheck // known flag
			toSpot, _ := cmd.Flags().GetBool("to-spot")     //nolint:errcheck // known flag

			if toPerp == toSpot {
				return output.NewCLIError(output.ErrValidation, "exactly one of --to-perp or --to-spot is required")
			}

			amount, err := parseDecimalField("amount", amountStr)
			if err != nil {
				return err
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			input := exchange.USDClassTransferInput{Amount: amount, ToPerp: toPerp, DryRun: cfg.DryRun}
			raw, err := exec.USDClassTransfer(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, raw, nil)
		},
	}

	cmd.Flags().String("amount", "", amountHelp)
	cmd.Flags().Bool("to-perp", false, toPerpHelp)
	cmd.Flags().Bool("to-spot", false, toSpotHelp)
	mustMarkRequiredFlags(cmd, "amount")

	return cmd
}
