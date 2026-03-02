package exchange

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/output"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/signer"
	"github.com/timbrinded/hlgo/pkg/wire"
)

// mockResolver returns fixed asset info for testing.
type mockResolver struct {
	info *resolver.AssetInfo
	err  error
}

func (m *mockResolver) ResolveAsset(_ context.Context, _ string) (*resolver.AssetInfo, error) {
	return m.info, m.err
}

func newMidsServer(t *testing.T, mids string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/info" {
			http.NotFound(w, r)
			return
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["type"] != "allMids" {
			t.Fatalf("type = %q, want allMids", req["type"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mids)
	}))
}

func TestExecutor_PlaceOrder_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	c := client.NewClient("http://unused")
	r := &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
			IsSpot:     false,
		},
	}

	exec := NewExecutor(s, c, r, false)

	result, err := exec.PlaceOrder(context.Background(), PlaceOrderInput{
		Coin:  "BTC",
		Side:  "buy",
		Price: decimal.NewFromInt(50000),
		Size:  decimal.NewFromFloat(0.01),
		Tif:   "Gtc",
		Builder: &BuilderInfo{
			B: "0x1234567890abcdef1234567890abcdef12345678",
			F: 10,
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("PlaceOrder dry-run error: %v", err)
	}
	if result.Action == nil {
		t.Fatal("expected action in dry-run result")
	}
	if result.Response != nil {
		t.Error("expected nil response in dry-run")
	}
	if result.Resolved.Coin != "BTC" {
		t.Errorf("Resolved.Coin = %q, want BTC", result.Resolved.Coin)
	}
	action := result.Action
	if action.Orders[0].P != "50000" {
		t.Errorf("price = %q, want 50000", action.Orders[0].P)
	}
	if action.Builder == nil {
		t.Fatal("expected builder in action")
	}
	if action.Builder.F != 10 {
		t.Errorf("builder fee = %d, want 10", action.Builder.F)
	}
}

func TestExecutor_PlaceOrder_DryRun_WithBracketTriggers(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
			IsSpot:     false,
		},
	}, false)

	tp := "50500"
	sl := "49500"
	result, err := exec.PlaceOrder(context.Background(), PlaceOrderInput{
		Coin:      "BTC",
		Side:      "buy",
		Price:     decimal.NewFromInt(50000),
		Size:      decimal.RequireFromString("0.01"),
		Tif:       "Gtc",
		TpTrigger: &tp,
		SlTrigger: &sl,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("PlaceOrder dry-run error: %v", err)
	}

	if result.Action == nil {
		t.Fatal("expected action in dry-run result")
	}
	if got := result.Action.Grouping; got != "normalTpsl" {
		t.Fatalf("grouping = %q, want normalTpsl", got)
	}

	if got := len(result.Action.Orders); got != 3 {
		t.Fatalf("orders len = %d, want 3", got)
	}

	wantTP, err := wire.PriceToWire(decimal.RequireFromString(tp), 3, false)
	if err != nil {
		t.Fatalf("wire TP price: %v", err)
	}
	wantSL, err := wire.PriceToWire(decimal.RequireFromString(sl), 3, false)
	if err != nil {
		t.Fatalf("wire SL price: %v", err)
	}

	tpOrder := result.Action.Orders[1]
	slOrder := result.Action.Orders[2]
	if tpOrder.T.Trigger == nil || slOrder.T.Trigger == nil {
		t.Fatal("expected trigger wires on tp/sl orders")
	}
	if tpOrder.T.Trigger.TriggerPx != wantTP {
		t.Errorf("tp trigger px = %q, want %q", tpOrder.T.Trigger.TriggerPx, wantTP)
	}
	if slOrder.T.Trigger.TriggerPx != wantSL {
		t.Errorf("sl trigger px = %q, want %q", slOrder.T.Trigger.TriggerPx, wantSL)
	}
	if tpOrder.B {
		t.Errorf("tp order side is buy, want sell for long bracket")
	}
	if slOrder.B {
		t.Errorf("sl order side is buy, want sell for long bracket")
	}
}

