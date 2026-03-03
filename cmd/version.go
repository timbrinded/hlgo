package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// BuildInfo holds version metadata injected at build time via ldflags.
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// newVersionCmd creates the version command. Build info is captured by
// closure — no package-level variable needed.
func newVersionCmd(info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:         "version",
		Short:       "Print the hlgo version",
		Annotations: map[string]string{"skipConfig": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
		},
	}
}
