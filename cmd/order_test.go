package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestOrderPlace_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	_ = run("info") // warm up — info alone just prints help

	err := run("order", "place",
		"--coin", "BTC", "--side", "buy", "--price", "50000", "--size", "0.01",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["action"] == nil {
		t.Error("expected 'action' in dry-run output")
	}
	if result["resolved"] == nil {
		t.Error("expected 'resolved' in dry-run output")
	}
}

func TestOrderPlace_RequiredFlags(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	// Missing --coin, --side, --price, --size.
	err := run("order", "place", "--dry-run")
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestOrderPlace_InvalidSide(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "place",
		"--coin", "BTC", "--side", "long", "--price", "50000", "--size", "0.01",
		"--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid side")
	}
}

func TestOrderPlace_InvalidTif(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "place",
		"--coin", "BTC", "--side", "buy", "--price", "50000", "--size", "0.01",
		"--tif", "fok", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid TIF")
	}
}

func TestOrderMarket_InvalidSlippage(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "market",
		"--coin", "BTC", "--side", "buy", "--size", "0.01",
		"--slippage", "abc", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid slippage")
	}
}

func TestOrderMarket_NegativeSlippage(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "market",
		"--coin", "BTC", "--side", "buy", "--size", "0.01",
		"--slippage", "-0.1", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for negative slippage")
	}
}

func TestOrderMarket_TooLargeSlippage(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "market",
		"--coin", "BTC", "--side", "sell", "--size", "0.01",
		"--slippage", "100", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for slippage >= 100")
	}
}

func TestOrderPlace_WithCloid(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("order", "place",
		"--coin", "BTC", "--side", "sell", "--price", "100000", "--size", "0.001",
		"--cloid", "my-order-123", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	json.Unmarshal(stdout.Bytes(), &result)
	action, ok := result["action"].(map[string]any)
	if !ok {
		t.Fatal("expected action map")
	}
	orders, ok := action["orders"].([]any)
	if !ok || len(orders) == 0 {
		t.Fatal("expected orders array")
	}
	order := orders[0].(map[string]any)
	if order["c"] != "my-order-123" {
		t.Errorf("cloid = %v, want my-order-123", order["c"])
	}
}

func TestOrderPlace_WithBuilder_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("order", "place",
		"--coin", "BTC", "--side", "buy", "--price", "50000", "--size", "0.01",
		"--builder", "0x1234567890abcdef1234567890abcdef12345678",
		"--builder-fee-tenths-bp", "10",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	action, ok := result["action"].(map[string]any)
	if !ok {
		t.Fatal("expected action map")
	}
	builder, ok := action["builder"].(map[string]any)
	if !ok {
		t.Fatal("expected builder map")
	}
	if builder["b"] != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("builder.b = %v", builder["b"])
	}
	if builder["f"] != float64(10) {
		t.Errorf("builder.f = %v, want 10", builder["f"])
	}
}

func TestOrderPlace_BuilderRequiresFee(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "place",
		"--coin", "BTC", "--side", "buy", "--price", "50000", "--size", "0.01",
		"--builder", "0x1234567890abcdef1234567890abcdef12345678",
		"--dry-run",
	)
	if err == nil {
		t.Fatal("expected error when builder fee is missing")
	}
}

func TestOrderCancel_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("order", "cancel",
		"--coin", "BTC", "--oid", "12345", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if result["type"] != "cancel" {
		t.Errorf("type = %v, want cancel", result["type"])
	}
}

func TestOrderCancel_MutualExclusion(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	// Both --oid and --cloid.
	err := run("order", "cancel",
		"--coin", "BTC", "--oid", "12345", "--cloid", "abc", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}
}

func TestOrderCancel_NeitherOidNorCloid(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "cancel", "--coin", "BTC", "--dry-run")
	if err == nil {
		t.Fatal("expected error when neither --oid nor --cloid provided")
	}
}

func TestOrderSubcommands_AllRegistered(t *testing.T) {
	root := NewRootCommand("test")
	var orderCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Use == "order" {
			orderCmd = c
			break
		}
	}
	if orderCmd == nil {
		t.Fatal("order command not found")
	}

	want := []string{"place", "market", "cancel", "cancel-all", "modify", "batch", "schedule-cancel"}
	cmds := make(map[string]bool)
	for _, sub := range orderCmd.Commands() {
		cmds[sub.Name()] = true
	}
	for _, name := range want {
		if !cmds[name] {
			t.Errorf("subcommand %q not registered on order", name)
		}
	}
}