func TestExecutor_PlaceOrder_RejectsInvalidTriggerPrice(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
			IsSpot:     false,
		},
	}, false)

	bad := "not-a-price"
	_, err = exec.PlaceOrder(context.Background(), PlaceOrderInput{
		Coin:      "BTC",
		Side:      "buy",
		Price:     decimal.NewFromInt(50000),
		Size:      decimal.RequireFromString("0.01"),
		Tif:       "Gtc",
		TpTrigger: &bad,
		DryRun:    true,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("code = %q, want %q", cliErr.Code, output.ErrValidation)
	}
}

func TestExecutor_PlaceOrder_WrapsTpWireValidationError(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
			IsSpot:     false,
		},
	}, false)

	invalidTp := "50500.12" // exceeds 5 significant figures for wire price rules.
	_, err = exec.PlaceOrder(context.Background(), PlaceOrderInput{
		Coin:      "BTC",
		Side:      "buy",
		Price:     decimal.NewFromInt(50000),
		Size:      decimal.RequireFromString("0.01"),
		Tif:       "Gtc",
		TpTrigger: &invalidTp,
		DryRun:    true,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("code = %q, want %q", cliErr.Code, output.ErrValidation)
	}
	if got := cliErr.Details["flag"]; got != "--tp" {
		t.Errorf("details.flag = %v, want --tp", got)
	}
	if !strings.Contains(cliErr.Message, "--tp trigger price:") {
		t.Errorf("message = %q, want --tp trigger context", cliErr.Message)
	}
	if _, ok := cliErr.Details["max_sig_figs"]; !ok {
		t.Errorf("expected original validation details to be preserved, got %#v", cliErr.Details)
	}
}

func TestExecutor_PlaceOrder_WrapsSlWireValidationError(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
			IsSpot:     false,
		},
	}, false)

	invalidSl := "49500.12" // exceeds 5 significant figures for wire price rules.
	_, err = exec.PlaceOrder(context.Background(), PlaceOrderInput{
		Coin:      "BTC",
		Side:      "buy",
		Price:     decimal.NewFromInt(50000),
		Size:      decimal.RequireFromString("0.01"),
		Tif:       "Gtc",
		SlTrigger: &invalidSl,
		DryRun:    true,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("code = %q, want %q", cliErr.Code, output.ErrValidation)
	}
	if got := cliErr.Details["flag"]; got != "--sl" {
		t.Errorf("details.flag = %v, want --sl", got)
	}
	if !strings.Contains(cliErr.Message, "--sl trigger price:") {
		t.Errorf("message = %q, want --sl trigger context", cliErr.Message)
	}
	if _, ok := cliErr.Details["max_sig_figs"]; !ok {
		t.Errorf("expected original validation details to be preserved, got %#v", cliErr.Details)
	}
}

func TestExecutor_PlaceMarketOrder_DryRunPerp(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	srv := newMidsServer(t, `{"BTC":"50000"}`)
	defer srv.Close()

	exec := NewExecutor(s, client.NewClient(srv.URL), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:       0,
			Coin:          "BTC",
			CanonicalCoin: "BTC",
			SzDecimals:    3,
			IsSpot:        false,
		},
	}, false)

	result, err := exec.PlaceMarketOrder(context.Background(), PlaceMarketOrderInput{
		Coin:            "BTC",
		Side:            "sell",
		Size:            decimal.NewFromFloat(0.01),
		SlippagePercent: decimal.NewFromFloat(0.5),
		DryRun:          true,
	})
	if err != nil {
		t.Fatalf("PlaceMarketOrder dry-run error: %v", err)
	}

	if result.Action == nil {
		t.Fatal("expected action in dry-run result")
	}
	action := result.Action
	if len(action.Orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(action.Orders))
	}
	if got := action.Orders[0].T.Limit.Tif; got != "Ioc" {
		t.Errorf("Tif = %q, want Ioc", got)
	}
	if got := action.Orders[0].P; got != "49750" {
		t.Errorf("price = %q, want 49750", got)
	}
}

