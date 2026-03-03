package cmd

import "github.com/spf13/cobra"

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "High-level agent workflows",
		Long: `Compose multiple info and exchange operations into agent-native workflows
for state snapshots, PnL analysis, and bracket order execution.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newAgentSnapshotCmd(),
		newAgentPnlCmd(),
		newAgentBracketCmd(),
	)

	return cmd
}
