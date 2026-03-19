package cmd

import "github.com/spf13/cobra"

func newAccountCmd() *cobra.Command {
	return newHelpCommandGroup(
		"account",
		"Account transfers, withdrawals, and agent management",
		`Transfer USDC between spot and perp, withdraw to Arbitrum, manage agent
wallet approvals, and perform cross-account transfers. Account commands
sign with the configured private key via the user-signed path (chain ID 421614).`,
		newAccountTransferCmd(),
		newAccountWithdrawCmd(),
		newAccountClassTransferCmd(),
		newAccountSendAssetCmd(),
		newAccountApproveAgentCmd(),
		newAccountSetAbstractionCmd(),
	)
}