func TestExecutor_PlaceMarketOrder_DryRunSpotCanonicalLookup(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	// Intentionally expose only the canonical spot market key.
	srv := newMidsServer(t, `{"PURR/USDC":"1.2"}`)
	defer srv.Close()

	exec := NewExecutor(s, client.NewClient(srv.URL), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:       10000,
			Coin:          "PURR",
			CanonicalCoin: "PURR/USDC",
			SzDecimals:    2,
			IsSpot:        true,
		},
	}, false)

	result, err := exec.PlaceMarketOrder(context.Background(), PlaceMarketOrderInput{
		Coin:            "PURR",
		Side:            "buy",
		Size:            decimal.NewFromInt(10),
		SlippagePercent: decimal.NewFromFloat(0.5),
		DryRun:          true,
	})
	if err != nil {
		t.Fatalf("PlaceMarketOrder spot dry-run error: %v", err)
	}

	if got := result.Action.Orders[0].P; got != "1.206" {
		t.Errorf("price = %q, want 1.206", got)
	}
}

func TestExecutor_PlaceMarketOrder_SnapsPriceToWireRules(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	srv := newMidsServer(t, `{"BTC":"12345.6789"}`)
	defer srv.Close()

	exec := NewExecutor(s, client.NewClient(srv.URL), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:       0,
			Coin:          "BTC",
			CanonicalCoin: "BTC",
			SzDecimals:    3,
			IsSpot:        false,
		},
	}, false)

	result, err := exec.PlaceMarketOrder(context.Background(), PlaceMarketOrderInput{
		Coin:            "BTC",
		Side:            "buy",
		Size:            decimal.NewFromFloat(0.1),
		SlippagePercent: decimal.Zero,
		DryRun:          true,
	})
	if err != nil {
		t.Fatalf("PlaceMarketOrder dry-run error: %v", err)
	}

	rawPx, err := decimal.NewFromString("12345.6789")
	if err != nil {
		t.Fatalf("parse raw price: %v", err)
	}
	want := wire.NearestValidPrice(rawPx, 3, false).String()
	if got := result.Action.Orders[0].P; got != want {
		t.Errorf("price = %q, want %q", got, want)
	}
}

func TestExecutor_PlaceMarketOrder_MissingMid(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	srv := newMidsServer(t, `{"BTC":"50000"}`)
	defer srv.Close()

	exec := NewExecutor(s, client.NewClient(srv.URL), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:       1,
			Coin:          "ETH",
			CanonicalCoin: "ETH",
			SzDecimals:    4,
			IsSpot:        false,
		},
	}, false)

	_, err = exec.PlaceMarketOrder(context.Background(), PlaceMarketOrderInput{
		Coin:            "ETH",
		Side:            "buy",
		Size:            decimal.NewFromInt(1),
		SlippagePercent: decimal.NewFromFloat(0.5),
		DryRun:          true,
	})
	if err == nil {
		t.Fatal("expected missing mid error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("code = %q, want %q", cliErr.Code, output.ErrValidation)
	}
}

func TestExecutor_PlaceMarketOrder_RejectsInvalidSlippage(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	tests := []struct {
		name     string
		slippage decimal.Decimal
	}{
		{name: "negative", slippage: decimal.NewFromFloat(-0.1)},
		{name: "too large", slippage: decimal.NewFromInt(100)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := exec.PlaceMarketOrder(context.Background(), PlaceMarketOrderInput{
				Coin:            "BTC",
				Side:            "buy",
				Size:            decimal.NewFromInt(1),
				SlippagePercent: tc.slippage,
				DryRun:          true,
			})
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestExecutor_PlaceOrder_SendsToExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","response":{"type":"order"}}`)
	}))
	defer srv.Close()

	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	c := client.NewClient(srv.URL)
	r := &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
		},
	}

	exec := NewExecutor(s, c, r, false)

	result, err := exec.PlaceOrder(context.Background(), PlaceOrderInput{
		Coin:  "BTC",
		Side:  "buy",
		Price: decimal.NewFromInt(50000),
		Size:  decimal.NewFromFloat(0.01),
		Tif:   "Gtc",
	})
	if err != nil {
		t.Fatalf("PlaceOrder error: %v", err)
	}
	if result.Response == nil {
		t.Fatal("expected response from exchange")
	}

	var resp map[string]any
	json.Unmarshal(result.Response, &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
}

