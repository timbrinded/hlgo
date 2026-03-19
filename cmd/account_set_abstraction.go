package cmd

import (
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
)

func newAccountSetAbstractionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-abstraction",
		Short: "Set user abstraction mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			user, _ := cmd.Flags().GetString("user")               //nolint:errcheck // known flag
			abstraction, _ := cmd.Flags().GetString("abstraction") //nolint:errcheck // known flag

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			input := exchange.UserSetAbstractionInput{User: user, Abstraction: abstraction, DryRun: cfg.DryRun}
			raw, err := exec.UserSetAbstraction(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, raw, nil)
		},
	}

	cmd.Flags().String("user", "", "user address")
	cmd.Flags().String("abstraction", "", "abstraction string")

	mustMarkRequiredFlags(cmd, "user", "abstraction")

	return cmd
}
