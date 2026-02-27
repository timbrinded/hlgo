//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

const e2eTestPrivateKey = "0x0123456789012345678901234567890123456789012345678901234567890123"

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	dir, err := os.MkdirTemp("", "hlgo-e2e")
	if err != nil {
		panic("cannot create temp dir: " + err.Error())
	}
	defer os.RemoveAll(dir)

	binaryPath = filepath.Join(dir, "hlgo")
	cmd := exec.Command("go", "build", "-o", binaryPath, "..")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("cannot build hlgo: " + err.Error())
	}

	// Isolate E2E runs from user-local config and ensure dry-run order tests have a key.
	if os.Getenv("HL_CONFIG") == "" {
		_ = os.Setenv("HL_CONFIG", filepath.Join(dir, "e2e-config.yaml"))
	}
	if os.Getenv("HL_AGENT_KEY") == "" {
		_ = os.Setenv("HL_AGENT_KEY", e2eTestPrivateKey)
	}

	os.Exit(m.Run())
}

// runHlgoOnNetwork runs the hlgo binary against either testnet or mainnet.
func runHlgoOnNetwork(t *testing.T, testnet bool, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	fullArgs := make([]string, 0, len(args)+1)
	if testnet {
		fullArgs = append(fullArgs, "--testnet")
	}
	fullArgs = append(fullArgs, args...)
	cmd := exec.Command(binaryPath, fullArgs...)

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run hlgo: %v", err)
		}
	}
	return stdout, stderr, exitCode
}

// runHlgo runs against testnet by default.
func runHlgo(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	return runHlgoOnNetwork(t, true, args...)
}

// parseJSONOutput parses stdout as JSON into a generic map.
func parseJSONOutput(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout)
	}
	return result
}

// requireExitCode asserts the exit code matches.
func requireExitCode(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("exit code = %d, want %d", got, want)
	}
}

// --- Info E2E tests (read-only, safe to run freely) ---

func TestE2E_InfoMids(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "mids")
	requireExitCode(t, exitCode, 0)

	var mids map[string]string
	if err := json.Unmarshal([]byte(stdout), &mids); err != nil {
		t.Fatalf("failed to parse mids: %v", err)
	}

	if _, ok := mids["BTC"]; !ok {
		t.Error("expected BTC in mids output")
	}
	if _, ok := mids["ETH"]; !ok {
		t.Error("expected ETH in mids output")
	}
}

func TestE2E_InfoMeta(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "meta")
	requireExitCode(t, exitCode, 0)

	var meta map[string]any
	if err := json.Unmarshal([]byte(stdout), &meta); err != nil {
		t.Fatalf("failed to parse meta: %v", err)
	}

	universe, ok := meta["universe"].([]any)
	if !ok || len(universe) == 0 {
		t.Fatal("expected non-empty universe array")
	}

	first := universe[0].(map[string]any)
	if first["name"] == nil {
		t.Error("expected 'name' field in universe entry")
	}
}

func TestE2E_InfoMetaSpot(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "meta", "--spot")
	requireExitCode(t, exitCode, 0)

	var meta map[string]any
	if err := json.Unmarshal([]byte(stdout), &meta); err != nil {
		t.Fatalf("failed to parse spot meta: %v", err)
	}

	if meta["universe"] == nil {
		t.Error("expected 'universe' in spot meta output")
	}
}

func TestE2E_InfoBook(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "book", "BTC")
	requireExitCode(t, exitCode, 0)

	var book map[string]any
	if err := json.Unmarshal([]byte(stdout), &book); err != nil {
		t.Fatalf("failed to parse book: %v", err)
	}

	levels, ok := book["levels"].([]any)
	if !ok || len(levels) < 2 {
		t.Fatal("expected at least 2 sides (bid/ask) in levels")
	}
}

