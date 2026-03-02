package cmd

import (
	"strconv"
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
			address, _ := cmd.Flags().GetString("address")               //nolint:errcheck // known flag
			lookbackHours, _ := cmd.Flags().GetInt("lookback-hours")     //nolint:errcheck // known flag
			aggregateByTime, _ := cmd.Flags().GetBool("aggregate-fills") //nolint:errcheck // known flag

			if lookbackHours <= 0 {
				return output.NewCLIError(output.ErrValidation, "lookback-hours must be positive").
					WithDetails("value", lookbackHours)
			}

			user, err := info.ResolveUserAddress(address, cfg)
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
				return printResult(cmd, cfg, mustMarshal(map[string]any{
					"user":           user,
					"lookback_hours": lookbackHours,
					"requests": map[string]any{
						"state": info.ClearinghouseStateRequest{
							Type: "clearinghouseState",
							User: user,
							Dex:  cfg.Dex,
						},
						"mids": info.AllMidsRequest{
							Type: "allMids",
							Dex:  cfg.Dex,
						},
						"fills": info.UserFillsRequest{
							Type:            "userFillsByTime",
							User:            user,
							StartTime:       startTime,
							EndTime:         endTime,
							AggregateByTime: aggregateByTimePtr,
						},
						"user_funding": info.UserFundingRequest{
							Type:      "userFunding",
							User:      user,
							StartTime: startTime,
							EndTime:   endTime,
						},
					},
				}), nil)
			}

			ic := buildInfoClient(cfg)

			stateRaw, err := ic.ClearinghouseState(cmd.Context(), user, cfg.Dex)
			if err != nil {
				return err
			}
			state, err := info.ParseStateResult(stateRaw)
			if err != nil {
				return err
			}

			mids := make(info.MidsResult)
			if len(state.AssetPositions) > 0 {
				midsRaw, err := ic.AllMids(cmd.Context(), cfg.Dex)
				if err != nil {
					return err
				}
				mids, err = info.ParseMidsResult(midsRaw)
				if err != nil {
					return err
				}
			}

			result := &agentPnlResult{
				Address:       user,
				LookbackHours: lookbackHours,
				Timestamp:     now.Format(time.RFC3339),
			}

			realizedPnl := decimal.Zero
			fillsRaw, fillsErr := ic.UserFillsByTime(cmd.Context(), user, startTime, endTime, aggregateByTimePtr)
			if fillsErr != nil {
				result.RealizedUnavailable = true
				result.Errors = append(result.Errors, toAgentStepError("fills", fillsErr))
			} else {
				fills, parseErr := info.ParseFillsResult(fillsRaw)
				if parseErr != nil {
					result.RealizedUnavailable = true
					result.Errors = append(result.Errors, toAgentStepError("fills", parseErr))
				} else {
					realizedPnl, result.Errors = addClosedPnl(fills, result.Errors)
				}
			}

			fundingByCoin := make(map[string]decimal.Decimal)
			totalFunding := decimal.Zero
			userFundingRaw, fundingErr := ic.UserFunding(cmd.Context(), user, startTime, endTime)
			if fundingErr != nil {
				result.FundingUnavailable = true
				result.Errors = append(result.Errors, toAgentStepError("user-funding", fundingErr))
			} else {
				funding, parseErr := info.ParseUserFundingResult(userFundingRaw)
				if parseErr != nil {
					result.FundingUnavailable = true
					result.Errors = append(result.Errors, toAgentStepError("user-funding", parseErr))
				} else {
					fundingByCoin, totalFunding, result.Errors = aggregateFundingByCoin(funding, result.Errors)
				}
			}

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
					return output.NewCLIError(output.ErrAPI, "failed to parse unrealized pnl").
						WithDetails("coin", pos.Coin).
						WithDetails("value", pos.UnrealizedPnl)
				}
				totalUnrealized = totalUnrealized.Add(unrealized)
			}

			if len(state.AssetPositions) > 0 && len(result.Positions) == 0 {
				return output.NewCLIError(output.ErrAPI, "agent pnl failed: unable to price any open positions").
					WithDetails("errors", result.Errors)
			}

			totalPnl := totalUnrealized.Add(realizedPnl).Add(totalFunding)
			result.TotalUnrealizedPnl = totalUnrealized.String()
			result.RealizedPnl = realizedPnl.String()
			result.TotalFundingPnl = totalFunding.String()
			result.TotalPnl = totalPnl.String()
			result.Partial = result.FundingUnavailable || result.RealizedUnavailable || len(result.Errors) > 0

			return printResult(cmd, cfg, mustMarshal(result), agentPnlTable{result: result})
		},
	}

	cmd.Flags().String("address", "", "user address (default: derived from agent wallet)")
	cmd.Flags().Int("lookback-hours", 24, "lookback window in hours for realized/funding attribution")
	cmd.Flags().Bool("aggregate-fills", false, "request aggregateByTime for user fills endpoint")
	return cmd
}

