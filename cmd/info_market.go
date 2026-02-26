package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

// validCandleIntervals is the set of intervals accepted by the candle API.
var validCandleIntervals = map[string]bool{
	"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
	"1h": true, "2h": true, "4h": true, "8h": true, "12h": true,
	"1d": true, "3d": true, "1w": true, "1M": true,
}

func newInfoMidsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mids",
		Short: "Get all mid-market prices",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			ic := buildInfoClient(cfg)
			dex, _ := cmd.Flags().GetString("dex") //nolint:errcheck // known flag

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.AllMidsRequest{Type: "allMids", Dex: dex}), nil)
			}

			raw, err := ic.AllMids(cmd.Context(), dex)
			if err != nil {
				return err
			}

			result, err := info.ParseMidsResult(raw)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().String("dex", "", "HIP-3 perp dex name")
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
	cmd.Flags().String("dex", "", "HIP-3 perp dex name")
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
	cmd.Flags().String("dex", "", "HIP-3 perp dex name")
	return cmd
}

func newInfoBookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "book <coin>",
		Short: "Get L2 order book for a coin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			ic := buildInfoClient(cfg)
			coin := args[0]
			sigfigs, _ := cmd.Flags().GetInt("sigfigs") //nolint:errcheck // known flag
			depth, _ := cmd.Flags().GetInt("depth")     //nolint:errcheck // known flag

			var nSigFigs *int
			if cmd.Flags().Changed("sigfigs") {
				nSigFigs = &sigfigs
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.L2BookRequest{Type: "l2Book", Coin: coin, NSigFigs: nSigFigs}), nil)
			}

			raw, err := ic.L2Book(cmd.Context(), coin, nSigFigs)
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
	cmd.Flags().Int("depth", 0, "max levels per side (client-side truncation)")
	return cmd
}

func newInfoTradesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trades <coin>",
		Short: "Get recent trades for a coin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			ic := buildInfoClient(cfg)
			coin := args[0]

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.RecentTradesRequest{Type: "recentTrades", Coin: coin}), nil)
			}

			raw, err := ic.RecentTrades(cmd.Context(), coin)
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

			if !validCandleIntervals[interval] {
				return output.NewCLIError(output.ErrValidation, "invalid candle interval: "+interval).
					WithDetails("interval", interval).
					WithDetails("valid", "1m,3m,5m,15m,30m,1h,2h,4h,8h,12h,1d,3d,1w,1M")
			}

			ic := buildInfoClient(cfg)
			startStr, _ := cmd.Flags().GetString("start") //nolint:errcheck // known flag
			endStr, _ := cmd.Flags().GetString("end")     //nolint:errcheck // known flag

			startTime, err := parseTimeFlag(startStr)
			if err != nil {
				return err
			}
			endTime, err := parseTimeFlag(endStr)
			if err != nil {
				return err
			}

			// Default: last 24 hours.
			if startTime == 0 {
				startTime = time.Now().Add(-24 * time.Hour).UnixMilli()
			}
			if endTime == 0 {
				endTime = time.Now().UnixMilli()
			}

			if cfg.DryRun {
				req := info.CandleSnapshotRequest{
					Type: "candleSnapshot", Coin: coin, Interval: interval,
					StartTime: startTime, EndTime: endTime,
				}
				return printResult(cmd, cfg, mustMarshal(req), nil)
			}

			raw, err := ic.CandleSnapshot(cmd.Context(), coin, interval, startTime, endTime)
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