func TestE2E_InfoTrades(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "trades", "BTC")
	requireExitCode(t, exitCode, 0)

	var trades []any
	if err := json.Unmarshal([]byte(stdout), &trades); err != nil {
		t.Fatalf("failed to parse trades: %v", err)
	}

	if len(trades) == 0 {
		t.Skip("no recent trades for BTC on testnet")
	}

	first := trades[0].(map[string]any)
	for _, field := range []string{"coin", "side", "px", "sz"} {
		if first[field] == nil {
			t.Errorf("expected '%s' field in trade", field)
		}
	}
}

func TestE2E_InfoCandles(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "candles", "BTC", "1h")
	requireExitCode(t, exitCode, 0)

	var candles []any
	if err := json.Unmarshal([]byte(stdout), &candles); err != nil {
		t.Fatalf("failed to parse candles: %v", err)
	}

	if len(candles) == 0 {
		t.Skip("no candles for BTC 1h on testnet")
	}

	first := candles[0].(map[string]any)
	for _, field := range []string{"t", "o", "h", "l", "c", "v"} {
		if first[field] == nil {
			t.Errorf("expected '%s' field in candle", field)
		}
	}
}

func TestE2E_InfoCandlesInvalidInterval(t *testing.T) {
	_, stderr, exitCode := runHlgo(t, "info", "candles", "BTC", "2m")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for invalid interval")
	}

	if !strings.Contains(stderr, "VALIDATION_ERROR") {
		t.Errorf("expected VALIDATION_ERROR in stderr, got: %s", stderr)
	}
}

func TestE2E_InfoState(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "state")
	requireExitCode(t, exitCode, 0)

	result := parseJSONOutput(t, stdout)
	// Should return clearinghouse state (even if no positions).
	if result == nil {
		t.Fatal("expected non-nil state result")
	}
}

func TestE2E_InfoOpenOrders(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "open-orders")
	requireExitCode(t, exitCode, 0)

	// Result is an array (may be empty).
	var orders []any
	if err := json.Unmarshal([]byte(stdout), &orders); err != nil {
		t.Fatalf("failed to parse open orders: %v", err)
	}
}

func TestE2E_InfoFunding(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "funding", "BTC", "--predicted")
	requireExitCode(t, exitCode, 0)

	// Predicted fundings returns an array of arrays.
	var result any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse predicted fundings: %v", err)
	}
}

func TestE2E_InfoPerpDexs(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "perp-dexs")
	requireExitCode(t, exitCode, 0)

	var dexs []any
	if err := json.Unmarshal([]byte(stdout), &dexs); err != nil {
		t.Fatalf("failed to parse perp dexs: %v", err)
	}
}

func TestE2E_InfoRateLimit(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "info", "rate-limit")
	requireExitCode(t, exitCode, 0)

	result := parseJSONOutput(t, stdout)
	if result == nil {
		t.Fatal("expected non-nil rate limit result")
	}
}

type e2eSpotMeta struct {
	Universe []e2eSpotMarket `json:"universe"`
	Tokens   []e2eSpotToken  `json:"tokens"`
}

type e2eSpotMarket struct {
	Name   string `json:"name"`
	Index  int    `json:"index"`
	Tokens []int  `json:"tokens"`
}

type e2eSpotToken struct {
	Name     string  `json:"name"`
	Index    int     `json:"index"`
	FullName *string `json:"fullName"`
}

func loadSpotMeta(t *testing.T, testnet bool) *e2eSpotMeta {
	t.Helper()

	stdout, stderr, exitCode := runHlgoOnNetwork(t, testnet, "info", "meta", "--spot")
	if exitCode != 0 {
		t.Fatalf("spot meta query failed (exit %d): %s", exitCode, stderr)
	}

	var meta e2eSpotMeta
	if err := json.Unmarshal([]byte(stdout), &meta); err != nil {
		t.Fatalf("failed to parse spot meta: %v", err)
	}
	return &meta
}

func unitAlias(name string, fullName *string) string {
	if fullName == nil {
		return strings.ToUpper(name)
	}
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(*fullName)), "UNIT ") {
		return strings.ToUpper(name)
	}

	alias := strings.ToUpper(name)
	for len(alias) > 1 && strings.HasPrefix(alias, "U") {
		alias = alias[1:]
	}
	return alias
}

