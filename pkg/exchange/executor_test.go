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
		Coin:   "BTC",
		Side:   "buy",
		Price:  decimal.NewFromInt(50000),
		Size:   decimal.NewFromFloat(0.01),
		Tif:    "Gtc",
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

func TestExecutor_CancelOrders_DryRun(t *testing.T) {
	s, err := signer.NewSigner(testPrivateKey)
	if err != nil {
		t.Fatalf("creating signer: %v", err)
	}

	exec := NewExecutor(s, client.NewClient("http://unused"), nil, false)

	result, err := exec.CancelOrders(context.Background(), []CancelWire{
		{A: 0, O: 12345},
	}, "", true)
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
	}, "", true)
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
