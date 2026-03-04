package cmd

import (
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
			addr, _ := cmd.Flags().GetString("address") //nolint:errcheck // known flag //nolint:errcheck // known flag
			dex, _ := cmd.Flags().GetString("dex")      //nolint:errcheck // known flag

			user, err := info.ResolveUserAddress(addr, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.ClearinghouseStateRequest{
					Type: "clearinghouseState", User: user, Dex: dex,
				}), nil)
			}

			ic := buildInfoClient(cfg)
			raw, err := ic.ClearinghouseState(cmd.Context(), user, dex)
			if err != nil {
				return err
			}

			result, err := info.ParseStateResult(raw)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	cmd.Flags().String("dex", "", "HIP-3 perp dex name")
	return cmd
}

func newInfoSpotStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spot-state",
		Short: "Get spot clearinghouse state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			addr, _ := cmd.Flags().GetString("address") //nolint:errcheck // known flag

			user, err := info.ResolveUserAddress(addr, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.SpotClearinghouseStateRequest{
					Type: "spotClearinghouseState", User: user,
				}), nil)
			}

			ic := buildInfoClient(cfg)
			raw, err := ic.SpotClearinghouseState(cmd.Context(), user)
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
			addr, _ := cmd.Flags().GetString("address") //nolint:errcheck // known flag
			dex, _ := cmd.Flags().GetString("dex")      //nolint:errcheck // known flag

			user, err := info.ResolveUserAddress(addr, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.FrontendOpenOrdersRequest{
					Type: "frontendOpenOrders", User: user, Dex: dex,
				}), nil)
			}

			ic := buildInfoClient(cfg)
			raw, err := ic.FrontendOpenOrders(cmd.Context(), user, dex)
			if err != nil {
				return err
			}

			result, err := info.ParseOpenOrdersResult(raw)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	cmd.Flags().String("dex", "", "HIP-3 perp dex name")
	return cmd
}

func newInfoFillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fills",
		Short: "Get fill history for a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			addr, _ := cmd.Flags().GetString("address") //nolint:errcheck // known flag

			user, err := info.ResolveUserAddress(addr, cfg)
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

			ic := buildInfoClient(cfg)

			// Use time-based endpoint when start or end is specified.
			if startStr != "" || endStr != "" {
				startTime, err := parseTimeFlag(startStr)
				if err != nil {
					return err
				}
				endTime, err := parseTimeFlag(endStr)
				if err != nil {
					return err
				}

				if cfg.DryRun {
					return printResult(cmd, cfg, mustMarshal(info.UserFillsRequest{
						Type:            "userFillsByTime",
						User:            user,
						StartTime:       startTime,
						EndTime:         endTime,
						AggregateByTime: aggregateByTimePtr,
					}), nil)
				}

				raw, err := ic.UserFillsByTime(cmd.Context(), user, startTime, endTime, aggregateByTimePtr)
				if err != nil {
					return err
				}

				result, err := info.ParseFillsResult(raw)
				if err != nil {
					return err
				}
				return printResult(cmd, cfg, raw, result)
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.UserFillsRequest{
					Type:            "userFills",
					User:            user,
					AggregateByTime: aggregateByTimePtr,
				}), nil)
			}

			raw, err := ic.UserFills(cmd.Context(), user, aggregateByTimePtr)
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

func newInfoOrderStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order-status <oid>",
		Short: "Get the status of a specific order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			addr, _ := cmd.Flags().GetString("address") //nolint:errcheck // known flag

			user, err := info.ResolveUserAddress(addr, cfg)
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
				return printResult(cmd, cfg, mustMarshal(info.OrderStatusRequest{
					Type: "orderStatus", User: user, Oid: oid,
				}), nil)
			}

			ic := buildInfoClient(cfg)
			raw, err := ic.OrderStatus(cmd.Context(), user, oid)
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
			addr, _ := cmd.Flags().GetString("address") //nolint:errcheck // known flag

			user, err := info.ResolveUserAddress(addr, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.UserRateLimitRequest{
					Type: "userRateLimit", User: user,
				}), nil)
			}

			ic := buildInfoClient(cfg)
			raw, err := ic.UserRateLimit(cmd.Context(), user)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, nil)
		},
	}
	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	return cmd
}
