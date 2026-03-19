package cmd

import "github.com/spf13/cobra"

func newInfoCmd() *cobra.Command {
	return newHelpCommandGroup(
		"info",
		"Read market and account data from Hyperliquid",
		`Query the Hyperliquid Info API for market data, order books, trades,
candles, funding rates, account state, and open orders. All info commands
are read-only and require no wallet configuration.`,
		newInfoLookupCmd(),
		newInfoMidsCmd(),
		newInfoMetaCmd(),
		newInfoMetaAndCtxsCmd(),
		newInfoBookCmd(),
		newInfoTradesCmd(),
		newInfoCandlesCmd(),
		newInfoStateCmd(),
		newInfoSpotStateCmd(),
		newInfoOpenOrdersCmd(),
		newInfoFillsCmd(),
		newInfoOrderStatusCmd(),
		newInfoRateLimitCmd(),
		newInfoFundingCmd(),
		newInfoPerpDexsCmd(),
	)
}
