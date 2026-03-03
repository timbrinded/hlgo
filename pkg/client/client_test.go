package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/timbrinded/hlgo/pkg/output"
)

// newTestServer creates an httptest.Server with the given handler.
func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestPostInfo_Success(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Verify request body.
		body, _ := io.ReadAll(r.Body)
		var req map[string]string
		json.Unmarshal(body, &req)
		if req["type"] != "allMids" {
			t.Errorf("expected type=allMids, got %s", req["type"])
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"BTC":"95123.5","ETH":"3412.1"}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err != nil {
		t.Fatalf("PostInfo returned error: %v", err)
	}

	// Verify we got the expected JSON back.
	var mids map[string]string
	if err := json.Unmarshal(result, &mids); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if mids["BTC"] != "95123.5" {
		t.Errorf("BTC mid = %q, want %q", mids["BTC"], "95123.5")
	}
	if mids["ETH"] != "3412.1" {
		t.Errorf("ETH mid = %q, want %q", mids["ETH"], "3412.1")
	}
}

func TestPostExchange_Success(t *testing.T) {
	expiresAfter := int64(1700000012345)
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]json.RawMessage
		json.Unmarshal(body, &req)

		// Verify envelope fields.
		var nonce json.Number
		json.Unmarshal(req["nonce"], &nonce)
		if nonce.String() != "1700000000000" {
			t.Errorf("nonce = %s, want 1700000000000", nonce)
		}

		var sig SignatureWire
		json.Unmarshal(req["signature"], &sig)
		if sig.R != "0xdead" || sig.S != "0xbeef" || sig.V != 27 {
			t.Errorf("signature = %+v, want {R:0xdead S:0xbeef V:27}", sig)
		}

		// Vault address should be present.
		var vault string
		json.Unmarshal(req["vaultAddress"], &vault)
		if vault != "0xvault" {
			t.Errorf("vaultAddress = %q, want %q", vault, "0xvault")
		}

		var gotExpiresAfter json.Number
		if err := json.Unmarshal(req["expiresAfter"], &gotExpiresAfter); err != nil {
			t.Fatalf("failed to decode expiresAfter: %v", err)
		}
		if gotExpiresAfter.String() != "1700000012345" {
			t.Errorf("expiresAfter = %s, want 1700000012345", gotExpiresAfter)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","response":{"type":"order","data":{"statuses":[{"filled":{"totalSz":"1.5","avgPx":"95000"}}]}}}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	action := map[string]any{"type": "order", "orders": []any{}}
	result, err := c.PostExchange(context.Background(), action, 1700000000000, SignatureWire{R: "0xdead", S: "0xbeef", V: 27}, "0xvault", &expiresAfter)
	if err != nil {
		t.Fatalf("PostExchange returned error: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestPostExchange_OmitsEmptyVaultAddress(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var raw map[string]json.RawMessage
		json.Unmarshal(body, &raw)

		if _, exists := raw["vaultAddress"]; exists {
			t.Error("vaultAddress should be omitted when empty")
		}
		if _, exists := raw["expiresAfter"]; exists {
			t.Error("expiresAfter should be omitted when empty")
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.PostExchange(context.Background(), map[string]string{"type": "cancel"}, 1700000000000, SignatureWire{R: "0x1", S: "0x2", V: 27}, "", nil)
	if err != nil {
		t.Fatalf("PostExchange returned error: %v", err)
	}
}

func TestPostExchange_StatusErrReturnsAPIError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"err","response":"invalid agent_name."}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.PostExchange(
		context.Background(),
		map[string]string{"type": "approveAgent"},
		1700000000000,
		SignatureWire{R: "0x1", S: "0x2", V: 27},
		"",
		nil,
	)
	if err == nil {
		t.Fatal("expected error for exchange status=err")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrAPI {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrAPI)
	}
	if cliErr.Message != "exchange error: invalid agent_name." {
		t.Errorf("message = %q, want %q", cliErr.Message, "exchange error: invalid agent_name.")
	}
	if cliErr.Details["path"] != "/exchange" {
		t.Errorf("path = %v, want /exchange", cliErr.Details["path"])
	}
	if cliErr.Details["exchange_status"] != "err" {
		t.Errorf("exchange_status = %v, want err", cliErr.Details["exchange_status"])
	}
	if cliErr.Details["exchange_response"] != "invalid agent_name." {
		t.Errorf("exchange_response = %v, want invalid agent_name.", cliErr.Details["exchange_response"])
	}
}

func TestPostExchange_OrderStatusesErrorReturnsAPIError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","response":{"type":"order","data":{"statuses":[{"error":"Order price cannot be more than 80% away from the reference price"}]}}}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.PostExchange(
		context.Background(),
		map[string]string{"type": "order"},
		1700000000000,
		SignatureWire{R: "0x1", S: "0x2", V: 27},
		"",
		nil,
	)
	if err == nil {
		t.Fatal("expected error for exchange data.statuses[].error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrAPI {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrAPI)
	}
	if cliErr.Message != "exchange action returned error statuses" {
		t.Errorf("message = %q, want %q", cliErr.Message, "exchange action returned error statuses")
	}

	rawErrors, ok := cliErr.Details["exchange_errors"].([]string)
	if !ok {
		t.Fatalf("exchange_errors is not []string: %#v", cliErr.Details["exchange_errors"])
	}
	if len(rawErrors) != 1 {
		t.Fatalf("exchange_errors len = %d, want 1", len(rawErrors))
	}
	if rawErrors[0] != "Order price cannot be more than 80% away from the reference price" {
		t.Errorf("exchange_errors[0] = %q", rawErrors[0])
	}
}

