package cmd

import "github.com/spf13/cobra"

func newAccountCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "account",
		Short: "Account transfers, withdrawals, and agent management",
		Long: `Transfer USDC between spot and perp, withdraw to Arbitrum, manage agent
wallet approvals, and perform cross-account transfers. Account commands
sign with the master wallet via the user-signed path (chain ID 42161).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}