func findUnitSpotPair(meta *e2eSpotMeta, unitFullName string) (pairAlias string, canonicalBase string, wantAsset int, ok bool) {
	tokenByIndex := make(map[int]e2eSpotToken, len(meta.Tokens))
	usdcIndex := -1
	for _, token := range meta.Tokens {
		tokenByIndex[token.Index] = token
		if strings.EqualFold(token.Name, "USDC") {
			usdcIndex = token.Index
		}
	}

	var baseToken *e2eSpotToken
	for i := range meta.Tokens {
		token := &meta.Tokens[i]
		if token.FullName != nil && *token.FullName == unitFullName {
			if strings.HasPrefix(strings.ToUpper(token.Name), "U") {
				baseToken = token
				break
			}
			if baseToken == nil {
				baseToken = token
			}
		}
	}
	if baseToken == nil {
		return "", "", 0, false
	}

	var market *e2eSpotMarket
	for i := range meta.Universe {
		m := &meta.Universe[i]
		if len(m.Tokens) < 2 || m.Tokens[0] != baseToken.Index {
			continue
		}
		if market == nil {
			market = m
		}
		if usdcIndex >= 0 && m.Tokens[1] == usdcIndex {
			market = m
			break
		}
	}
	if market == nil {
		return "", "", 0, false
	}

	quoteToken, ok := tokenByIndex[market.Tokens[1]]
	if !ok {
		return "", "", 0, false
	}

	baseAlias := unitAlias(baseToken.Name, baseToken.FullName)
	quoteAlias := unitAlias(quoteToken.Name, quoteToken.FullName)
	return baseAlias + "/" + quoteAlias, baseToken.Name, 10000 + market.Index, true
}

func assertSpotResolutionByAlias(t *testing.T, testnet bool, pairAlias, canonicalBase string, wantAsset int) {
	t.Helper()

	stdout, stderr, exitCode := runHlgoOnNetwork(t, testnet,
		"order", "place",
		"--coin", pairAlias,
		"--side", "buy",
		"--price", "1000",
		"--size", "1",
		"--dry-run",
	)
	if exitCode != 0 {
		t.Fatalf("spot resolver dry-run failed for %s (exit %d): %s", pairAlias, exitCode, stderr)
	}

	result := parseJSONOutput(t, stdout)
	resolved, ok := result["resolved"].(map[string]any)
	if !ok {
		t.Fatalf("expected resolved map for %s, got %T", pairAlias, result["resolved"])
	}

	resolvedCoin, ok := resolved["coin"].(string)
	if !ok {
		t.Fatalf("resolved.coin type = %T, want string", resolved["coin"])
	}
	if resolvedCoin != canonicalBase {
		t.Fatalf("resolved.coin = %s, want %s", resolvedCoin, canonicalBase)
	}

	isSpot, ok := resolved["is_spot"].(bool)
	if !ok {
		t.Fatalf("resolved.is_spot type = %T, want bool", resolved["is_spot"])
	}
	if !isSpot {
		t.Fatalf("%s resolved as perp, want spot", pairAlias)
	}

	asset, ok := resolved["asset_id"].(float64)
	if !ok {
		t.Fatalf("resolved.asset_id type = %T, want number", resolved["asset_id"])
	}
	if int(asset) != wantAsset {
		t.Fatalf("resolved.asset_id = %d, want %d", int(asset), wantAsset)
	}
}

func assertPerpResolution(t *testing.T, testnet bool, coin string) {
	t.Helper()
	stdout, stderr, exitCode := runHlgoOnNetwork(t, testnet,
		"order", "place",
		"--coin", coin,
		"--side", "buy",
		"--price", "1000",
		"--size", "1",
		"--dry-run",
	)
	if exitCode != 0 {
		t.Fatalf("resolver dry-run failed for %s (exit %d): %s", coin, exitCode, stderr)
	}

	result := parseJSONOutput(t, stdout)
	resolved, ok := result["resolved"].(map[string]any)
	if !ok {
		t.Fatalf("expected resolved map for %s, got %T", coin, result["resolved"])
	}

	if resolvedCoin, ok := resolved["coin"].(string); !ok || resolvedCoin != coin {
		t.Fatalf("resolved.coin = %v, want %s", resolved["coin"], coin)
	}

	isSpot, ok := resolved["is_spot"].(bool)
	if !ok {
		t.Fatalf("resolved.is_spot type = %T, want bool", resolved["is_spot"])
	}
	if isSpot {
		t.Fatalf("%s resolved as spot, want perp", coin)
	}

	if _, ok := resolved["asset_id"].(float64); !ok {
		t.Fatalf("resolved.asset_id type = %T, want number", resolved["asset_id"])
	}
}

