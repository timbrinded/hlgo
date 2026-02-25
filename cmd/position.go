package cmd

import "github.com/spf13/cobra"

func newPositionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "position",
		Short: "Manage positions: leverage and margin",
		Long: `Set leverage mode (cross/isolated) and multiplier, and update isolated
margin for open positions. Position commands sign with the agent wallet
via the L1 phantom agent path.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}
