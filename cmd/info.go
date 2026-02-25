package cmd

import "github.com/spf13/cobra"

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Read market and account data from Hyperliquid",
		Long: `Query the Hyperliquid Info API for market data, order books, trades,
candles, funding rates, account state, and open orders. All info commands
are read-only and require no wallet configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}