func TestExecutor_PlaceOrder_WithTpSlTriggers(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	c := client.NewClient("http://unused")
	r := &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
			IsSpot:     false,
		},
	}

	exec := NewExecutor(s, c, r, false)

	tp := "55000"
	sl := "45000"
	result, err := exec.PlaceOrder(context.Background(), PlaceOrderInput{
		Coin:      "BTC",
		Side:      "buy",
		Price:     decimal.NewFromInt(50000),
		Size:      decimal.NewFromFloat(0.01),
		Tif:       "Gtc",
		TpTrigger: &tp,
		SlTrigger: &sl,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("PlaceOrder with triggers error: %v", err)
	}

	action := result.Action

	// Expect 3 wires: main limit + TP trigger + SL trigger.
	if len(action.Orders) != 3 {
		t.Fatalf("expected 3 order wires, got %d", len(action.Orders))
	}
	if action.Grouping != "normalTpsl" {
		t.Errorf("Grouping = %q, want normalTpSl", action.Grouping)
	}

	// Main order: buy limit.
	main := action.Orders[0]
	if main.B != true {
		t.Error("main order should be buy")
	}
	if main.T.Limit == nil {
		t.Fatal("main order should be Limit type")
	}
	if main.T.Limit.Tif != "Gtc" {
		t.Errorf("main Tif = %q, want Gtc", main.T.Limit.Tif)
	}

	// TP trigger: sell (opposite), reduce-only, trigger type.
	tpWire := action.Orders[1]
	if tpWire.B != false {
		t.Error("TP trigger should be sell (opposite of buy)")
	}
	if !tpWire.R {
		t.Error("TP trigger should be reduce-only")
	}
	if tpWire.T.Trigger == nil {
		t.Fatal("TP order should be Trigger type")
	}
	if tpWire.T.Trigger.TriggerPx != "55000" {
		t.Errorf("TP TriggerPx = %q, want 55000", tpWire.T.Trigger.TriggerPx)
	}
	if tpWire.T.Trigger.Tpsl != "tp" {
		t.Errorf("TP Tpsl = %q, want tp", tpWire.T.Trigger.Tpsl)
	}

	// SL trigger: sell (opposite), reduce-only, trigger type.
	slWire := action.Orders[2]
	if slWire.B != false {
		t.Error("SL trigger should be sell (opposite of buy)")
	}
	if !slWire.R {
		t.Error("SL trigger should be reduce-only")
	}
	if slWire.T.Trigger == nil {
		t.Fatal("SL order should be Trigger type")
	}
	if slWire.T.Trigger.TriggerPx != "45000" {
		t.Errorf("SL TriggerPx = %q, want 45000", slWire.T.Trigger.TriggerPx)
	}
	if slWire.T.Trigger.Tpsl != "sl" {
		t.Errorf("SL Tpsl = %q, want sl", slWire.T.Trigger.Tpsl)
	}
}

func TestExecutor_PlaceOrder_TpOnlyTrigger(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{AssetID: 3, Coin: "ETH", SzDecimals: 4},
	}, false)

	tp := "4000"
	result, err := exec.PlaceOrder(context.Background(), PlaceOrderInput{
		Coin:      "ETH",
		Side:      "sell",
		Price:     decimal.NewFromInt(3500),
		Size:      decimal.NewFromFloat(1.0),
		Tif:       "Gtc",
		TpTrigger: &tp,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("PlaceOrder with TP-only error: %v", err)
	}

	// 2 wires: main limit + TP trigger.
	action := result.Action
	if len(action.Orders) != 2 {
		t.Fatalf("expected 2 order wires, got %d", len(action.Orders))
	}
	if action.Grouping != "normalTpsl" {
		t.Errorf("Grouping = %q, want normalTpSl", action.Grouping)
	}

	// TP trigger is buy (opposite of sell).
	tpWire := action.Orders[1]
	if tpWire.B != true {
		t.Error("TP trigger on sell order should be buy")
	}
	if tpWire.T.Trigger.Tpsl != "tp" {
		t.Errorf("Tpsl = %q, want tp", tpWire.T.Trigger.Tpsl)
	}
}

