package cmd

import "github.com/spf13/cobra"

func newAgentCmd() *cobra.Command {
	return newHelpCommandGroup(
		"agent",
		"High-level agent workflows",
		`Compose multiple info and exchange operations into agent-native workflows
for state snapshots, PnL analysis, and bracket order execution.`,
		newAgentSnapshotCmd(),
		newAgentPnlCmd(),
		newAgentBracketCmd(),
	)
}