func TestPostExchange_WaitingForFillStatusesAreAccepted(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","response":{"type":"order","data":{"statuses":["success","waitingForFill",{"error":"waitingForFill"},{"resting":{"oid":123}}]}}}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.PostExchange(
		context.Background(),
		map[string]string{"type": "order"},
		1700000000000,
		SignatureWire{R: "0x1", S: "0x2", V: 27},
		"",
		nil,
	)
	if err != nil {
		t.Fatalf("expected waitingForFill statuses to be treated as benign, got error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected non-empty exchange response")
	}
}

func TestPostExchange_MixedOrderStatusesDetectsError(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","response":{"type":"cancel","data":{"statuses":["success","waitingForFill",{"error":"order was already canceled"},"success"]}}}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.PostExchange(
		context.Background(),
		map[string]string{"type": "cancel"},
		1700000000000,
		SignatureWire{R: "0x1", S: "0x2", V: 27},
		"",
		nil,
	)
	if err == nil {
		t.Fatal("expected error for mixed statuses containing an error entry")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrAPI {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrAPI)
	}
	rawErrors, ok := cliErr.Details["exchange_errors"].([]string)
	if !ok {
		t.Fatalf("exchange_errors is not []string: %#v", cliErr.Details["exchange_errors"])
	}
	if len(rawErrors) != 1 || rawErrors[0] != "order was already canceled" {
		t.Fatalf("unexpected exchange_errors: %#v", rawErrors)
	}
}

func TestPostInfo_4xx_NotRetried(t *testing.T) {
	var callCount atomic.Int32

	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid request"}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithRetries(3))
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "bad"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	// 4xx must NOT be retried.
	if got := callCount.Load(); got != 1 {
		t.Errorf("4xx was called %d times, want exactly 1 (no retries)", got)
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrAPI {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrAPI)
	}
	if cliErr.Details["status_code"] != http.StatusBadRequest {
		t.Errorf("status_code = %v, want %d", cliErr.Details["status_code"], http.StatusBadRequest)
	}
}

func TestPostInfo_5xx_Retried_EventuallyFails(t *testing.T) {
	var callCount atomic.Int32

	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `internal server error`)
	})
	defer srv.Close()

	maxRetries := 2
	c := NewClient(srv.URL, WithRetries(maxRetries))
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}

	// Should be called 1 (initial) + maxRetries times.
	expectedCalls := int32(1 + maxRetries)
	if got := callCount.Load(); got != expectedCalls {
		t.Errorf("5xx call count = %d, want %d (1 initial + %d retries)", got, expectedCalls, maxRetries)
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrAPI {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrAPI)
	}
}

func TestPostInfo_5xx_ThenSuccess(t *testing.T) {
	var callCount atomic.Int32

	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		n := callCount.Add(1)
		if n <= 2 {
			// First two calls: 500.
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `server error`)
			return
		}
		// Third call: success.
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"recovered":true}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithRetries(2))
	result, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}

	var data map[string]bool
	json.Unmarshal(result, &data)
	if !data["recovered"] {
		t.Error("expected recovered=true in response")
	}

	if got := callCount.Load(); got != 3 {
		t.Errorf("call count = %d, want 3 (2 failures + 1 success)", got)
	}
}

func TestPostInfo_RateLimit_429(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `rate limited`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrRateLimit {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrRateLimit)
	}
}

