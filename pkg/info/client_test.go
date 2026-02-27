package info

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/timbrinded/hlgo/pkg/client"
)

func TestInfoClient_AllMids(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		json.Unmarshal(body, &req)
		if req["type"] != "allMids" {
			t.Errorf("type = %q, want allMids", req["type"])
		}
		fmt.Fprint(w, `{"BTC":"95000","ETH":"3400"}`)
	}))
	defer srv.Close()

	ic := NewInfoClient(client.NewClient(srv.URL))
	raw, err := ic.AllMids(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, err := ParseMidsResult(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if m["BTC"] != "95000" {
		t.Errorf("BTC = %q, want 95000", m["BTC"])
	}
}

func TestInfoClient_AllMids_WithDex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		json.Unmarshal(body, &req)
		if req["dex"] != "xyz" {
			t.Errorf("dex = %q, want xyz", req["dex"])
		}
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	ic := NewInfoClient(client.NewClient(srv.URL))
	_, err := ic.AllMids(context.Background(), "xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfoClient_L2Book(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		if req["type"] != "l2Book" {
			t.Errorf("type = %v, want l2Book", req["type"])
		}
		if req["coin"] != "BTC" {
			t.Errorf("coin = %v, want BTC", req["coin"])
		}
		fmt.Fprint(w, `{"coin":"BTC","levels":[[],[]]}`)
	}))
	defer srv.Close()

	ic := NewInfoClient(client.NewClient(srv.URL))
	_, err := ic.L2Book(context.Background(), "BTC", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfoClient_CandleSnapshot_NestedReq(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		if req["type"] != "candleSnapshot" {
			t.Errorf("type = %v, want candleSnapshot", req["type"])
		}
		nested, ok := req["req"].(map[string]any)
		if !ok {
			t.Fatalf("expected req object, got %T", req["req"])
		}
		if nested["coin"] != "BTC" {
			t.Errorf("req.coin = %v, want BTC", nested["coin"])
		}
		if nested["interval"] != "1h" {
			t.Errorf("req.interval = %v, want 1h", nested["interval"])
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	ic := NewInfoClient(client.NewClient(srv.URL))
	_, err := ic.CandleSnapshot(context.Background(), "BTC", "1h", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfoClient_ClearinghouseState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		json.Unmarshal(body, &req)
		if req["type"] != "clearinghouseState" {
			t.Errorf("type = %q, want clearinghouseState", req["type"])
		}
		if req["user"] != "0xabc" {
			t.Errorf("user = %q, want 0xabc", req["user"])
		}
		fmt.Fprint(w, `{"assetPositions":[],"marginSummary":{}}`)
	}))
	defer srv.Close()

	ic := NewInfoClient(client.NewClient(srv.URL))
	_, err := ic.ClearinghouseState(context.Background(), "0xabc", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInfoClient_PredictedFundings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		json.Unmarshal(body, &req)
		if req["type"] != "predictedFundings" {
			t.Errorf("type = %q, want predictedFundings", req["type"])
		}
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	ic := NewInfoClient(client.NewClient(srv.URL))
	_, err := ic.PredictedFundings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
