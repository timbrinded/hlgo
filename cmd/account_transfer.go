package cmd

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newAccountTransferCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "Transfer USDC between spot and perp classes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			amountStr, _ := cmd.Flags().GetString("amount")        //nolint:errcheck // known flag
			toPerp, _ := cmd.Flags().GetBool("to-perp")            //nolint:errcheck // known flag
			toSpot, _ := cmd.Flags().GetBool("to-spot")            //nolint:errcheck // known flag
			onBehalfOf, _ := cmd.Flags().GetString("on-behalf-of") //nolint:errcheck // known flag

			if toPerp == toSpot {
				return output.NewCLIError(output.ErrValidation, "exactly one of --to-perp or --to-spot is required")
			}

			amount, err := decimal.NewFromString(amountStr)
			if err != nil {
				return output.NewCLIError(output.ErrValidation, "invalid amount").
					WithDetails("value", amountStr)
			}
			if onBehalfOf != "" && !common.IsHexAddress(onBehalfOf) {
				return output.NewCLIError(output.ErrValidation, "invalid on-behalf-of address").
					WithDetails("on_behalf_of", onBehalfOf)
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			raw, err := exec.USDClassTransfer(cmd.Context(), exchange.USDClassTransferInput{
				Amount:     amount,
				ToPerp:     toPerp,
				OnBehalfOf: onBehalfOf,
				DryRun:     cfg.DryRun,
			})
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, raw, nil)
		},
	}

	cmd.Flags().String("amount", "", "USDC amount")
	cmd.Flags().Bool("to-perp", false, "transfer from spot to perp")
	cmd.Flags().Bool("to-spot", false, "transfer from perp to spot")
	cmd.Flags().String("on-behalf-of", "", "account address to act on behalf of")
	//nolint:errcheck // MarkFlagRequired on known flags never fails
	cmd.MarkFlagRequired("amount")

	return cmd
}
