package cmd

import (
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
)

func newAccountSendAssetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-asset",
		Short: "Send a spot asset to another address",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			destination, _ := cmd.Flags().GetString("destination") //nolint:errcheck // known flag
			token, _ := cmd.Flags().GetString("token")             //nolint:errcheck // known flag
			amountStr, _ := cmd.Flags().GetString("amount")        //nolint:errcheck // known flag

			if err := requireConfirm("send-asset", confirmationAccepted(cmd), cfg.DryRun); err != nil {
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

			input := exchange.SpotSendInput{Destination: destination, Token: token, Amount: amount, DryRun: cfg.DryRun}
			raw, err := exec.SpotSend(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, raw, nil)
		},
	}

	cmd.Flags().String("destination", "", "destination EVM address")
	cmd.Flags().String("token", "", "spot token identifier (e.g. PURR:0x1)")
	cmd.Flags().String("amount", "", "token amount")
	cmd.Flags().Bool("confirm", false, "confirm execution for asset send")
	cmd.Flags().Bool("yes", false, "alias for --confirm")

	mustMarkRequiredFlags(cmd, "destination", "token", "amount")

	return cmd
}
