package cmd

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

var allowedAbstractions = map[string]struct{}{
	"unifiedAccount":  {},
	"portfolioMargin": {},
	"disabled":        {},
}

func newAccountSetAbstractionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-abstraction",
		Short: "Set user abstraction mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			user, _ := cmd.Flags().GetString("user")               //nolint:errcheck // known flag
			abstraction, _ := cmd.Flags().GetString("abstraction") //nolint:errcheck // known flag

			if !common.IsHexAddress(user) {
				return output.NewCLIError(output.ErrValidation, "invalid user address").
					WithDetails("user", user)
			}
			abstraction = strings.TrimSpace(abstraction)
			if abstraction == "" {
				return output.NewCLIError(output.ErrValidation, "abstraction is required")
			}
			if _, ok := allowedAbstractions[abstraction]; !ok {
				return output.NewCLIError(output.ErrValidation, "unsupported abstraction value").
					WithDetails("value", abstraction).
					WithDetails("allowed", []string{"unifiedAccount", "portfolioMargin", "disabled"})
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			raw, err := exec.UserSetAbstraction(cmd.Context(), exchange.UserSetAbstractionInput{
				User:        strings.ToLower(user),
				Abstraction: abstraction,
				DryRun:      cfg.DryRun,
			})
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, raw, nil)
		},
	}

	cmd.Flags().String("user", "", "user address")
	cmd.Flags().String("abstraction", "", "abstraction string")

	for _, required := range []string{"user", "abstraction"} {
		//nolint:errcheck // MarkFlagRequired on known flags never fails
		cmd.MarkFlagRequired(required)
	}

	return cmd
}
