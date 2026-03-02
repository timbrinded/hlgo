package cmd

import (
	"github.com/ethereum/go-ethereum/common"
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
			vault, _ := cmd.Flags().GetString("vault")    //nolint:errcheck // known flag

			if mode != "cross" && mode != "isolated" {
				return output.NewCLIError(output.ErrValidation, "mode must be 'cross' or 'isolated'").
					WithDetails("value", mode)
			}

			if leverage < 1 {
				return output.NewCLIError(output.ErrValidation, "leverage must be at least 1").
					WithDetails("value", leverage)
			}

			if vault != "" && !common.IsHexAddress(vault) {
				return output.NewCLIError(output.ErrValidation, "invalid vault address").
					WithDetails("vault", vault)
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.UpdateLeverage(cmd.Context(), exchange.UpdateLeverageInput{
				Coin:      coin,
				IsCross:   mode == "cross",
				Leverage:  leverage,
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
	cmd.Flags().Int("leverage", 0, "leverage multiplier (max is asset-specific, API-enforced)")
	cmd.Flags().String("mode", "cross", "margin mode: cross or isolated")
	cmd.Flags().String("vault", "", "vault address")

	for _, required := range []string{"coin", "leverage"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}
