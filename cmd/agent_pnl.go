package cmd

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

type agentPositionPnl struct {
	Coin          string `json:"coin"`
	Size          string `json:"size"`
	EntryPrice    string `json:"entry_price"`
	MidPrice      string `json:"mid_price"`
	UnrealizedPnl string `json:"unrealized_pnl"`
	FundingPnl    string `json:"funding_pnl"`
}

type agentPnlResult struct {
	Address             string             `json:"address"`
	LookbackHours       int                `json:"lookback_hours"`
	Positions           []agentPositionPnl `json:"positions"`
	TotalUnrealizedPnl  string             `json:"total_unrealized_pnl"`
	RealizedPnl         string             `json:"realized_pnl"`
	TotalFundingPnl     string             `json:"total_funding_pnl"`
	TotalPnl            string             `json:"total_pnl"`
	FundingUnavailable  bool               `json:"funding_unavailable"`
	RealizedUnavailable bool               `json:"realized_unavailable"`
	Timestamp           string             `json:"timestamp"`
	Partial             bool               `json:"partial"`
	Errors              []agentStepError   `json:"errors,omitempty"`
}

type agentPnlTable struct {
	result *agentPnlResult
}

func (agentPnlTable) Headers() []string {
	return []string{
		"COIN",
		"SIZE",
		"ENTRY_PX",
		"MID_PX",
		"UNREALIZED_PNL",
		"FUNDING_PNL",
	}
}

func (t agentPnlTable) Rows() [][]string {
	if t.result == nil {
		return nil
	}

	rows := make([][]string, 0, len(t.result.Positions)+3)
	for _, pos := range t.result.Positions {
		rows = append(rows, []string{
			pos.Coin,
			pos.Size,
			pos.EntryPrice,
			pos.MidPrice,
			pos.UnrealizedPnl,
			pos.FundingPnl,
		})
	}

	rows = append(rows,
		[]string{"TOTAL", "", "", "", t.result.TotalUnrealizedPnl, t.result.TotalFundingPnl},
		[]string{"REALIZED", "", "", "", t.result.RealizedPnl, ""},
		[]string{"NET", "", "", "", t.result.TotalPnl, ""},
	)

	return rows
}

func newAgentPnlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pnl",
		Short: "Compute unrealized, realized, and funding PnL",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			lookbackHours, _ := cmd.Flags().GetInt("lookback-hours")     //nolint:errcheck // known flag
			aggregateByTime, _ := cmd.Flags().GetBool("aggregate-fills") //nolint:errcheck // known flag

			if lookbackHours <= 0 {
				return output.NewCLIError(output.ErrValidation, "lookback-hours must be positive").
					WithDetails("value", lookbackHours)
			}

			user, err := resolveAddressFlagUser(cmd, cfg)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			endTime := now.UnixMilli()
			startTime := now.Add(-time.Duration(lookbackHours) * time.Hour).UnixMilli()
			var aggregateByTimePtr *bool
			if cmd.Flags().Changed("aggregate-fills") {
				aggregateByTimePtr = &aggregateByTime
			}
			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(agentPnlDryRunPayload(user, cfg.Dex, lookbackHours, startTime, endTime, aggregateByTimePtr)), nil)
			}

			result, err := runAgentPnl(cmd, cfg, user, lookbackHours, now, startTime, endTime, aggregateByTimePtr)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, mustMarshal(result), agentPnlTable{result: result})
		},
	}

	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	cmd.Flags().Int("lookback-hours", 24, "lookback window in hours for realized/funding attribution")
	cmd.Flags().Bool("aggregate-fills", false, "request aggregateByTime for user fills endpoint")
	return cmd
}

func agentPnlDryRunPayload(user, dex string, lookbackHours int, startTime, endTime int64, aggregateByTimePtr *bool) map[string]any {
	return map[string]any{
		"user":           user,
		"lookback_hours": lookbackHours,
		"requests": map[string]any{
			"state":        info.ClearinghouseStateRequest{Type: "clearinghouseState", User: user, Dex: dex},
			"mids":         info.AllMidsRequest{Type: "allMids", Dex: dex},
			"fills":        info.UserFillsRequest{Type: "userFillsByTime", User: user, StartTime: startTime, EndTime: endTime, AggregateByTime: aggregateByTimePtr},
			"user_funding": info.UserFundingRequest{Type: "userFunding", User: user, StartTime: startTime, EndTime: endTime},
		},
	}
}

