package cmd

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "config",
		Short:       "Configure hlgo settings and credentials",
		Annotations: map[string]string{"skipConfig": "true"},
		Long: `Initialize configuration, display resolved settings (with key redaction),
and test wallet connectivity and agent approval status. Configuration is
stored at ~/.hlgo/config.yaml by default.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}
