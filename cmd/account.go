package cmd

import "github.com/spf13/cobra"

func newAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Account transfers, withdrawals, and agent management",
		Long: `Transfer USDC between spot and perp, withdraw to Arbitrum, manage agent
wallet approvals, and perform cross-account transfers. Account commands
sign with the master wallet via the user-signed path (chain ID 421614).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newAccountTransferCmd(),
		newAccountWithdrawCmd(),
		newAccountClassTransferCmd(),
		newAccountSendAssetCmd(),
		newAccountApproveAgentCmd(),
		newAccountSetAbstractionCmd(),
	)

	return cmd
}
