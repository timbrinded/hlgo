package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// newVersionCmd creates the version command. The version string is captured by
// closure — no package-level variable needed.
func newVersionCmd(version, commit, date string) *cobra.Command {
	type versionPayload struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Date    string `json:"date"`
	}

	return &cobra.Command{
		Use:         "version",
		Short:       "Print the hlgo version",
		Annotations: map[string]string{"skipConfig": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(versionPayload{
				Version: version,
				Commit:  commit,
				Date:    date,
			})
		},
	}
}
