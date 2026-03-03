package cmd

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newAccountWithdrawCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "withdraw",
		Short: "Withdraw USDC to an Arbitrum address",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			destination, _ := cmd.Flags().GetString("destination") //nolint:errcheck // known flag
			amountStr, _ := cmd.Flags().GetString("amount")        //nolint:errcheck // known flag
			confirm, _ := cmd.Flags().GetBool("confirm")           //nolint:errcheck // known flag
			yes, _ := cmd.Flags().GetBool("yes")                   //nolint:errcheck // known flag
			onBehalfOf, _ := cmd.Flags().GetString("on-behalf-of") //nolint:errcheck // known flag

			if err := requireConfirm("withdraw", confirm || yes, cfg.DryRun); err != nil {
				return err
			}

			if !common.IsHexAddress(destination) {
				return output.NewCLIError(output.ErrValidation, "invalid destination address").
					WithDetails("destination", destination)
			}
			if onBehalfOf != "" && !common.IsHexAddress(onBehalfOf) {
				return output.NewCLIError(output.ErrValidation, "invalid on-behalf-of address").
					WithDetails("on_behalf_of", onBehalfOf)
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

			raw, err := exec.Withdraw3(cmd.Context(), exchange.Withdraw3Input{
				Destination: strings.ToLower(destination),
				Amount:      amount,
				OnBehalfOf:  onBehalfOf,
				DryRun:      cfg.DryRun,
			})
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, raw, nil)
		},
	}

	cmd.Flags().String("destination", "", "destination EVM address")
	cmd.Flags().String("amount", "", "USDC amount")
	cmd.Flags().String("on-behalf-of", "", "account address to act on behalf of")
	cmd.Flags().Bool("confirm", false, "confirm execution for withdrawal")
	cmd.Flags().Bool("yes", false, "alias for --confirm")

	for _, required := range []string{"destination", "amount"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}