func runAgentPnl(cmd *cobra.Command, cfg *config.Config, user string, lookbackHours int, now time.Time, startTime, endTime int64, aggregateByTimePtr *bool) (*agentPnlResult, error) {
	ic := buildInfoClient(cfg)
	_, state, err := fetchPerpState(cmd.Context(), cfg, user, cfg.Dex)
	if err != nil {
		return nil, err
	}

	mids := make(info.MidsResult)
	if len(state.AssetPositions) > 0 {
		_, mids, err = fetchMids(cmd.Context(), cfg, cfg.Dex)
		if err != nil {
			return nil, err
		}
	}

	result := &agentPnlResult{Address: user, LookbackHours: lookbackHours, Timestamp: now.Format(time.RFC3339)}
	realizedPnl, fundingByCoin, totalFunding := populateAgentPnlAttribution(ic, cmd, cfg, user, startTime, endTime, aggregateByTimePtr, result)
	totalUnrealized := decimal.Zero
	result.Positions = make([]agentPositionPnl, 0, len(state.AssetPositions))
	for _, ap := range state.AssetPositions {
		pos, stepErr := positionPnl(ap.Position, mids, fundingByCoin)
		if stepErr != nil {
			result.Errors = append(result.Errors, *stepErr)
			continue
		}
		result.Positions = append(result.Positions, *pos)

		unrealized, parseErr := decimal.NewFromString(pos.UnrealizedPnl)
		if parseErr != nil {
			return nil, output.NewCLIError(output.ErrAPI, "failed to parse unrealized pnl").
				WithDetails("coin", pos.Coin).
				WithDetails("value", pos.UnrealizedPnl)
		}
		totalUnrealized = totalUnrealized.Add(unrealized)
	}
	if len(state.AssetPositions) > 0 && len(result.Positions) == 0 {
		return nil, output.NewCLIError(output.ErrAPI, "agent pnl failed: unable to price any open positions").
			WithDetails("errors", result.Errors)
	}

	totalPnl := totalUnrealized.Add(realizedPnl).Add(totalFunding)
	result.TotalUnrealizedPnl = totalUnrealized.String()
	result.RealizedPnl = realizedPnl.String()
	result.TotalFundingPnl = totalFunding.String()
	result.TotalPnl = totalPnl.String()
	result.Partial = result.FundingUnavailable || result.RealizedUnavailable || len(result.Errors) > 0
	return result, nil
}

func populateAgentPnlAttribution(ic *info.InfoClient, cmd *cobra.Command, cfg *config.Config, user string, startTime, endTime int64, aggregateByTimePtr *bool, result *agentPnlResult) (decimal.Decimal, map[string]decimal.Decimal, decimal.Decimal) {
	realizedPnl := decimal.Zero
	fillsRaw, fillsErr := ic.UserFillsByTime(cmd.Context(), user, startTime, endTime, aggregateByTimePtr)
	if fillsErr != nil {
		result.RealizedUnavailable = true
		result.Errors = append(result.Errors, toAgentStepError("fills", fillsErr))
	} else if fills, parseErr := info.ParseFillsResult(fillsRaw); parseErr != nil {
		result.RealizedUnavailable = true
		result.Errors = append(result.Errors, toAgentStepError("fills", parseErr))
	} else {
		realizedPnl, result.Errors = addClosedPnl(fills, result.Errors, cfg.Dex)
	}

	fundingByCoin := make(map[string]decimal.Decimal)
	totalFunding := decimal.Zero
	userFundingRaw, fundingErr := ic.UserFunding(cmd.Context(), user, startTime, endTime)
	if fundingErr != nil {
		result.FundingUnavailable = true
		result.Errors = append(result.Errors, toAgentStepError("user-funding", fundingErr))
	} else if funding, parseErr := info.ParseUserFundingResult(userFundingRaw); parseErr != nil {
		result.FundingUnavailable = true
		result.Errors = append(result.Errors, toAgentStepError("user-funding", parseErr))
	} else {
		fundingByCoin, totalFunding, result.Errors = aggregateFundingByCoin(funding, result.Errors, cfg.Dex)
	}
	return realizedPnl, fundingByCoin, totalFunding
}