func TestExecutor_CancelOrders_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	result, err := exec.CancelOrders(context.Background(), []CancelWire{
		{A: 0, O: 12345},
	}, "", true, nil)
	if err != nil {
		t.Fatalf("CancelOrders dry-run error: %v", err)
	}

	var action CancelAction
	if err := json.Unmarshal(result, &action); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if action.Type != "cancel" {
		t.Errorf("Type = %q, want cancel", action.Type)
	}
	if action.Cancels[0].O != 12345 {
		t.Errorf("OID = %d, want 12345", action.Cancels[0].O)
	}
}

func TestExecutor_CancelByCloid_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	result, err := exec.CancelByCloid(context.Background(), []CancelByCloidWire{
		{Asset: 0, Cloid: "my-id"},
	}, "", true, nil)
	if err != nil {
		t.Fatalf("CancelByCloid dry-run error: %v", err)
	}

	var action CancelByCloidAction
	if err := json.Unmarshal(result, &action); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if action.Cancels[0].Cloid != "my-id" {
		t.Errorf("Cloid = %q, want my-id", action.Cancels[0].Cloid)
	}
}

func TestExecutor_UpdateLeverage_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    1,
			Coin:       "ETH",
			SzDecimals: 4,
		},
	}, false)

	result, err := exec.UpdateLeverage(context.Background(), UpdateLeverageInput{
		Coin:     "ETH",
		IsCross:  true,
		Leverage: 10,
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("UpdateLeverage dry-run error: %v", err)
	}

	var action UpdateLeverageAction
	if err := json.Unmarshal(result, &action); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if action.Type != "updateLeverage" {
		t.Errorf("Type = %q, want updateLeverage", action.Type)
	}
	if action.Asset != 1 {
		t.Errorf("Asset = %d, want 1", action.Asset)
	}
	if !action.IsCross {
		t.Error("IsCross = false, want true")
	}
	if action.Leverage != 10 {
		t.Errorf("Leverage = %d, want 10", action.Leverage)
	}
}

func TestExecutor_UpdateLeverage_InvalidLeverage(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{AssetID: 0, Coin: "BTC", SzDecimals: 3},
	}, false)

	_, err = exec.UpdateLeverage(context.Background(), UpdateLeverageInput{
		Coin:     "BTC",
		Leverage: 0,
		DryRun:   true,
	})
	if err == nil {
		t.Fatal("expected validation error for leverage < 1")
	}
}

func TestExecutor_UpdateIsolatedMargin_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
		},
	}, false)

	result, err := exec.UpdateIsolatedMargin(context.Background(), UpdateIsolatedMarginInput{
		Coin:   "BTC",
		IsBuy:  true,
		Amount: decimal.NewFromFloat(100.5),
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("UpdateIsolatedMargin dry-run error: %v", err)
	}

	var action UpdateIsolatedMarginAction
	if err := json.Unmarshal(result, &action); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if action.Ntli != 100500000 {
		t.Errorf("Ntli = %d, want 100500000", action.Ntli)
	}
}

func TestExecutor_UpdateIsolatedMargin_NegativeAmount(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
		},
	}, false)

	result, err := exec.UpdateIsolatedMargin(context.Background(), UpdateIsolatedMarginInput{
		Coin:   "BTC",
		IsBuy:  false,
		Amount: decimal.NewFromFloat(-50.0),
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("UpdateIsolatedMargin negative amount error: %v", err)
	}

	var action UpdateIsolatedMarginAction
	if err := json.Unmarshal(result, &action); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if action.Ntli != -50000000 {
		t.Errorf("Ntli = %d, want -50000000", action.Ntli)
	}
}

