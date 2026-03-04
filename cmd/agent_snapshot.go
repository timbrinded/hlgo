package cmd

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

type agentStepError struct {
	Step    string           `json:"step"`
	Code    output.ErrorCode `json:"code"`
	Error   string           `json:"error"`
	Details map[string]any   `json:"details,omitempty"`
}

type agentSnapshotResult struct {
	AccountValue  string                `json:"account_value,omitempty"`
	PerpPositions []info.AssetPosition  `json:"perp_positions"`
	SpotBalances  any                   `json:"spot_balances,omitempty"`
	OpenOrders    info.OpenOrdersResult `json:"open_orders"`
	RecentFills   info.FillsResult      `json:"recent_fills"`
	MidPrices     info.MidsResult       `json:"mid_prices,omitempty"`
	Timestamp     string                `json:"timestamp"`
	Partial       bool                  `json:"partial"`
	Errors        []agentStepError      `json:"errors,omitempty"`
}

type agentSnapshotTable struct {
	result *agentSnapshotResult
}

func (agentSnapshotTable) Headers() []string {
	return []string{
		"ACCOUNT_VALUE",
		"PERP_POSITIONS",
		"SPOT_BALANCES",
		"OPEN_ORDERS",
		"RECENT_FILLS",
		"PARTIAL",
	}
}

func (t agentSnapshotTable) Rows() [][]string {
	if t.result == nil {
		return nil
	}

	return [][]string{{
		t.result.AccountValue,
		strconv.Itoa(len(t.result.PerpPositions)),
		strconv.Itoa(spotBalanceCount(t.result.SpotBalances)),
		strconv.Itoa(len(t.result.OpenOrders)),
		strconv.Itoa(len(t.result.RecentFills)),
		strconv.FormatBool(t.result.Partial),
	}}
}

func newAgentSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Aggregate account state into one response",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.FromContext(cmd.Context())
			address, _ := cmd.Flags().GetString("address") //nolint:errcheck // known flag

			user, err := info.ResolveUserAddress(address, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(map[string]any{
					"user": user,
					"requests": map[string]any{
						"state": info.ClearinghouseStateRequest{
							Type: "clearinghouseState",
							User: user,
							Dex:  cfg.Dex,
						},
						"spot_state": info.SpotClearinghouseStateRequest{
							Type: "spotClearinghouseState",
							User: user,
						},
						"open_orders": info.FrontendOpenOrdersRequest{
							Type: "frontendOpenOrders",
							User: user,
							Dex:  cfg.Dex,
						},
						"fills": info.UserFillsRequest{
							Type: "userFills",
							User: user,
						},
						"mids": info.AllMidsRequest{
							Type: "allMids",
							Dex:  cfg.Dex,
						},
					},
				}), nil)
			}

			ic := buildInfoClient(cfg)
			result := &agentSnapshotResult{
				PerpPositions: make([]info.AssetPosition, 0),
				OpenOrders:    make(info.OpenOrdersResult, 0),
				RecentFills:   make(info.FillsResult, 0),
				Timestamp:     time.Now().UTC().Format(time.RFC3339),
			}
			successCount := 0

			stateRaw, err := ic.ClearinghouseState(cmd.Context(), user, cfg.Dex)
			if err != nil {
				result.Errors = append(result.Errors, toAgentStepError("state", err))
			} else {
				state, parseErr := info.ParseStateResult(stateRaw)
				if parseErr != nil {
					result.Errors = append(result.Errors, toAgentStepError("state", parseErr))
				} else {
					result.AccountValue = state.MarginSummary.AccountValue
					result.PerpPositions = state.AssetPositions
					successCount++
				}
			}

			spotRaw, err := ic.SpotClearinghouseState(cmd.Context(), user)
			if err != nil {
				result.Errors = append(result.Errors, toAgentStepError("spot-state", err))
			} else {
				balances, parseErr := extractSpotBalances(spotRaw)
				if parseErr != nil {
					result.Errors = append(result.Errors, toAgentStepError("spot-state", parseErr))
				} else {
					result.SpotBalances = balances
					successCount++
				}
			}

			openOrdersRaw, err := ic.FrontendOpenOrders(cmd.Context(), user, cfg.Dex)
			if err != nil {
				result.Errors = append(result.Errors, toAgentStepError("open-orders", err))
			} else {
				orders, parseErr := info.ParseOpenOrdersResult(openOrdersRaw)
				if parseErr != nil {
					result.Errors = append(result.Errors, toAgentStepError("open-orders", parseErr))
				} else {
					result.OpenOrders = orders
					successCount++
				}
			}

			fillsRaw, err := ic.UserFills(cmd.Context(), user, nil)
			if err != nil {
				result.Errors = append(result.Errors, toAgentStepError("fills", err))
			} else {
				fills, parseErr := info.ParseFillsResult(fillsRaw)
				if parseErr != nil {
					result.Errors = append(result.Errors, toAgentStepError("fills", parseErr))
				} else {
					if len(fills) > 10 {
						fills = fills[:10]
					}
					result.RecentFills = fills
					successCount++
				}
			}

			midsRaw, err := ic.AllMids(cmd.Context(), cfg.Dex)
			if err != nil {
				result.Errors = append(result.Errors, toAgentStepError("mids", err))
			} else {
				mids, parseErr := info.ParseMidsResult(midsRaw)
				if parseErr != nil {
					result.Errors = append(result.Errors, toAgentStepError("mids", parseErr))
				} else {
					result.MidPrices = mids
					successCount++
				}
			}

			if successCount == 0 {
				return output.NewCLIError(output.ErrAPI, "agent snapshot failed: all subqueries failed").
					WithDetails("errors", result.Errors)
			}

			result.Partial = len(result.Errors) > 0

			return printResult(cmd, cfg, mustMarshal(result), agentSnapshotTable{result: result})
		},
	}

	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	return cmd
}

func toAgentStepError(step string, err error) agentStepError {
	var cliErr *output.CLIError
	if errors.As(err, &cliErr) {
		var details map[string]any
		if len(cliErr.Details) > 0 {
			details = make(map[string]any, len(cliErr.Details))
			for k, v := range cliErr.Details {
				details[k] = v
			}
		}
		return agentStepError{
			Step:    step,
			Code:    cliErr.Code,
			Error:   cliErr.Message,
			Details: details,
		}
	}

	return agentStepError{
		Step:  step,
		Code:  output.ErrAPI,
		Error: err.Error(),
	}
}

func extractSpotBalances(raw json.RawMessage) (any, error) {
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	obj, ok := parsed.(map[string]any)
	if !ok {
		return parsed, nil
	}

	for _, key := range []string{"balances", "balancesAndHolds", "balances_and_holds"} {
		if balances, exists := obj[key]; exists {
			return balances, nil
		}
	}

	return parsed, nil
}

func spotBalanceCount(v any) int {
	switch vv := v.(type) {
	case nil:
		return 0
	case []any:
		return len(vv)
	case map[string]any:
		for _, key := range []string{"balances", "balancesAndHolds", "balances_and_holds"} {
			if balances, ok := vv[key].([]any); ok {
				return len(balances)
			}
		}
		return len(vv)
	default:
		return 1
	}
}
