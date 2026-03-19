package cmd

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/info"
)

func newInfoStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Get perp clearinghouse state (positions, margins)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			dex, _ := cmd.Flags().GetString("dex") //nolint:errcheck // known flag

			user, err := resolveAddressFlagUser(cmd, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.ClearinghouseStateRequest{Type: "clearinghouseState", User: user, Dex: dex}), nil)
			}

			raw, result, err := fetchPerpState(cmd.Context(), cfg, user, dex)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	return cmd
}

func newInfoSpotStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spot-state",
		Short: "Get spot clearinghouse state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())

			user, err := resolveAddressFlagUser(cmd, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.SpotClearinghouseStateRequest{Type: "spotClearinghouseState", User: user}), nil)
			}

			raw, err := buildInfoClient(cfg).SpotClearinghouseState(cmd.Context(), user)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, nil)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	return cmd
}

func newInfoOpenOrdersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open-orders",
		Short: "Get open orders for a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			dex, _ := cmd.Flags().GetString("dex") //nolint:errcheck // known flag

			user, err := resolveAddressFlagUser(cmd, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.FrontendOpenOrdersRequest{Type: "frontendOpenOrders", User: user, Dex: dex}), nil)
			}

			raw, result, err := fetchOpenOrders(cmd, cfg, user, dex)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	return cmd
}

func newInfoFillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fills",
		Short: "Get fill history for a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())

			user, err := resolveAddressFlagUser(cmd, cfg)
			if err != nil {
				return err
			}

			startStr, _ := cmd.Flags().GetString("start")                  //nolint:errcheck // known flag
			endStr, _ := cmd.Flags().GetString("end")                      //nolint:errcheck // known flag
			aggregateByTime, _ := cmd.Flags().GetBool("aggregate-by-time") //nolint:errcheck // known flag

			var aggregateByTimePtr *bool
			if cmd.Flags().Changed("aggregate-by-time") {
				aggregateByTimePtr = &aggregateByTime
			}

			request := info.UserFillsRequest{Type: "userFills", User: user, AggregateByTime: aggregateByTimePtr}
			if startStr != "" || endStr != "" {
				request.Type = "userFillsByTime"
				if request.StartTime, err = parseTimeFlag(startStr); err != nil {
					return err
				}
				if request.EndTime, err = parseTimeFlag(endStr); err != nil {
					return err
				}
			}
			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(request), nil)
			}

			raw, err := fetchUserFillsRaw(cmd.Context(), buildInfoClient(cfg), user, request)
			if err != nil {
				return err
			}
			result, err := info.ParseFillsResult(raw)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	cmd.Flags().String("start", "", "start time (Unix ms or ISO 8601)")
	cmd.Flags().String("end", "", "end time (Unix ms or ISO 8601)")
	cmd.Flags().Bool("aggregate-by-time", false, "aggregate partial fills by timestamp")
	return cmd
}

func fetchUserFillsRaw(ctx context.Context, ic *info.InfoClient, user string, request info.UserFillsRequest) ([]byte, error) {
	if request.Type == "userFillsByTime" {
		return ic.UserFillsByTime(ctx, user, request.StartTime, request.EndTime, request.AggregateByTime)
	}
	return ic.UserFills(ctx, user, request.AggregateByTime)
}

func newInfoOrderStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order-status <oid>",
		Short: "Get the status of a specific order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())

			user, err := resolveAddressFlagUser(cmd, cfg)
			if err != nil {
				return err
			}

			// Auto-detect: numeric = OID, otherwise = CLOID.
			var oid any
			if numOid, err := strconv.ParseInt(args[0], 10, 64); err == nil {
				oid = numOid
			} else {
				oid = args[0]
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.OrderStatusRequest{Type: "orderStatus", User: user, Oid: oid}), nil)
			}

			raw, err := buildInfoClient(cfg).OrderStatus(cmd.Context(), user, oid)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, nil)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	return cmd
}

func newInfoRateLimitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rate-limit",
		Short: "Get rate limit info for a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())

			user, err := resolveAddressFlagUser(cmd, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.UserRateLimitRequest{Type: "userRateLimit", User: user}), nil)
			}

			raw, err := buildInfoClient(cfg).UserRateLimit(cmd.Context(), user)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, nil)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	return cmd
}