func TestExecutor_UpdateIsolatedMargin_AmountOverflow(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
		},
	}, false)

	amount, err := decimal.NewFromString("9223372036854.775808")
	if err != nil {
		t.Fatalf("parse amount: %v", err)
	}

	_, err = exec.UpdateIsolatedMargin(context.Background(), UpdateIsolatedMarginInput{
		Coin:   "BTC",
		IsBuy:  true,
		Amount: amount,
		DryRun: true,
	})
	if err == nil {
		t.Fatal("expected validation error for out-of-range ntli")
	}
}

func TestExecutor_ModifyOrder_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), &mockResolver{
		info: &resolver.AssetInfo{
			AssetID:    0,
			Coin:       "BTC",
			SzDecimals: 3,
			IsSpot:     false,
		},
	}, false)

	result, err := exec.ModifyOrder(context.Background(), ModifyOrderInput{
		Coin:   "BTC",
		Oid:    12345,
		Side:   "buy",
		Price:  decimal.NewFromInt(50000),
		Size:   decimal.NewFromFloat(0.01),
		Tif:    "Gtc",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("ModifyOrder dry-run error: %v", err)
	}

	if result.Action == nil {
		t.Fatal("expected action in dry-run result")
	}
	if result.Response != nil {
		t.Error("expected nil response in dry-run")
	}
	modAction := result.Action
	if modAction.Oid != 12345 {
		t.Errorf("Oid = %d, want 12345", modAction.Oid)
	}
	if result.Resolved == nil {
		t.Fatal("expected resolved in dry-run result")
	}
	if result.Resolved.Coin != "BTC" {
		t.Errorf("Resolved.Coin = %q, want BTC", result.Resolved.Coin)
	}
	if result.Resolved.Price != "50000" {
		t.Errorf("Resolved.Price = %q, want 50000", result.Resolved.Price)
	}
}

func TestExecutor_ScheduleCancel_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	cancelTime := int64(1700000000000)
	result, err := exec.ScheduleCancel(context.Background(), ScheduleCancelInput{
		Time:   &cancelTime,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("ScheduleCancel dry-run error: %v", err)
	}

	var action ScheduleCancelAction
	if err := json.Unmarshal(result, &action); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if action.Type != "scheduleCancel" {
		t.Errorf("Type = %q, want scheduleCancel", action.Type)
	}
	if action.Time == nil || *action.Time != 1700000000000 {
		t.Errorf("Time = %v, want 1700000000000", action.Time)
	}
}

func TestExecutor_ScheduleCancel_Clear_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	result, err := exec.ScheduleCancel(context.Background(), ScheduleCancelInput{
		Time:   nil,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("ScheduleCancel clear dry-run error: %v", err)
	}

	var action ScheduleCancelAction
	if err := json.Unmarshal(result, &action); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if action.Time != nil {
		t.Errorf("Time = %v, want nil", action.Time)
	}
}

func TestExecutor_USDClassTransfer_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	raw, err := exec.USDClassTransfer(context.Background(), USDClassTransferInput{
		Amount: decimal.NewFromFloat(12.34),
		ToPerp: true,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("USDClassTransfer dry-run error: %v", err)
	}

	var action map[string]any
	if err := json.Unmarshal(raw, &action); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if action["type"] != "usdClassTransfer" {
		t.Errorf("type = %v, want usdClassTransfer", action["type"])
	}
	if action["amount"] != "12.34" {
		t.Errorf("amount = %v, want 12.34", action["amount"])
	}
	if action["toPerp"] != true {
		t.Errorf("toPerp = %v, want true", action["toPerp"])
	}
	if action["hyperliquidChain"] != "Testnet" {
		t.Errorf("hyperliquidChain = %v, want Testnet", action["hyperliquidChain"])
	}
	if action["signatureChainId"] != "0x66eee" {
		t.Errorf("signatureChainId = %v, want 0x66eee", action["signatureChainId"])
	}
}

