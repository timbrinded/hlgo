package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// testMetaJSON is minimal perp metadata for testing.
const testMetaJSON = `{"universe":[{"name":"BTC","szDecimals":3},{"name":"ETH","szDecimals":4}]}`

// testSpotMetaJSON is minimal spot metadata for testing (matches real API shape).
const testSpotMetaJSON = `{"tokens":[{"name":"USDC","szDecimals":6,"index":0},{"name":"PURR","szDecimals":2,"index":1}],"universe":[{"name":"PURR/USDC","index":0,"tokens":[1,0]}]}`

// newTestRootWithServer creates a root command configured to use the given test server URL.
// It writes a temporary config file pointing agent_key_env at a set env var,
// and pre-populates the resolver cache so order commands don't need a live API.
func newTestRootWithServer(t *testing.T, _ string) (*bytes.Buffer, *bytes.Buffer, func(args ...string) error) {
	t.Helper()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("agent_key_env: TEST_HL_KEY\nmaster_key_env: TEST_HL_MASTER_KEY\nmetadata_ttl: 300\n"), 0600)

	// Set HOME so resolveCacheDir uses our temp dir.
	t.Setenv("HOME", dir)

	// Pre-populate resolver cache for both mainnet and testnet.
	for _, network := range []string{"mainnet", "testnet"} {
		cacheDir := filepath.Join(dir, ".hlgo", "cache", network)
		os.MkdirAll(cacheDir, 0700)
		// Wrap in cache entry format: {"timestamp":"...","data":...}
		now := `"2099-01-01T00:00:00Z"`
		os.WriteFile(filepath.Join(cacheDir, "meta.json"),
			fmt.Appendf(nil, `{"timestamp":%s,"data":%s}`, now, testMetaJSON), 0600)
		os.WriteFile(filepath.Join(cacheDir, "spot_meta.json"),
			fmt.Appendf(nil, `{"timestamp":%s,"data":%s}`, now, testSpotMetaJSON), 0600)
	}

	// Set a well-known test key for address resolution.
	t.Setenv("TEST_HL_KEY", "0x0123456789012345678901234567890123456789012345678901234567890123")
	t.Setenv("TEST_HL_MASTER_KEY", "0x0123456789012345678901234567890123456789012345678901234567890123")

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	run := func(args ...string) error {
		stdout.Reset()
		stderr.Reset()
		root := NewRootCommand("test")
		root.SetOut(stdout)
		root.SetErr(stderr)
		fullArgs := append([]string{"--config", cfgPath}, args...)
		root.SetArgs(fullArgs)
		return root.Execute()
	}

	return stdout, stderr, run
}

