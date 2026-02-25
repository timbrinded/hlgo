package cmd

import "github.com/spf13/cobra"

func newOrderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "order",
		Short: "Place, cancel, and manage orders",
		Long: `Submit limit and market orders, cancel by OID or CLOID, modify existing
orders, and batch-place from file. All order commands sign with the agent
wallet via the L1 phantom agent path.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}