func TestE2E_ResolverPerps_Testnet(t *testing.T) {
	for _, coin := range []string{"ETH", "BTC", "SOL"} {
		t.Run(coin, func(t *testing.T) {
			assertPerpResolution(t, true, coin)
		})
	}
}

func TestE2E_ResolverPerps_Mainnet(t *testing.T) {
	if os.Getenv("HL_E2E_MAINNET") != "1" {
		t.Skip("skipping mainnet resolver checks (set HL_E2E_MAINNET=1 to enable)")
	}
	for _, coin := range []string{"ETH", "BTC", "SOL"} {
		t.Run(coin, func(t *testing.T) {
			assertPerpResolution(t, false, coin)
		})
	}
}

func TestE2E_ResolverSpotUnitAliases_Testnet(t *testing.T) {
	meta := loadSpotMeta(t, true)
	coins := []struct {
		symbol   string
		fullName string
	}{
		{symbol: "ETH", fullName: "Unit Ethereum"},
		{symbol: "BTC", fullName: "Unit Bitcoin"},
		{symbol: "SOL", fullName: "Unit Solana"},
	}

	for _, coin := range coins {
		coin := coin
		t.Run(coin.symbol, func(t *testing.T) {
			pairAlias, canonicalBase, wantAsset, ok := findUnitSpotPair(meta, coin.fullName)
			if !ok {
				t.Skipf("no %s unit spot pair found on testnet", coin.symbol)
			}
			assertSpotResolutionByAlias(t, true, pairAlias, canonicalBase, wantAsset)
		})
	}
}

func TestE2E_ResolverSpotUnitAliases_Mainnet(t *testing.T) {
	if os.Getenv("HL_E2E_MAINNET") != "1" {
		t.Skip("skipping mainnet resolver checks (set HL_E2E_MAINNET=1 to enable)")
	}

	meta := loadSpotMeta(t, false)
	coins := []struct {
		symbol   string
		fullName string
	}{
		{symbol: "ETH", fullName: "Unit Ethereum"},
		{symbol: "BTC", fullName: "Unit Bitcoin"},
		{symbol: "SOL", fullName: "Unit Solana"},
	}

	for _, coin := range coins {
		coin := coin
		t.Run(coin.symbol, func(t *testing.T) {
			pairAlias, canonicalBase, wantAsset, ok := findUnitSpotPair(meta, coin.fullName)
			if !ok {
				t.Skipf("no %s unit spot pair found on mainnet", coin.symbol)
			}
			assertSpotResolutionByAlias(t, false, pairAlias, canonicalBase, wantAsset)
		})
	}
}

// --- Order E2E tests (dry-run only by default) ---

func TestE2E_OrderPlaceDryRun(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "order", "place",
		"--coin", "BTC", "--side", "buy", "--price", "10000", "--size", "0.001",
		"--dry-run",
	)
	requireExitCode(t, exitCode, 0)

	result := parseJSONOutput(t, stdout)
	if result["action"] == nil {
		t.Error("expected 'action' in dry-run output")
	}
	if result["resolved"] == nil {
		t.Error("expected 'resolved' in dry-run output")
	}
}

func TestE2E_OrderMarketDryRun(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "order", "market",
		"--coin", "BTC", "--side", "buy", "--size", "0.001",
		"--dry-run",
	)
	requireExitCode(t, exitCode, 0)

	result := parseJSONOutput(t, stdout)
	if result["action"] == nil {
		t.Error("expected 'action' in market dry-run output")
	}
}