func positionPnl(position info.Position, mids info.MidsResult, fundingByCoin map[string]decimal.Decimal) (*agentPositionPnl, *agentStepError) {
	size, err := decimal.NewFromString(position.Szi)
	if err != nil {
		stepErr := agentStepError{
			Step:  "pnl",
			Code:  output.ErrAPI,
			Error: "invalid position size for " + position.Coin,
		}
		return nil, &stepErr
	}

	entryPx, err := decimal.NewFromString(position.EntryPx)
	if err != nil {
		stepErr := agentStepError{
			Step:  "pnl",
			Code:  output.ErrAPI,
			Error: "invalid entry price for " + position.Coin,
		}
		return nil, &stepErr
	}

	midStr, ok := mids[position.Coin]
	if !ok {
		stepErr := agentStepError{
			Step:  "pnl",
			Code:  output.ErrAPI,
			Error: "missing mid price for " + position.Coin,
		}
		return nil, &stepErr
	}

	midPx, err := decimal.NewFromString(midStr)
	if err != nil {
		stepErr := agentStepError{
			Step:  "pnl",
			Code:  output.ErrAPI,
			Error: "invalid mid price for " + position.Coin,
		}
		return nil, &stepErr
	}

	unrealized := midPx.Sub(entryPx).Mul(size)
	funding := fundingByCoin[position.Coin]

	return &agentPositionPnl{
		Coin:          position.Coin,
		Size:          position.Szi,
		EntryPrice:    position.EntryPx,
		MidPrice:      midStr,
		UnrealizedPnl: unrealized.String(),
		FundingPnl:    funding.String(),
	}, nil
}

func addClosedPnl(fills info.FillsResult, errs []agentStepError) (decimal.Decimal, []agentStepError) {
	total := decimal.Zero
	for _, fill := range fills {
		closedPnl := fill.ClosedPnl
		if closedPnl == "" {
			continue
		}

		value, err := decimal.NewFromString(closedPnl)
		if err != nil {
			errs = append(errs, agentStepError{
				Step:  "fills",
				Code:  output.ErrAPI,
				Error: "invalid closedPnl for oid " + strconv.FormatInt(fill.Oid, 10),
			})
			continue
		}
		total = total.Add(value)
	}
	return total, errs
}

func aggregateFundingByCoin(funding info.UserFundingResult, errs []agentStepError) (map[string]decimal.Decimal, decimal.Decimal, []agentStepError) {
	byCoin := make(map[string]decimal.Decimal)
	total := decimal.Zero

	for _, entry := range funding {
		coin := entry.Delta.Coin
		if coin == "" {
			coin = "UNKNOWN"
		}
		if entry.Delta.USDC == "" {
			continue
		}

		value, err := decimal.NewFromString(entry.Delta.USDC)
		if err != nil {
			errs = append(errs, agentStepError{
				Step:  "user-funding",
				Code:  output.ErrAPI,
				Error: "invalid funding usdc for coin " + coin,
			})
			continue
		}

		byCoin[coin] = byCoin[coin].Add(value)
		total = total.Add(value)
	}

	return byCoin, total, errs
}