func TestOrderModify_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("order", "modify",
		"--coin", "BTC", "--oid", "12345", "--side", "buy",
		"--price", "50000", "--size", "0.01", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["action"] == nil {
		t.Error("expected 'action' in dry-run output")
	}
	if result["resolved"] == nil {
		t.Error("expected 'resolved' in dry-run output")
	}
}

func TestOrderModify_DryRun_PriceOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/info" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"coin":"BTC","side":"B","limitPx":"50000","sz":"0.02","oid":12345}]`))
	}))
	defer srv.Close()

	t.Setenv("HLGO_API_URL", srv.URL)
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("order", "modify",
		"--coin", "BTC", "--oid", "12345", "--side", "buy",
		"--price", "51000", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	action, ok := result["action"].(map[string]any)
	if !ok {
		t.Fatalf("action = %T, want object", result["action"])
	}
	order, ok := action["order"].(map[string]any)
	if !ok {
		t.Fatalf("order = %T, want object", action["order"])
	}
	if got := order["p"]; got != "51000" {
		t.Errorf("price = %v, want 51000", got)
	}
	if got := order["s"]; got != "0.02" {
		t.Errorf("size = %v, want 0.02", got)
	}
}

func TestOrderModify_InvalidVault(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "modify",
		"--coin", "BTC", "--oid", "12345", "--side", "buy",
		"--price", "50000", "--size", "0.01",
		"--vault", "not-a-hex-address", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid vault address")
	}
}

func TestOrderModify_RequiredFlags(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "modify", "--dry-run")
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestOrderModify_RequiresPriceOrSize(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "modify",
		"--coin", "BTC", "--oid", "12345", "--side", "buy", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error when neither --price nor --size is provided")
	}
}

func TestOrderScheduleCancel_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("order", "schedule-cancel", "--timeout", "5m", "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "scheduleCancel" {
		t.Errorf("type = %v, want scheduleCancel", result["type"])
	}
	if result["time"] == nil {
		t.Error("expected 'time' in schedule-cancel output")
	}
}

func TestOrderScheduleCancel_Clear(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("order", "schedule-cancel", "--clear", "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "scheduleCancel" {
		t.Errorf("type = %v, want scheduleCancel", result["type"])
	}
	// When clearing, time should be absent (null/omitted).
	if _, ok := result["time"]; ok {
		t.Error("expected 'time' to be absent in clear output")
	}
}

func TestOrderScheduleCancel_MutualExclusion(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "schedule-cancel", "--timeout", "5m", "--clear", "--dry-run")
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}
}

func TestOrderScheduleCancel_NeitherFlag(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "schedule-cancel", "--dry-run")
	if err == nil {
		t.Fatal("expected error when neither --timeout nor --clear provided")
	}
}

func TestOrderScheduleCancel_TimeoutTooShort(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "schedule-cancel", "--timeout", "1s", "--dry-run")
	if err == nil {
		t.Fatal("expected error when timeout is less than 5 seconds")
	}
}

func TestOrderBatch_RequiredFlags(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("order", "batch", "--dry-run")
	if err == nil {
		t.Fatal("expected error for missing --file flag")
	}
}

func TestOrderBatch_DryRun(t *testing.T) {
	// Create a temporary batch file.
	dir := t.TempDir()
	batchFile := dir + "/orders.json"
	content := `[
		{"coin": "BTC", "side": "buy", "price": "50000", "size": "0.01"},
		{"coin": "ETH", "side": "sell", "price": "3000", "size": "0.1"}
	]`
	os.WriteFile(batchFile, []byte(content), 0600) //nolint:errcheck // test helper

	stdout, _, run := newTestRootWithServer(t, "")

	err := run("order", "batch", "--file", batchFile, "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "order" {
		t.Errorf("type = %v, want order", result["type"])
	}
	orders, ok := result["orders"].([]any)
	if !ok {
		t.Fatal("expected orders array")
	}
	if len(orders) != 2 {
		t.Errorf("orders len = %d, want 2", len(orders))
	}
}