func TestPostInfo_NetworkError(t *testing.T) {
	// Use a server that's already closed to simulate a network error.
	srv := newTestServer(func(http.ResponseWriter, *http.Request) {})
	srv.Close()

	c := NewClient(srv.URL, WithRetries(0))
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected error for closed server")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrNetwork {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrNetwork)
	}
}

func TestPostInfo_Timeout(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		// Sleep longer than the client timeout.
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithTimeout(50*time.Millisecond), WithRetries(0))
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrNetwork {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrNetwork)
	}
}

func TestPostInfo_ContextCancelled(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	c := NewClient(srv.URL, WithRetries(0))
	_, err := c.PostInfo(ctx, map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrNetwork {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrNetwork)
	}
}

func TestPostInfo_UseNumber_PreservesFinancialPrecision(t *testing.T) {
	// Return a JSON number that would lose precision as float64.
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// This number has too many digits for float64 to represent exactly.
		fmt.Fprint(w, `{"price":"95123.5","qty":12345678901234567}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err != nil {
		t.Fatalf("PostInfo returned error: %v", err)
	}

	// Decode using json.Number to verify the raw value is preserved.
	dec := json.NewDecoder(bytes.NewReader(result))
	dec.UseNumber()
	var data map[string]any
	if err := dec.Decode(&data); err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}

	// The qty field should be preserved as json.Number, not mangled by float64 conversion.
	qty, ok := data["qty"].(json.Number)
	if !ok {
		t.Fatalf("qty is %T, expected json.Number", data["qty"])
	}
	if qty.String() != "12345678901234567" {
		t.Errorf("qty = %s, want 12345678901234567 (precision lost?)", qty)
	}
}

func TestPostInfo_InvalidResponseJSON(t *testing.T) {
	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `not valid json`)
	})
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrAPI {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrAPI)
	}
}

func TestWithRetries_ZeroMeansNoRetries(t *testing.T) {
	var callCount atomic.Int32

	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `error`)
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithRetries(0))
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected error")
	}

	if got := callCount.Load(); got != 1 {
		t.Errorf("call count = %d, want 1 (no retries)", got)
	}
}

func TestWithRetries_NegativeClampedToZero(t *testing.T) {
	var callCount atomic.Int32

	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `error`)
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithRetries(-5))
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected error")
	}

	if got := callCount.Load(); got != 1 {
		t.Errorf("call count = %d, want 1 (negative retries clamped to 0)", got)
	}
}

func TestPostInfo_429_Retried(t *testing.T) {
	var callCount atomic.Int32

	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `rate limited`)
	})
	defer srv.Close()

	maxRetries := 2
	c := NewClient(srv.URL, WithRetries(maxRetries))
	_, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err == nil {
		t.Fatal("expected error for 429")
	}

	// 429 is transient — should be retried like 5xx.
	expectedCalls := int32(1 + maxRetries)
	if got := callCount.Load(); got != expectedCalls {
		t.Errorf("429 was called %d times, want %d (1 initial + %d retries)", got, expectedCalls, maxRetries)
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrRateLimit {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrRateLimit)
	}
}

func TestPostInfo_429_ThenSuccess(t *testing.T) {
	var callCount atomic.Int32

	srv := newTestServer(func(w http.ResponseWriter, _ *http.Request) {
		n := callCount.Add(1)
		if n <= 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `rate limited`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"recovered":true}`)
	})
	defer srv.Close()

	c := NewClient(srv.URL, WithRetries(2))
	result, err := c.PostInfo(context.Background(), map[string]string{"type": "allMids"})
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}

	var data map[string]bool
	json.Unmarshal(result, &data)
	if !data["recovered"] {
		t.Error("expected recovered=true in response")
	}

	if got := callCount.Load(); got != 2 {
		t.Errorf("call count = %d, want 2 (1 rate-limit + 1 success)", got)
	}
}

func TestDefaultClientSettings(t *testing.T) {
	c := NewClient("https://api.hyperliquid.xyz")

	if c.baseURL != "https://api.hyperliquid.xyz" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://api.hyperliquid.xyz")
	}
	if c.httpClient.Timeout != defaultTimeout {
		t.Errorf("timeout = %v, want %v", c.httpClient.Timeout, defaultTimeout)
	}
	if c.maxRetries != defaultMaxRetries {
		t.Errorf("maxRetries = %d, want %d", c.maxRetries, defaultMaxRetries)
	}
}