func TestExecutor_ClassTransfer_DryRunUsesUSDClassTransferAction(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	raw, err := exec.ClassTransfer(context.Background(), ClassTransferInput{
		Amount: decimal.NewFromFloat(1.5),
		ToPerp: false,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("ClassTransfer dry-run error: %v", err)
	}

	var action map[string]any
	if err := json.Unmarshal(raw, &action); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if action["type"] != "usdClassTransfer" {
		t.Errorf("type = %v, want usdClassTransfer", action["type"])
	}
	if action["toPerp"] != false {
		t.Errorf("toPerp = %v, want false", action["toPerp"])
	}
}

func TestExecutor_Withdraw3_InvalidDestination(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}
	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	_, err = exec.Withdraw3(context.Background(), Withdraw3Input{
		Destination: "not-an-address",
		Amount:      decimal.NewFromInt(1),
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid destination")
	}
}

func TestExecutor_ApproveAgent_InvalidAddress(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}
	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	_, err = exec.ApproveAgent(context.Background(), ApproveAgentInput{
		AgentAddress: "bad",
		DryRun:       true,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid agent address")
	}
}

func TestExecutor_ApproveAgent_RevokeDryRunOmitsAgentName(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}
	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	raw, err := exec.ApproveAgent(context.Background(), ApproveAgentInput{
		AgentAddress: "0x1111111111111111111111111111111111111111",
		AgentName:    "",
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("ApproveAgent dry-run error: %v", err)
	}

	var action map[string]any
	if err := json.Unmarshal(raw, &action); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, exists := action["agentName"]; exists {
		t.Fatalf("agentName should be omitted on revoke-style payload, got %#v", action["agentName"])
	}
}

func TestExecutor_SpotSend_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	raw, err := exec.SpotSend(context.Background(), SpotSendInput{
		Destination: "0x1111111111111111111111111111111111111111",
		Token:       "PURR:0x1",
		Amount:      decimal.NewFromFloat(3.5),
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("SpotSend dry-run error: %v", err)
	}

	var action map[string]any
	if err := json.Unmarshal(raw, &action); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if action["type"] != "spotSend" {
		t.Errorf("type = %v, want spotSend", action["type"])
	}
	if action["token"] != "PURR:0x1" {
		t.Errorf("token = %v, want PURR:0x1", action["token"])
	}
	if action["amount"] != "3.5" {
		t.Errorf("amount = %v, want 3.5", action["amount"])
	}
}

func TestExecutor_UserSetAbstraction_SendsToExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/exchange" {
			http.NotFound(w, r)
			return
		}

		var req map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		var action map[string]any
		if err := json.Unmarshal(req["action"], &action); err != nil {
			t.Fatalf("decode action: %v", err)
		}
		if action["type"] != "userSetAbstraction" {
			t.Errorf("action.type = %v, want userSetAbstraction", action["type"])
		}
		if action["hyperliquidChain"] != "Testnet" {
			t.Errorf("hyperliquidChain = %v, want Testnet", action["hyperliquidChain"])
		}
		if action["signatureChainId"] != "0x66eee" {
			t.Errorf("signatureChainId = %v, want 0x66eee", action["signatureChainId"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","response":{"type":"default"}}`)
	}))
	defer srv.Close()

	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}
	exec := NewExecutor(s, client.NewClient(srv.URL), nil, false)

	resp, err := exec.UserSetAbstraction(context.Background(), UserSetAbstractionInput{
		User:        "0x1111111111111111111111111111111111111111",
		Abstraction: "disabled",
	})
	if err != nil {
		t.Fatalf("UserSetAbstraction error: %v", err)
	}
	if len(resp) == 0 {
		t.Fatal("expected non-empty response")
	}
}

func TestExecutor_UserSetAbstraction_InvalidValue(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}
	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	_, err = exec.UserSetAbstraction(context.Background(), UserSetAbstractionInput{
		User:        "0x1111111111111111111111111111111111111111",
		Abstraction: "none",
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected validation error for unsupported abstraction")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("code = %q, want %q", cliErr.Code, output.ErrValidation)
	}
}
