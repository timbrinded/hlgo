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
			timeoutStr, _ := cmd.Flags().GetString("timeout") //nolint:errcheck // known flag
			clear, _ := cmd.Flags().GetBool("clear")          //nolint:errcheck // known flag

			hasTimeout := cmd.Flags().Changed("timeout")

			if hasTimeout && clear {
				return output.NewCLIError(output.ErrValidation, "--timeout and --clear are mutually exclusive")
			}
			if !hasTimeout && !clear {
				return output.NewCLIError(output.ErrValidation, "one of --timeout or --clear is required")
			}

			var cancelTime *int64
			if hasTimeout {
				d, err := time.ParseDuration(timeoutStr)
				if err != nil {
					return output.NewCLIError(output.ErrValidation, "invalid timeout duration").
						WithDetails("value", timeoutStr).
						WithDetails("hint", "use Go duration format (e.g. 5m, 1h, 30s)")
				}
				if d <= 0 {
					return output.NewCLIError(output.ErrValidation, "timeout must be positive").
						WithDetails("value", timeoutStr)
				}
				ms := time.Now().Add(d).UnixMilli()
				cancelTime = &ms
			}
			// When --clear, cancelTime stays nil (which tells API to remove the schedule).

			exec, err := buildExecutor(cfg)
			if err != nil {
				return err
			}

			result, err := exec.ScheduleCancel(cmd.Context(), exchange.ScheduleCancelInput{
				Time:   cancelTime,
				DryRun: cfg.DryRun,
			})
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
