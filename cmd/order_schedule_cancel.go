package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/exchange"
	"github.com/timbrinded/hlgo/pkg/output"
)

func newOrderScheduleCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule-cancel",
		Short: "Set or clear a dead man's switch for order cancellation",
		Long: `Set a timeout after which all open orders are automatically cancelled,
or clear an existing schedule. Exactly one of --timeout or --clear must be provided.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			clear, _ := cmd.Flags().GetBool("clear") //nolint:errcheck // known flag

			hasTimeout := cmd.Flags().Changed("timeout")
			if hasTimeout && clear {
				return output.NewCLIError(output.ErrValidation, "--timeout and --clear are mutually exclusive")
			}
			if !hasTimeout && !clear {
				return output.NewCLIError(output.ErrValidation, "one of --timeout or --clear is required")
			}

			cancelTime, err := parseScheduleCancelTime(cmd, hasTimeout)
			if err != nil {
				return err
			}

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			input := exchange.ScheduleCancelInput{Time: cancelTime, DryRun: cfg.DryRun}
			result, err := exec.ScheduleCancel(cmd.Context(), input)
			if err != nil {
				return err
			}

			return printResult(cmd, cfg, result, nil)
		},
	}

	cmd.Flags().String("timeout", "", "cancellation timeout (Go duration, e.g. 5m, 1h)")
	cmd.Flags().Bool("clear", false, "clear existing schedule")

	return cmd
}

func parseScheduleCancelTime(cmd *cobra.Command, hasTimeout bool) (*int64, error) {
	if !hasTimeout {
		return nil, nil
	}

	timeoutText, _ := cmd.Flags().GetString("timeout") //nolint:errcheck // known flag
	timeout, err := time.ParseDuration(timeoutText)
	if err != nil {
		return nil, output.NewCLIError(output.ErrValidation, "invalid timeout duration").
			WithDetails("value", timeoutText).
			WithDetails("hint", "use Go duration format (e.g. 5m, 1h, 30s)")
	}
	if timeout <= 0 {
		return nil, output.NewCLIError(output.ErrValidation, "timeout must be positive").
			WithDetails("value", timeoutText)
	}
	if timeout < 5*time.Second {
		return nil, output.NewCLIError(output.ErrValidation, "timeout must be at least 5 seconds").
			WithDetails("value", timeoutText)
	}

	cancelTime := time.Now().Add(timeout).UnixMilli()
	return &cancelTime, nil
}
