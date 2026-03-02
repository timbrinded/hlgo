package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

func TestAgentSubcommands_Registered(t *testing.T) {
	root := NewRootCommand("test")

	var agentCmdName string
	var subcommands []string
	for _, c := range root.Commands() {
		if c.Name() == "agent" {
			agentCmdName = c.Name()
			for _, sub := range c.Commands() {
				subcommands = append(subcommands, sub.Name())
			}
			break
		}
	}

	if agentCmdName != "agent" {
		t.Fatal("agent command not registered")
	}

	expected := map[string]bool{"snapshot": true, "pnl": true, "bracket": true}
	for _, name := range subcommands {
		delete(expected, name)
	}
	for missing := range expected {
		t.Errorf("missing agent subcommand %q", missing)
	}
}

func TestAgentSnapshot_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("agent", "snapshot", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	requests, ok := result["requests"].(map[string]any)
	if !ok {
		t.Fatalf("requests = %T, want object", result["requests"])
	}

	for _, key := range []string{"state", "spot_state", "open_orders", "fills", "mids"} {
		if _, exists := requests[key]; !exists {
			t.Errorf("missing %q in snapshot dry-run requests", key)
		}
	}
}

func TestAgentPnl_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("agent", "pnl", "--lookback-hours", "12", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	requests, ok := result["requests"].(map[string]any)
	if !ok {
		t.Fatalf("requests = %T, want object", result["requests"])
	}
	userFunding, ok := requests["user_funding"].(map[string]any)
	if !ok {
		t.Fatalf("user_funding = %T, want object", requests["user_funding"])
	}
	if userFunding["type"] != "userFunding" {
		t.Errorf("user_funding.type = %v, want userFunding", userFunding["type"])
	}
}

func TestAgentBracket_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("agent", "bracket",
		"--coin", "BTC",
		"--side", "buy",
		"--price", "50000",
		"--size", "0.01",
		"--tp", "51000",
		"--sl", "49000",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	action, ok := result["action"].(map[string]any)
	if !ok {
		t.Fatalf("action = %T, want object", result["action"])
	}
	if action["grouping"] != "normalTpsl" {
		t.Errorf("grouping = %v, want normalTpsl", action["grouping"])
	}
	orders, ok := action["orders"].([]any)
	if !ok {
		t.Fatalf("orders = %T, want array", action["orders"])
	}
	if len(orders) != 3 {
		t.Fatalf("orders len = %d, want 3", len(orders))
	}
}

func TestAgentBracket_ValidatesTpDirection(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("agent", "bracket",
		"--coin", "BTC",
		"--side", "buy",
		"--price", "50000",
		"--size", "0.01",
		"--tp", "49000",
		"--sl", "48000",
		"--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid buy TP direction")
	}
}

func TestAgentSnapshot_PartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		switch req["type"] {
		case "clearinghouseState":
			_, _ = w.Write([]byte(`{"assetPositions":[],"marginSummary":{"accountValue":"100","totalMarginUsed":"0","totalNtlPos":"0","totalRawUsd":"0"},"crossMarginSummary":{"accountValue":"100","totalMarginUsed":"0","totalNtlPos":"0","totalRawUsd":"0"},"crossMaintenanceMarginUsed":"0","withdrawable":"100","time":1700000000000}`))
		case "spotClearinghouseState":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"spot down"}`))
		case "frontendOpenOrders":
			_, _ = w.Write([]byte(`[]`))
		case "userFills":
			_, _ = w.Write([]byte(`[]`))
		case "allMids":
			_, _ = w.Write([]byte(`{"BTC":"50000"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown type"}`))
		}
	}))
	defer srv.Close()

	t.Setenv("HLGO_API_URL", srv.URL)
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("agent", "snapshot"); err != nil {
		t.Fatalf("expected partial success, got error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if result["partial"] != true {
		t.Fatalf("partial = %v, want true", result["partial"])
	}
	errorsList, ok := result["errors"].([]any)
	if !ok || len(errorsList) == 0 {
		t.Fatalf("errors = %T %v, want non-empty array", result["errors"], result["errors"])
	}
}

func TestToAgentStepError_PreservesDetails(t *testing.T) {
	in := output.NewCLIError(output.ErrValidation, "invalid value").
		WithDetails("value", "1014.33").
		WithDetails("nearest_valid", "1014.3")

	got := toAgentStepError("tp", in)
	if got.Code != output.ErrValidation {
		t.Fatalf("code = %q, want %q", got.Code, output.ErrValidation)
	}
	if got.Details == nil {
		t.Fatal("expected details to be preserved")
	}
	if got.Details["value"] != "1014.33" {
		t.Fatalf("details.value = %v, want 1014.33", got.Details["value"])
	}
	if got.Details["nearest_valid"] != "1014.3" {
		t.Fatalf("details.nearest_valid = %v, want 1014.3", got.Details["nearest_valid"])
	}
}

