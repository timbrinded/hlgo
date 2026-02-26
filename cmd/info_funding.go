package cmd

import (
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/info"
)

func newInfoFundingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "funding <coin>",
		Short: "Get funding rate history or predicted rates",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			coin := args[0]
			predicted, _ := cmd.Flags().GetBool("predicted") //nolint:errcheck // known flag

			ic := buildInfoClient(cfg)

			if predicted {
				if cfg.DryRun {
					return printResult(cmd, cfg, mustMarshal(info.PredictedFundingsRequest{
						Type: "predictedFundings",
					}), nil)
				}

				raw, err := ic.PredictedFundings(cmd.Context())
				if err != nil {
					return err
				}

				result, err := info.ParsePredictedFundingsResult(raw)
				if err != nil {
					return err
				}
				return printResult(cmd, cfg, raw, result)
			}

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

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.FundingHistoryRequest{
					Type: "fundingHistory", Coin: coin, StartTime: startTime, EndTime: endTime,
				}), nil)
			}

			raw, err := ic.FundingHistory(cmd.Context(), coin, startTime, endTime)
			if err != nil {
				return err
			}

			result, err := info.ParseFundingResult(raw)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
	cmd.Flags().Bool("predicted", false, "fetch predicted funding rates instead of history")
	cmd.Flags().String("start", "", "start time (Unix ms or ISO 8601)")
	cmd.Flags().String("end", "", "end time (Unix ms or ISO 8601)")
	return cmd
}

func newInfoPerpDexsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "perp-dexs",
		Short: "List HIP-3 perp dexes",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			ic := buildInfoClient(cfg)

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(info.PerpDexsRequest{Type: "perpDexs"}), nil)
			}

			raw, err := ic.PerpDexs(cmd.Context())
			if err != nil {
				return err
			}

			result, err := info.ParsePerpDexsResult(raw)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, raw, result)
		},
	}
}
