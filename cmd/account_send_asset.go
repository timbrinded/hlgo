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

func newAccountSendAssetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-asset",
		Short: "Send a spot asset to another address",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			destination, _ := cmd.Flags().GetString("destination") //nolint:errcheck // known flag
			token, _ := cmd.Flags().GetString("token")             //nolint:errcheck // known flag
			amountStr, _ := cmd.Flags().GetString("amount")        //nolint:errcheck // known flag
			confirm, _ := cmd.Flags().GetBool("confirm")           //nolint:errcheck // known flag
			yes, _ := cmd.Flags().GetBool("yes")                   //nolint:errcheck // known flag
			onBehalfOf, _ := cmd.Flags().GetString("on-behalf-of") //nolint:errcheck // known flag

			if err := requireConfirm("send-asset", confirm || yes, cfg.DryRun); err != nil {
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
			if strings.TrimSpace(token) == "" {
				return output.NewCLIError(output.ErrValidation, "token is required")
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

			raw, err := exec.SpotSend(cmd.Context(), exchange.SpotSendInput{
				Destination: strings.ToLower(destination),
				Token:       token,
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
	cmd.Flags().String("token", "", "spot token identifier (e.g. PURR:0x1)")
	cmd.Flags().String("amount", "", "token amount")
	cmd.Flags().String("on-behalf-of", "", "account address to act on behalf of")
	cmd.Flags().Bool("confirm", false, "confirm execution for asset send")
	cmd.Flags().Bool("yes", false, "alias for --confirm")

	for _, required := range []string{"destination", "token", "amount"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}
