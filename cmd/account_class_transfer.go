package cmd

import "github.com/spf13/cobra"

func newAccountClassTransferCmd() *cobra.Command {
	return newUSDClassTransferCmd(
		"class-transfer",
		"Alias of transfer using usdClassTransfer semantics",
		"transfer amount",
		"transfer toward perp class",
		"transfer toward spot class",
	)
}