func TestAgentSnapshot_EmitsEmptyArrays(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		switch req["type"] {
		case "clearinghouseState":
			_, _ = w.Write([]byte(`{"assetPositions":[],"marginSummary":{"accountValue":"100","totalMarginUsed":"0","totalNtlPos":"0","totalRawUsd":"0"},"crossMarginSummary":{"accountValue":"100","totalMarginUsed":"0","totalNtlPos":"0","totalRawUsd":"0"},"crossMaintenanceMarginUsed":"0","withdrawable":"100","time":1700000000000}`))
		case "spotClearinghouseState":
			_, _ = w.Write([]byte(`{"balances":[]}`))
		case "frontendOpenOrders":
			_, _ = w.Write([]byte(`[]`))
		case "userFills":
			_, _ = w.Write([]byte(`[]`))
		case "allMids":
			_, _ = w.Write([]byte(`{"BTC":"50000"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown type"}`))
		}
	}))
	defer srv.Close()

	t.Setenv("HLGO_API_URL", srv.URL)
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("agent", "snapshot"); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	perpPositions, ok := result["perp_positions"].([]any)
	if !ok {
		t.Fatalf("perp_positions = %T, want array", result["perp_positions"])
	}
	if len(perpPositions) != 0 {
		t.Fatalf("perp_positions len = %d, want 0", len(perpPositions))
	}

	openOrders, ok := result["open_orders"].([]any)
	if !ok {
		t.Fatalf("open_orders = %T, want array", result["open_orders"])
	}
	if len(openOrders) != 0 {
		t.Fatalf("open_orders len = %d, want 0", len(openOrders))
	}

	recentFills, ok := result["recent_fills"].([]any)
	if !ok {
		t.Fatalf("recent_fills = %T, want array", result["recent_fills"])
	}
	if len(recentFills) != 0 {
		t.Fatalf("recent_fills len = %d, want 0", len(recentFills))
	}
}

func TestAgentSnapshot_AllFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer srv.Close()

	t.Setenv("HLGO_API_URL", srv.URL)
	_, _, run := newTestRootWithServer(t, "")

	if err := run("agent", "snapshot"); err == nil {
		t.Fatal("expected error when all snapshot subqueries fail")
	}
}

func TestAgentPnl_SkipsMidsWhenNoPositions(t *testing.T) {
	var midsCalled atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		switch req["type"] {
		case "clearinghouseState":
			_, _ = w.Write([]byte(`{"assetPositions":[],"marginSummary":{"accountValue":"100","totalMarginUsed":"0","totalNtlPos":"0","totalRawUsd":"0"},"crossMarginSummary":{"accountValue":"100","totalMarginUsed":"0","totalNtlPos":"0","totalRawUsd":"0"},"crossMaintenanceMarginUsed":"0","withdrawable":"100","time":1700000000000}`))
		case "userFillsByTime":
			_, _ = w.Write([]byte(`[]`))
		case "userFunding":
			_, _ = w.Write([]byte(`[]`))
		case "allMids":
			midsCalled.Store(true)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"mids down"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown type"}`))
		}
	}))
	defer srv.Close()

	t.Setenv("HLGO_API_URL", srv.URL)
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("agent", "pnl", "--lookback-hours", "24"); err != nil {
		t.Fatalf("expected success with no positions even if mids unavailable, got: %v", err)
	}

	if midsCalled.Load() {
		t.Fatal("mids endpoint should not be called when there are no positions")
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if result["total_unrealized_pnl"] != "0" {
		t.Fatalf("total_unrealized_pnl = %v, want 0", result["total_unrealized_pnl"])
	}
	if result["partial"] != false {
		t.Fatalf("partial = %v, want false", result["partial"])
	}
}

func TestPositionPnl_ComputesSignedUnrealized(t *testing.T) {
	pos := info.Position{
		Coin:    "BTC",
		Szi:     "-2",
		EntryPx: "100",
	}
	pnl, stepErr := positionPnl(pos, info.MidsResult{"BTC": "90"}, map[string]decimal.Decimal{"BTC": decimal.NewFromFloat(-0.5)})
	if stepErr != nil {
		t.Fatalf("unexpected step error: %+v", stepErr)
	}
	if pnl.UnrealizedPnl != "20" {
		t.Errorf("unrealized = %q, want 20", pnl.UnrealizedPnl)
	}
	if pnl.FundingPnl != "-0.5" {
		t.Errorf("funding = %q, want -0.5", pnl.FundingPnl)
	}
}

func TestAggregateFundingByCoin(t *testing.T) {
	funding := info.UserFundingResult{
		{Delta: info.UserFundingDelta{Coin: "BTC", USDC: "-0.2"}},
		{Delta: info.UserFundingDelta{Coin: "BTC", USDC: "0.1"}},
		{Delta: info.UserFundingDelta{Coin: "ETH", USDC: "0.3"}},
	}
	byCoin, total, errs := aggregateFundingByCoin(funding, nil)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if got := byCoin["BTC"].String(); got != "-0.1" {
		t.Errorf("BTC funding = %q, want -0.1", got)
	}
	if got := byCoin["ETH"].String(); got != "0.3" {
		t.Errorf("ETH funding = %q, want 0.3", got)
	}
	if got := total.String(); got != "0.2" {
		t.Errorf("total funding = %q, want 0.2", got)
	}
}
