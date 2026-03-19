package cmd

import "github.com/spf13/cobra"

func newOrderCmd() *cobra.Command {
	return newHelpCommandGroup(
		"order",
		"Place, cancel, and manage orders",
		`Submit limit and market orders, cancel by OID or CLOID, modify existing
orders, and batch-place from file. All order commands sign with the configured
private key via the L1 phantom agent path.`,
		newOrderPlaceCmd(),
		newOrderMarketCmd(),
		newOrderCancelCmd(),
		newOrderCancelAllCmd(),
		newOrderModifyCmd(),
		newOrderBatchCmd(),
		newOrderScheduleCancelCmd(),
	)
}
