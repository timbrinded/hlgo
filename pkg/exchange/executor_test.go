package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/resolver"
	"github.com/timbrinded/hlgo/pkg/signer"
)

// mockResolver returns fixed asset info for testing.
type mockResolver struct {
	info *resolver.AssetInfo
	err  error
}

func (m *mockResolver) ResolveAsset(_ context.Context, _ string) (*resolver.AssetInfo, error) {
	return m.info, m.err
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
	if result.Action.Orders[0].P != "50000" {
		t.Errorf("price = %q, want 50000", result.Action.Orders[0].P)
	}
	if result.Action.Builder == nil {
		t.Fatal("expected builder in action")
	}
	if result.Action.Builder.F != 10 {
		t.Errorf("builder fee = %d, want 10", result.Action.Builder.F)
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
	if len(result.Action.Orders) != 2 {
		t.Fatalf("expected 2 order wires, got %d", len(result.Action.Orders))
	}
	if result.Action.Grouping != "normalTpsl" {
		t.Errorf("Grouping = %q, want normalTpSl", result.Action.Grouping)
	}

	// TP trigger is buy (opposite of sell).
	tpWire := result.Action.Orders[1]
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
