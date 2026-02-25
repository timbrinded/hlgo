package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCmd creates the version command. The version string is captured by
// closure — no package-level variable needed.
func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the hlgo version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
			return err
		},
	}
}