func TestE2E_OrderCancelDryRun(t *testing.T) {
	stdout, _, exitCode := runHlgo(t, "order", "cancel",
		"--coin", "BTC", "--oid", "12345",
		"--dry-run",
	)
	requireExitCode(t, exitCode, 0)

	result := parseJSONOutput(t, stdout)
	if result["type"] != "cancel" {
		t.Errorf("type = %v, want cancel", result["type"])
	}
}

// TestE2E_OrderPlaceAndCancel is the full lifecycle test.
// Only runs when HL_E2E_WRITE=1 to avoid accidental order placement.
func TestE2E_OrderPlaceAndCancel(t *testing.T) {
	if os.Getenv("HL_E2E_WRITE") != "1" {
		t.Skip("skipping write test (set HL_E2E_WRITE=1 to enable)")
	}

	// 1. Place a far-from-market limit order (won't fill).
	stdout, stderr, exitCode := runHlgo(t, "order", "place",
		"--coin", "BTC", "--side", "buy", "--price", "10000", "--size", "0.001",
	)
	if exitCode != 0 {
		t.Fatalf("place failed (exit %d): %s", exitCode, stderr)
	}

	placeResult := parseJSONOutput(t, stdout)
	response, ok := placeResult["response"].(map[string]any)
	if !ok {
		t.Fatalf("expected response map, got: %v", placeResult)
	}

	status, ok := response["status"].(string)
	if !ok || status != "ok" {
		t.Fatalf("expected status ok, got: %v", response["status"])
	}

	// Extract OID from response.response.data.statuses[0].resting.oid
	statusData, ok := response["response"].(map[string]any)
	if !ok {
		t.Fatalf("expected response.response map, got: %v", response)
	}
	dataMap, ok := statusData["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data map, got: %v", statusData)
	}
	statuses, ok := dataMap["statuses"].([]any)
	if !ok || len(statuses) == 0 {
		t.Fatalf("expected statuses array, got: %v", dataMap)
	}

	firstStatus := statuses[0].(map[string]any)
	resting, ok := firstStatus["resting"].(map[string]any)
	if !ok {
		t.Fatalf("expected resting order, got: %v", firstStatus)
	}
	oidFloat, ok := resting["oid"].(float64)
	if !ok {
		t.Fatalf("expected oid number, got: %T %v", resting["oid"], resting["oid"])
	}

	oidStr := fmt.Sprintf("%.0f", oidFloat)
	t.Logf("placed order OID: %s", oidStr)

	// 2. Verify order appears in open-orders.
	stdout, _, exitCode = runHlgo(t, "info", "open-orders")
	requireExitCode(t, exitCode, 0)

	var orders []any
	if err := json.Unmarshal([]byte(stdout), &orders); err != nil {
		t.Fatalf("failed to parse open orders: %v", err)
	}

	found := false
	for _, o := range orders {
		order := o.(map[string]any)
		if oid, ok := order["oid"].(float64); ok && oid == oidFloat {
			found = true
			break
		}
	}
	if !found {
		t.Error("placed order not found in open-orders")
	}

	// 3. Cancel the order.
	cancelStdout, cancelStderr, cancelExit := runHlgo(t, "order", "cancel",
		"--coin", "BTC", "--oid", oidStr,
	)
	if cancelExit != 0 {
		t.Fatalf("cancel failed (exit %d): %s", cancelExit, cancelStderr)
	}
	t.Logf("cancel result: %s", cancelStdout)

	// 4. Verify order no longer in open-orders.
	stdout, _, exitCode = runHlgo(t, "info", "open-orders")
	requireExitCode(t, exitCode, 0)

	if err := json.Unmarshal([]byte(stdout), &orders); err != nil {
		t.Fatalf("failed to parse open orders: %v", err)
	}

	for _, o := range orders {
		order := o.(map[string]any)
		if oid, ok := order["oid"].(float64); ok && oid == oidFloat {
			t.Error("cancelled order still appears in open-orders")
		}
	}
}
