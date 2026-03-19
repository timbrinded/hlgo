package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newPositionLeverageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leverage",
		Short: "Set leverage and margin mode for a coin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			coin, _ := cmd.Flags().GetString("coin")      //nolint:errcheck // known flag
			leverage, _ := cmd.Flags().GetInt("leverage") //nolint:errcheck // known flag
			mode, _ := cmd.Flags().GetString("mode")      //nolint:errcheck // known flag

			mode = strings.ToLower(mode)
			if mode != "cross" && mode != "isolated" {
				return output.NewCLIError(output.ErrValidation, "mode must be 'cross' or 'isolated'").
					WithDetails("value", mode)
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			input := exchange.UpdateLeverageInput{Coin: coin, IsCross: mode == "cross", Leverage: leverage, DryRun: cfg.DryRun}
			result, err := exec.UpdateLeverage(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, result, nil)
		},
	}

	cmd.Flags().String("coin", "", "coin name (e.g. BTC, ETH)")
	cmd.Flags().Int("leverage", 0, "leverage multiplier (max is asset-specific, API-enforced)")
	cmd.Flags().String("mode", "cross", "margin mode: cross or isolated")

	mustMarkRequiredFlags(cmd, "coin", "leverage")

	return cmd
}
