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

			user, err := resolveAddressFlagUser(cmd, cfg)
			if err != nil {
				return err
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(snapshotDryRunPayload(user, cfg.Dex)), nil)
			}

			result, err := runAgentSnapshot(cmd, cfg, user)
			if err != nil {
				return err
			}
			return printResult(cmd, cfg, mustMarshal(result), agentSnapshotTable{result: result})
		},
	}

	cmd.Flags().String("address", "", "user address (default: derived from configured private key)")
	return cmd
}

func snapshotDryRunPayload(user, dex string) map[string]any {
	return map[string]any{
		"user": user,
		"requests": map[string]any{
			"state":       info.ClearinghouseStateRequest{Type: "clearinghouseState", User: user, Dex: dex},
			"spot_state":  info.SpotClearinghouseStateRequest{Type: "spotClearinghouseState", User: user},
			"open_orders": info.FrontendOpenOrdersRequest{Type: "frontendOpenOrders", User: user, Dex: dex},
			"fills":       info.UserFillsRequest{Type: "userFills", User: user},
			"mids":        info.AllMidsRequest{Type: "allMids", Dex: dex},
		},
	}
}

func runAgentSnapshot(cmd *cobra.Command, cfg *config.Config, user string) (*agentSnapshotResult, error) {
	ic := buildInfoClient(cfg)
	result := &agentSnapshotResult{PerpPositions: make([]info.AssetPosition, 0), OpenOrders: make(info.OpenOrdersResult, 0), RecentFills: make(info.FillsResult, 0), Timestamp: time.Now().UTC().Format(time.RFC3339)}
	successCount := 0

	_, state, err := fetchPerpState(cmd.Context(), cfg, user, cfg.Dex)
	if err != nil {
		result.Errors = append(result.Errors, toAgentStepError("state", err))
	} else {
		result.AccountValue = state.MarginSummary.AccountValue
		result.PerpPositions = state.AssetPositions
		successCount++
	}

	spotRaw, err := ic.SpotClearinghouseState(cmd.Context(), user)
	if err != nil {
		result.Errors = append(result.Errors, toAgentStepError("spot-state", err))
	} else if balances, parseErr := extractSpotBalances(spotRaw); parseErr != nil {
		result.Errors = append(result.Errors, toAgentStepError("spot-state", parseErr))
	} else {
		result.SpotBalances = balances
		successCount++
	}

	_, orders, err := fetchOpenOrders(cmd, cfg, user, cfg.Dex)
	if err != nil {
		result.Errors = append(result.Errors, toAgentStepError("open-orders", err))
	} else {
		result.OpenOrders = orders
		successCount++
	}

	fillsRaw, err := ic.UserFills(cmd.Context(), user, nil)
	if err != nil {
		result.Errors = append(result.Errors, toAgentStepError("fills", err))
	} else if fills, parseErr := info.ParseFillsResult(fillsRaw); parseErr != nil {
		result.Errors = append(result.Errors, toAgentStepError("fills", parseErr))
	} else {
		if len(fills) > 10 {
			fills = fills[:10]
		}
		result.RecentFills = fills
		successCount++
	}

	_, mids, err := fetchMids(cmd.Context(), cfg, cfg.Dex)
	if err != nil {
		result.Errors = append(result.Errors, toAgentStepError("mids", err))
	} else {
		result.MidPrices = mids
		successCount++
	}
	if successCount == 0 {
		return nil, output.NewCLIError(output.ErrAPI, "agent snapshot failed: all subqueries failed").
			WithDetails("errors", result.Errors)
	}

	result.Partial = len(result.Errors) > 0
	return result, nil
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
		return agentStepError{Step: step, Code: cliErr.Code, Error: cliErr.Message, Details: details}
	}

	return agentStepError{Step: step, Code: output.ErrAPI, Error: err.Error()}
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
