package cmd

import "github.com/spf13/cobra"

func newPositionCmd() *cobra.Command {
	return newHelpCommandGroup(
		"position",
		"Manage positions: leverage and margin",
		`Set leverage mode (cross/isolated) and multiplier, and update isolated
margin for open positions. Position commands sign with the configured private
key via the L1 phantom agent path.`,
		newPositionLeverageCmd(),
		newPositionMarginCmd(),
	)
}
