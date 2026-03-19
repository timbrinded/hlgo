package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

func isValidCandleInterval(interval string) bool {
	switch interval {
	case "1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "8h", "12h", "1d", "3d", "1w", "1M":
		return true
	default:
		return false
	}
}

func newInfoMidsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mids",
		Short: "Get all mid-market prices",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			dex, _ := cmd.Flags().GetString("dex") //nolint:errcheck // known flag

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.AllMidsRequest{Type: "allMids", Dex: dex}), nil)
			}

			raw, result, err := fetchMids(cmd.Context(), cfg, dex)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	return cmd
}

func newInfoMetaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "Get perp or spot universe metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			ic := buildInfoClient(cfg)
			spot, _ := cmd.Flags().GetBool("spot") //nolint:errcheck // known flag
			dex, _ := cmd.Flags().GetString("dex") //nolint:errcheck // known flag

			if cfg.DryRun {
				reqType := "meta"
				if spot {
					reqType = "spotMeta"
				}
				return printResult(cmd, cfg, mustMarshal(info.MetaRequest{Type: reqType, Dex: dex}), nil)
			}

			var raw []byte
			var err error
			if spot {
				raw, err = ic.SpotMeta(cmd.Context())
			} else {
				raw, err = ic.Meta(cmd.Context(), dex)
			}
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, nil)
		},
	}
	cmd.Flags().Bool("spot", false, "fetch spot metadata instead of perp")
	return cmd
}

func newInfoMetaAndCtxsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta-and-ctxs",
		Short: "Get metadata with asset contexts (mark price, funding, etc.)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			ic := buildInfoClient(cfg)
			spot, _ := cmd.Flags().GetBool("spot") //nolint:errcheck // known flag
			dex, _ := cmd.Flags().GetString("dex") //nolint:errcheck // known flag

			if cfg.DryRun {
				reqType := "metaAndAssetCtxs"
				if spot {
					reqType = "spotMetaAndAssetCtxs"
				}
				return printResult(cmd, cfg, mustMarshal(info.MetaAndCtxsRequest{Type: reqType, Dex: dex}), nil)
			}

			var raw []byte
			var err error
			if spot {
				raw, err = ic.SpotMetaAndAssetCtxs(cmd.Context())
			} else {
				raw, err = ic.MetaAndAssetCtxs(cmd.Context(), dex)
			}
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, nil)
		},
	}
	cmd.Flags().Bool("spot", false, "fetch spot metadata instead of perp")
	return cmd
}

func newInfoBookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "book <coin>",
		Short: "Get L2 order book for a coin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			coin := args[0]
			depth, nSigFigs, mantissaPtr, err := parseBookAggregation(cmd)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.L2BookRequest{Type: "l2Book", Coin: coin, NSigFigs: nSigFigs, Mantissa: mantissaPtr}), nil)
			}

			raw, err := buildInfoClient(cfg).L2Book(cmd.Context(), coin, nSigFigs, mantissaPtr)
			if err != nil {
				return err
			}

			result, err := info.ParseBookResult(raw)
			if err != nil {
				return err
			}

			// Client-side depth truncation.
			if depth > 0 {
				for i := range result.Levels {
					if len(result.Levels[i].Levels) > depth {
						result.Levels[i].Levels = result.Levels[i].Levels[:depth]
					}
				}
			}

			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().Int("sigfigs", 0, "number of significant figures (passed to API)")
	cmd.Flags().Int("mantissa", 0, "l2 aggregation mantissa (allowed only with --sigfigs 5: 1, 2, or 5)")
	cmd.Flags().Int("depth", 0, "max levels per side (client-side truncation)")
	return cmd
}

func parseBookAggregation(cmd *cobra.Command) (depth int, nSigFigs, mantissa *int, err error) {
	sigfigs, _ := cmd.Flags().GetInt("sigfigs")        //nolint:errcheck // known flag
	depth, _ = cmd.Flags().GetInt("depth")             //nolint:errcheck // known flag
	mantissaValue, _ := cmd.Flags().GetInt("mantissa") //nolint:errcheck // known flag

	if cmd.Flags().Changed("sigfigs") {
		nSigFigs = &sigfigs
	}
	if !cmd.Flags().Changed("mantissa") {
		return depth, nSigFigs, nil, nil
	}
	if !cmd.Flags().Changed("sigfigs") || sigfigs != 5 {
		return 0, nil, nil, output.NewCLIError(output.ErrValidation, "mantissa requires --sigfigs 5")
	}
	if mantissaValue != 1 && mantissaValue != 2 && mantissaValue != 5 {
		return 0, nil, nil, output.NewCLIError(output.ErrValidation, "mantissa must be one of 1, 2, or 5").
			WithDetails("mantissa", mantissaValue)
	}
	mantissa = &mantissaValue
	return depth, nSigFigs, mantissa, nil
}

func newInfoTradesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trades <coin>",
		Short: "Get recent trades for a coin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			coin := args[0]

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.RecentTradesRequest{Type: "recentTrades", Coin: coin}), nil)
			}

			raw, err := buildInfoClient(cfg).RecentTrades(cmd.Context(), coin)
			if err != nil {
				return err
			}

			result, err := info.ParseTradesResult(raw)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
}

func newInfoCandlesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "candles <coin> <interval>",
		Short: "Get OHLCV candle data",
		Long: `Fetch candle snapshots for a coin. Valid intervals:
  1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 8h, 12h, 1d, 3d, 1w, 1M`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			coin := args[0]
			interval := args[1]

			if !isValidCandleInterval(interval) {
				return output.NewCLIError(output.ErrValidation, "invalid candle interval: "+interval).
					WithDetails("interval", interval).
					WithDetails("valid", "1m,3m,5m,15m,30m,1h,2h,4h,8h,12h,1d,3d,1w,1M")
			}

			startTime, endTime, err := resolveCandleTimeRange(cmd)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.CandleSnapshotRequest{Type: "candleSnapshot", Req: info.CandleSnapshotReq{Coin: coin, Interval: interval, StartTime: startTime, EndTime: endTime}}), nil)
			}

			raw, err := buildInfoClient(cfg).CandleSnapshot(cmd.Context(), coin, interval, startTime, endTime)
			if err != nil {
				return err
			}

			result, err := info.ParseCandlesResult(raw)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().String("start", "", "start time (Unix ms or ISO 8601)")
	cmd.Flags().String("end", "", "end time (Unix ms or ISO 8601)")
	return cmd
}

func resolveCandleTimeRange(cmd *cobra.Command) (int64, int64, error) {
	startText, _ := cmd.Flags().GetString("start") //nolint:errcheck // known flag
	endText, _ := cmd.Flags().GetString("end")     //nolint:errcheck // known flag

	startTime, err := parseTimeFlag(startText)
	if err != nil {
		return 0, 0, err
	}
	endTime, err := parseTimeFlag(endText)
	if err != nil {
		return 0, 0, err
	}
	if startTime == 0 {
		startTime = time.Now().Add(-24 * time.Hour).UnixMilli()
	}
	if endTime == 0 {
		endTime = time.Now().UnixMilli()
	}
	return startTime, endTime, nil
}