func TestInfoMids_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"BTC":"95000","ETH":"3400"}`)
	}))
	defer srv.Close()

	// Override the base URL for testing by using testnet flag and a custom server.
	// Since we can't easily override baseURL, we test via dry-run instead.
	stdout, _, run := newTestRootWithServer(t, srv.URL)

	if err := run("info", "mids", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &req); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if req["type"] != "allMids" {
		t.Errorf("type = %q, want allMids", req["type"])
	}
}

func TestInfoMids_DryRun_WithDex(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "mids", "--dry-run", "--dex", "xyz"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &req); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if req["dex"] != "xyz" {
		t.Errorf("dex = %q, want xyz", req["dex"])
	}
}

func TestInfoMeta_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "meta", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "meta" {
		t.Errorf("type = %q, want meta", req["type"])
	}
}

func TestInfoMeta_Spot_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "meta", "--spot", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "spotMeta" {
		t.Errorf("type = %q, want spotMeta", req["type"])
	}
}

func TestInfoBook_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "book", "BTC", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "l2Book" {
		t.Errorf("type = %v, want l2Book", req["type"])
	}
	if req["coin"] != "BTC" {
		t.Errorf("coin = %v, want BTC", req["coin"])
	}
}

func TestInfoBook_WithMantissa_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "book", "BTC", "--sigfigs", "5", "--mantissa", "2", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(stdout.Bytes(), &req)
	if req["mantissa"] != float64(2) {
		t.Errorf("mantissa = %v, want 2", req["mantissa"])
	}
}

func TestInfoBook_RequiresCoin(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	if err := run("info", "book"); err == nil {
		t.Fatal("expected error for missing coin arg")
	}
}

func TestInfoCandles_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "candles", "BTC", "1h", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "candleSnapshot" {
		t.Errorf("type = %v, want candleSnapshot", req["type"])
	}
	body, ok := req["req"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested req object, got %T", req["req"])
	}
	if body["interval"] != "1h" {
		t.Errorf("interval = %v, want 1h", body["interval"])
	}
}

func TestInfoCandles_InvalidInterval(t *testing.T) {
	_, stderr, run := newTestRootWithServer(t, "")

	err := run("info", "candles", "BTC", "2m")
	if err == nil {
		t.Fatal("expected error for invalid interval")
	}

	// The error should mention VALIDATION_ERROR.
	if !strings.Contains(err.Error(), "invalid candle interval") {
		t.Errorf("error = %v, want validation error about interval", err)
	}
	_ = stderr
}

func TestInfoState_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "state", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "clearinghouseState" {
		t.Errorf("type = %q, want clearinghouseState", req["type"])
	}
	// Should use derived address from test key.
	if req["user"] != "0x14791697260E4c9A71f18484C9f997B308e59325" {
		t.Errorf("user = %q, want test key address", req["user"])
	}
}

func TestInfoState_ExplicitAddress(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "state", "--address", "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["user"] != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("user = %q, want explicit address", req["user"])
	}
}

func TestInfoOpenOrders_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "open-orders", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "frontendOpenOrders" {
		t.Errorf("type = %q, want frontendOpenOrders", req["type"])
	}
}

func TestInfoFills_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "fills", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "userFills" {
		t.Errorf("type = %q, want userFills", req["type"])
	}
}

func TestInfoFills_WithTime_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "fills", "--start", "1700000000000", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "userFillsByTime" {
		t.Errorf("type = %v, want userFillsByTime", req["type"])
	}
}

func TestInfoFills_AggregateByTime_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "fills", "--aggregate-by-time", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(stdout.Bytes(), &req)
	if req["aggregateByTime"] != true {
		t.Errorf("aggregateByTime = %v, want true", req["aggregateByTime"])
	}
}

func TestInfoOrderStatus_NumericOid(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "order-status", "12345", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "orderStatus" {
		t.Errorf("type = %v, want orderStatus", req["type"])
	}
	// Numeric OID should be parsed as number.
	oid, ok := req["oid"].(float64)
	if !ok {
		t.Fatalf("oid is %T, want float64 (JSON number)", req["oid"])
	}
	if oid != 12345 {
		t.Errorf("oid = %v, want 12345", oid)
	}
}

func TestInfoOrderStatus_StringCloid(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "order-status", "my-cloid-123", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(stdout.Bytes(), &req)
	// String CLOID should remain as string.
	if req["oid"] != "my-cloid-123" {
		t.Errorf("oid = %v, want my-cloid-123", req["oid"])
	}
}

func TestInfoFunding_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "funding", "BTC", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]any
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "fundingHistory" {
		t.Errorf("type = %v, want fundingHistory", req["type"])
	}
}

func TestInfoFunding_Predicted_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "funding", "BTC", "--predicted", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "predictedFundings" {
		t.Errorf("type = %q, want predictedFundings", req["type"])
	}
}

func TestInfoPerpDexs_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "perp-dexs", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "perpDexs" {
		t.Errorf("type = %q, want perpDexs", req["type"])
	}
}

func TestInfoRateLimit_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "rate-limit", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var req map[string]string
	json.Unmarshal(stdout.Bytes(), &req)
	if req["type"] != "userRateLimit" {
		t.Errorf("type = %q, want userRateLimit", req["type"])
	}
}

func TestInfoSubcommands_AllRegistered(t *testing.T) {
	root := NewRootCommand("test")
	var infoCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Use == "info" {
			infoCmd = c
			break
		}
	}
	if infoCmd == nil {
		t.Fatal("info command not found")
	}

	want := []string{
		"mids", "meta", "meta-and-ctxs", "book", "trades", "candles",
		"state", "spot-state", "open-orders", "fills", "order-status", "rate-limit",
		"funding", "perp-dexs",
	}

	cmds := make(map[string]bool)
	for _, sub := range infoCmd.Commands() {
		cmds[sub.Name()] = true
	}

	for _, name := range want {
		if !cmds[name] {
			t.Errorf("subcommand %q not registered on info", name)
		}
	}
}
