//go:build integration

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

var integrationBinaryPath string

const integrationTestPrivateKey = "0x0123456789012345678901234567890123456789012345678901234567890123"

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "hlgo-integration")
	if err != nil {
		panic("cannot create temp dir: " + err.Error())
	}

	integrationBinaryPath = filepath.Join(dir, "hlgo")
	cmd := exec.Command("go", "build", "-o", integrationBinaryPath, "..")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("cannot build hlgo: " + err.Error())
	}

	if os.Getenv("HL_CONFIG") == "" {
		cfgPath := filepath.Join(dir, "integration-config.yaml")
		cfg := []byte("private_key_env: HL_TEST_PRIVATE_KEY\nmetadata_ttl: 300\n")
		if err := os.WriteFile(cfgPath, cfg, 0600); err != nil {
			panic("cannot write integration config: " + err.Error())
		}
		_ = os.Setenv("HL_CONFIG", cfgPath)
	}

	os.Exit(m.Run())
}

func runIntegrationHLGO(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	fullArgs := append([]string{"--testnet"}, args...)
	cmd := exec.CommandContext(ctx, integrationBinaryPath, fullArgs...)
	cmd.Env = os.Environ()

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("command timed out: hlgo %s", strings.Join(fullArgs, " "))
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run hlgo: %v", err)
		}
	}
	return stdout, stderr, exitCode
}

func runIntegrationHLGOWithRetry(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	const attempts = 3
	for attempt := 1; attempt <= attempts; attempt++ {
		stdout, stderr, exitCode = runIntegrationHLGO(t, args...)
		if exitCode == 0 || !isTransientIntegrationError(exitCode, stderr) || attempt == attempts {
			return stdout, stderr, exitCode
		}
		time.Sleep(500 * time.Millisecond)
	}
	return stdout, stderr, exitCode
}

func isTransientIntegrationError(exitCode int, stderr string) bool {
	if exitCode == 3 || exitCode == 6 {
		return true
	}
	return strings.Contains(stderr, "Unexpected error connecting.")
}

func requireIntegrationExitCode(t *testing.T, got, want int, stderr string) {
	t.Helper()
	if got != want {
		t.Fatalf("exit code = %d, want %d, stderr=%s", got, want, stderr)
	}
}

func parseJSONArray(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("failed to parse JSON array: %v\nraw: %s", err, stdout)
	}
	return out
}

func parseJSONObject(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("failed to parse JSON object: %v\nraw: %s", err, stdout)
	}
	return out
}

func parseErrorObject(t *testing.T, stderr string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(stderr), &out); err != nil {
		t.Fatalf("failed to parse error JSON object: %v\nraw: %s", err, stderr)
	}
	return out
}

func requireFieldString(t *testing.T, obj map[string]any, field, want string) {
	t.Helper()
	got, ok := obj[field].(string)
	if !ok {
		t.Fatalf("field %q is missing or not a string: %#v", field, obj[field])
	}
	if got != want {
		t.Fatalf("field %q = %q, want %q", field, got, want)
	}
}

func requireFieldBool(t *testing.T, obj map[string]any, field string, want bool) {
	t.Helper()
	got, ok := obj[field].(bool)
	if !ok {
		t.Fatalf("field %q is missing or not a bool: %#v", field, obj[field])
	}
	if got != want {
		t.Fatalf("field %q = %v, want %v", field, got, want)
	}
}

func requireFieldPositiveInt64(t *testing.T, obj map[string]any, field string) int64 {
	t.Helper()
	raw, ok := obj[field]
	if !ok {
		t.Fatalf("field %q is missing", field)
	}

	floatVal, ok := raw.(float64)
	if !ok {
		t.Fatalf("field %q is not numeric: %#v", field, raw)
	}

	v := int64(floatVal)
	if v <= 0 {
		t.Fatalf("field %q = %d, want > 0", field, v)
	}
	return v
}

func requireErrorCode(t *testing.T, stderr, want string) map[string]any {
	t.Helper()
	errObj := parseErrorObject(t, stderr)
	requireFieldString(t, errObj, "code", want)
	return errObj
}

func ensurePrivateKeyForAccountDryRun(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("HL_TEST_PRIVATE_KEY")) == "" {
		t.Setenv("HL_TEST_PRIVATE_KEY", integrationTestPrivateKey)
	}
}

func setIntegrationPrivateKey(t *testing.T, privateKey string) {
	t.Helper()
	t.Setenv("HL_TEST_PRIVATE_KEY", strings.TrimSpace(privateKey))
}

func integrationAddressFromPrivateKey(t *testing.T, privateKeyHex string) string {
	t.Helper()

	key, err := ethcrypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x"))
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	return strings.ToLower(ethcrypto.PubkeyToAddress(key.PublicKey).Hex())
}

func newIntegrationSignerKey(t *testing.T) (privateKeyHex, address string) {
	t.Helper()

	key, err := ethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate signer key: %v", err)
	}

	return "0x" + hex.EncodeToString(ethcrypto.FromECDSA(key)), strings.ToLower(ethcrypto.PubkeyToAddress(key.PublicKey).Hex())
}

func restingBtcBidPrice(t *testing.T) string {
	t.Helper()

	stdout, stderr, code := runIntegrationHLGOWithRetry(t, "info", "mids")
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)

	mids := parseJSONObject(t, stdout)
	rawMid, ok := mids["BTC"].(string)
	if !ok || strings.TrimSpace(rawMid) == "" {
		t.Fatalf("expected BTC mid string, got %#v", mids["BTC"])
	}

	mid, err := strconv.ParseFloat(rawMid, 64)
	if err != nil {
		t.Fatalf("failed to parse BTC mid %q: %v", rawMid, err)
	}
	if mid <= 0 {
		t.Fatalf("BTC mid must be positive, got %f", mid)
	}

	// Keep a resting bid comfortably below market to reduce fill probability.
	price := int(mid * 0.8)
	if price < 1 {
		price = 1
	}
	return strconv.Itoa(price)
}

func assertNoSecretLeak(t *testing.T, stdout, stderr string) {
	t.Helper()
	candidates := []string{
		integrationTestPrivateKey,
		strings.TrimSpace(os.Getenv("HL_TEST_PRIVATE_KEY")),
	}

	for _, secret := range candidates {
		if secret == "" {
			continue
		}
		if strings.Contains(stdout, secret) || strings.Contains(stderr, secret) {
			t.Fatalf("detected secret leak in CLI output")
		}
	}
}

func TestIntegration_InfoLookupAllDexesAllowsInheritedDefaultDex(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "integration-config.yaml")
	cfg := []byte("private_key_env: HL_TEST_PRIVATE_KEY\ndefault_dex: tngs\nmetadata_ttl: 300\n")
	if err := os.WriteFile(cfgPath, cfg, 0600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	stdout, stderr, code := runIntegrationHLGO(t,
		"--config", cfgPath,
		"info", "lookup", "BTC",
		"--dry-run",
		"--all-dexes",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)

	obj := parseJSONObject(t, stdout)
	scope, ok := obj["scope"].(map[string]any)
	if !ok {
		t.Fatalf("expected scope object, got %#v", obj["scope"])
	}
	requireFieldBool(t, scope, "all_dexes", true)
	requireFieldString(t, scope, "dex", "tngs")

	requests, ok := obj["requests"].(map[string]any)
	if !ok {
		t.Fatalf("expected requests object, got %#v", obj["requests"])
	}
	if _, ok := requests["perp_dexs"]; !ok {
		t.Fatal("expected perp_dexs request")
	}

	hip3Reqs, ok := requests["hip3_meta"].([]any)
	if !ok || len(hip3Reqs) != 1 {
		t.Fatalf("hip3_meta = %T(len=%d), want array len 1", requests["hip3_meta"], len(hip3Reqs))
	}

	hip3Req, ok := hip3Reqs[0].(map[string]any)
	if !ok {
		t.Fatalf("hip3_meta[0] = %T, want object", hip3Reqs[0])
	}
	requireFieldString(t, hip3Req, "type", "meta")
	requireFieldString(t, hip3Req, "dex", "<each dex from perpDexs>")
}

func requiredLiveEnv(t *testing.T, names ...string) map[string]string {
	t.Helper()

	vals := make(map[string]string, len(names))
	var missing []string
	for _, name := range names {
		val := strings.TrimSpace(os.Getenv(name))
		if val == "" {
			missing = append(missing, name)
			continue
		}
		vals[name] = val
	}

	if len(missing) > 0 {
		t.Fatalf("live account integration requires env vars: %s", strings.Join(missing, ", "))
	}

	return vals
}

func newIntegrationCloid(t *testing.T) string {
	t.Helper()

	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("failed to generate cloid: %v", err)
	}
	return "0x" + hex.EncodeToString(b[:])
}

func newIntegrationAddress(t *testing.T) string {
	t.Helper()

	var b [20]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("failed to generate address: %v", err)
	}
	return "0x" + hex.EncodeToString(b[:])
}

func findOpenOrderByCloid(orders []map[string]any, cloid string) (map[string]any, bool) {
	for _, ord := range orders {
		if got, ok := ord["cloid"].(string); ok && got == cloid {
			return ord, true
		}
	}
	return nil, false
}

func findOpenOrderByOID(orders []map[string]any, oid int64) (map[string]any, bool) {
	for _, ord := range orders {
		got, ok := ord["oid"].(float64)
		if !ok {
			continue
		}
		if int64(got) == oid {
			return ord, true
		}
	}
	return nil, false
}

func findOpenOrderByLimitPx(orders []map[string]any, px string) (map[string]any, bool) {
	for _, ord := range orders {
		gotPx, ok := ord["limitPx"].(string)
		if !ok {
			continue
		}
		if gotPx == px || gotPx == px+".0" {
			return ord, true
		}
	}
	return nil, false
}

func TestIntegration_OrderLifecycle(t *testing.T) {
	if os.Getenv("HL_TEST_PRIVATE_KEY") == "" {
		t.Skip("skipping order lifecycle integration: HL_TEST_PRIVATE_KEY is not set")
	}

	cloid := newIntegrationCloid(t)
	basePrice := 23000 + int(time.Now().UnixNano()%1000)
	price1 := strconv.Itoa(basePrice)
	price2 := strconv.Itoa(basePrice + 1)

	cleanup := func() {
		_, _, _ = runIntegrationHLGO(t,
			"order", "cancel",
			"--coin", "BTC",
			"--cloid", cloid,
		)
	}
	defer cleanup()

	t.Log("step 1: place a resting order")
	stdout, stderr, code := runIntegrationHLGO(t,
		"order", "place",
		"--coin", "BTC",
		"--side", "buy",
		"--price", price1,
		"--size", "0.001",
		"--cloid", cloid,
	)
	assertNoSecretLeak(t, stdout, stderr)
	if code != 0 && strings.Contains(stderr, "does not exist") {
		t.Skipf("skipping order lifecycle integration: trading wallet is not registered: %s", strings.TrimSpace(stderr))
	}
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	t.Log("step 2: verify order appears in open-orders")
	stdout, stderr, code = runIntegrationHLGO(t, "info", "open-orders")
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	orders := parseJSONArray(t, stdout)
	openOrder, found := findOpenOrderByCloid(orders, cloid)
	if !found {
		t.Fatalf("expected order with cloid %q in open-orders", cloid)
	}

	oidFloat, ok := openOrder["oid"].(float64)
	if !ok {
		t.Fatalf("open order oid missing or not numeric: %+v", openOrder["oid"])
	}
	oid := int64(oidFloat)
	if oid <= 0 {
		t.Fatalf("unexpected oid value: %d", oid)
	}

	t.Log("step 3: verify order-status query for open order")
	stdout, stderr, code = runIntegrationHLGO(t, "info", "order-status", cloid)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	t.Log("step 4: modify the order price")
	stdout, stderr, code = runIntegrationHLGO(t,
		"order", "modify",
		"--coin", "BTC",
		"--oid", fmt.Sprintf("%d", oid),
		"--side", "buy",
		"--price", price2,
		"--size", "0.001",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	modifyResp := parseJSONObject(t, stdout)
	if response, ok := modifyResp["response"].(map[string]any); ok {
		if innerResponse, ok := response["response"].(map[string]any); ok {
			if data, ok := innerResponse["data"].(map[string]any); ok {
				if statuses, ok := data["statuses"].([]any); ok && len(statuses) > 0 {
					if first, ok := statuses[0].(map[string]any); ok {
						if resting, ok := first["resting"].(map[string]any); ok {
							if nextOID, ok := resting["oid"].(float64); ok {
								oid = int64(nextOID)
							}
						}
					}
				}
			}
		}
	}

	t.Log("step 5: locate modified order in open-orders")
	var modifiedOrder map[string]any
	var modifiedOID int64
	found = false
	for range 6 {
		stdout, stderr, code = runIntegrationHLGO(t, "info", "open-orders")
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		orders = parseJSONArray(t, stdout)
		if cand, ok := findOpenOrderByLimitPx(orders, price2); ok {
			modifiedOrder = cand
			found = true
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !found {
		t.Fatalf("expected modified order with limitPx %q in open-orders", price2)
	}
	modifiedOIDFloat, ok := modifiedOrder["oid"].(float64)
	if !ok {
		t.Fatalf("modified order oid missing or not numeric: %+v", modifiedOrder["oid"])
	}
	modifiedOID = int64(modifiedOIDFloat)
	if modifiedOID <= 0 {
		t.Fatalf("unexpected modified oid value: %d", modifiedOID)
	}

	t.Log("step 6: cancel modified order by oid")
	stdout, stderr, code = runIntegrationHLGO(t,
		"order", "cancel",
		"--coin", "BTC",
		"--oid", fmt.Sprintf("%d", modifiedOID),
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	t.Log("step 7: verify order is gone from open-orders")
	stdout, stderr, code = runIntegrationHLGO(t, "info", "open-orders")
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	orders = parseJSONArray(t, stdout)
	if _, found := findOpenOrderByOID(orders, modifiedOID); found {
		t.Fatalf("order with oid %d should be canceled but is still open", modifiedOID)
	}

	t.Log("step 8: verify order-status query after cancel")
	stdout, stderr, code = runIntegrationHLGO(t, "info", "order-status", cloid)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)
}

func TestIntegration_OnBehalf_AuthorizedRandomSignerFlow(t *testing.T) {
	deployerKey := strings.TrimSpace(os.Getenv("HL_TEST_PRIVATE_KEY"))
	if deployerKey == "" {
		t.Skip("skipping on-behalf integration: HL_TEST_PRIVATE_KEY is not set")
	}

	deployerAddr := integrationAddressFromPrivateKey(t, deployerKey)
	agentKey, agentAddr := newIntegrationSignerKey(t)
	agentName := fmt.Sprintf("agt%013d", time.Now().UnixNano()%10_000_000_000_000)
	cloid := newIntegrationCloid(t)
	price := restingBtcBidPrice(t)

	agentApproved := false
	orderPlaced := false
	defer func() {
		// Cleanup should always run as the deployer account.
		setIntegrationPrivateKey(t, deployerKey)
		if orderPlaced {
			_, _, _ = runIntegrationHLGO(t,
				"order", "cancel",
				"--coin", "BTC",
				"--cloid", cloid,
			)
		}
		if agentApproved {
			_, _, _ = runIntegrationHLGO(t,
				"account", "approve-agent",
				"--agent", agentAddr,
				"--revoke",
				"--confirm",
			)
		}
	}()

	t.Log("step 1: authorize random signer as agent from deployer account")
	setIntegrationPrivateKey(t, deployerKey)
	stdout, stderr, code := runIntegrationHLGOWithRetry(t,
		"account", "approve-agent",
		"--agent", agentAddr,
		"--name", agentName,
	)
	assertNoSecretLeak(t, stdout, stderr)
	if code != 0 && strings.Contains(stderr, "does not exist") {
		t.Skipf("skipping on-behalf integration: deployer account unavailable: %s", strings.TrimSpace(stderr))
	}
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)
	agentApproved = true

	t.Log("step 2: place order with random signer on behalf of deployer account")
	setIntegrationPrivateKey(t, agentKey)
	for attempt := 1; attempt <= 6; attempt++ {
		stdout, stderr, code = runIntegrationHLGO(t,
			"order", "place",
			"--coin", "BTC",
			"--side", "buy",
			"--price", price,
			"--size", "0.001",
			"--cloid", cloid,
		)
		assertNoSecretLeak(t, stdout, stderr)
		if code == 0 {
			break
		}

		// Approvals may take a moment to become usable in live environments.
		if strings.Contains(stderr, "does not exist") || strings.Contains(strings.ToLower(stderr), "not approved") {
			if attempt < 6 {
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}

		if isTransientIntegrationError(code, stderr) && attempt < 6 {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	if code != 0 && strings.Contains(stderr, "does not exist") {
		t.Skipf("skipping on-behalf integration: account/approval not ready: %s", strings.TrimSpace(stderr))
	}
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)
	orderPlaced = true

	t.Log("step 3: verify on-behalf order appears in deployer open-orders")
	found := false
	for range 8 {
		stdout, stderr, code = runIntegrationHLGOWithRetry(t,
			"info", "open-orders",
			"--address", deployerAddr,
		)
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		orders := parseJSONArray(t, stdout)
		if _, ok := findOpenOrderByCloid(orders, cloid); ok {
			found = true
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !found {
		t.Fatalf("expected on-behalf order with cloid %q in deployer open-orders", cloid)
	}

	t.Log("step 4: verify unsupported on-behalf account action is rejected")
	stdout, stderr, code = runIntegrationHLGO(t,
		"account", "transfer",
		"--amount", "1",
		"--to-perp",
		"--on-behalf-of", deployerAddr,
		"--dry-run",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 4, stderr)
	errObj := requireErrorCode(t, stderr, "API_ERROR")
	msg, ok := errObj["error"].(string)
	if !ok {
		t.Fatalf("missing error message: %#v", errObj["error"])
	}
	if !strings.Contains(msg, "unknown flag") {
		t.Fatalf("unexpected error message: %q", msg)
	}

	t.Log("step 5: verify schedule-cancel rejects on-behalf context")
	stdout, stderr, code = runIntegrationHLGO(t,
		"order", "schedule-cancel",
		"--timeout", "5m",
		"--on-behalf-of", deployerAddr,
		"--dry-run",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 4, stderr)
	errObj = requireErrorCode(t, stderr, "API_ERROR")
	msg, ok = errObj["error"].(string)
	if !ok {
		t.Fatalf("missing error message: %#v", errObj["error"])
	}
	if !strings.Contains(msg, "unknown flag") {
		t.Fatalf("unexpected error message: %q", msg)
	}
}

func TestIntegration_AccountDryRunPayloadContracts(t *testing.T) {
	ensurePrivateKeyForAccountDryRun(t)
	const mixedAddr = "0xABABABABABABABABABABABABABABABABABABABAB"
	const lowerAddr = "0xabababababababababababababababababababab"

	t.Run("transfer dry-run payload", func(t *testing.T) {
		stdout, stderr, code := runIntegrationHLGO(t,
			"account", "transfer",
			"--amount", "1.25",
			"--to-perp",
			"--dry-run",
		)
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		obj := parseJSONObject(t, stdout)

		requireFieldString(t, obj, "type", "usdClassTransfer")
		requireFieldString(t, obj, "amount", "1.25")
		requireFieldBool(t, obj, "toPerp", true)
		requireFieldString(t, obj, "signatureChainId", "0x66eee")
		requireFieldString(t, obj, "hyperliquidChain", "Testnet")
		requireFieldPositiveInt64(t, obj, "nonce")
	})

	t.Run("class-transfer dry-run payload", func(t *testing.T) {
		stdout, stderr, code := runIntegrationHLGO(t,
			"account", "class-transfer",
			"--amount", "7.5",
			"--to-spot",
			"--dry-run",
		)
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		obj := parseJSONObject(t, stdout)

		requireFieldString(t, obj, "type", "usdClassTransfer")
		requireFieldString(t, obj, "amount", "7.5")
		requireFieldBool(t, obj, "toPerp", false)
		requireFieldString(t, obj, "signatureChainId", "0x66eee")
		requireFieldString(t, obj, "hyperliquidChain", "Testnet")
		requireFieldPositiveInt64(t, obj, "nonce")
	})

	t.Run("withdraw dry-run payload", func(t *testing.T) {
		stdout, stderr, code := runIntegrationHLGO(t,
			"account", "withdraw",
			"--destination", mixedAddr,
			"--amount", "2",
			"--dry-run",
		)
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		obj := parseJSONObject(t, stdout)

		requireFieldString(t, obj, "type", "withdraw3")
		requireFieldString(t, obj, "amount", "2")
		requireFieldString(t, obj, "destination", lowerAddr)
		requireFieldString(t, obj, "signatureChainId", "0x66eee")
		requireFieldString(t, obj, "hyperliquidChain", "Testnet")
		requireFieldPositiveInt64(t, obj, "time")
	})

	t.Run("send-asset dry-run payload", func(t *testing.T) {
		stdout, stderr, code := runIntegrationHLGO(t,
			"account", "send-asset",
			"--destination", mixedAddr,
			"--token", "PURR:0x1",
			"--amount", "3.5",
			"--dry-run",
		)
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		obj := parseJSONObject(t, stdout)

		requireFieldString(t, obj, "type", "spotSend")
		requireFieldString(t, obj, "token", "PURR:0x1")
		requireFieldString(t, obj, "amount", "3.5")
		requireFieldString(t, obj, "destination", lowerAddr)
		requireFieldString(t, obj, "signatureChainId", "0x66eee")
		requireFieldString(t, obj, "hyperliquidChain", "Testnet")
		requireFieldPositiveInt64(t, obj, "time")
	})

	t.Run("approve-agent dry-run payload", func(t *testing.T) {
		stdout, stderr, code := runIntegrationHLGO(t,
			"account", "approve-agent",
			"--agent", mixedAddr,
			"--name", "plan-agent",
			"--dry-run",
		)
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		obj := parseJSONObject(t, stdout)

		requireFieldString(t, obj, "type", "approveAgent")
		requireFieldString(t, obj, "agentAddress", lowerAddr)
		requireFieldString(t, obj, "agentName", "plan-agent")
		requireFieldString(t, obj, "signatureChainId", "0x66eee")
		requireFieldString(t, obj, "hyperliquidChain", "Testnet")
		requireFieldPositiveInt64(t, obj, "nonce")
	})

	t.Run("approve-agent revoke dry-run clears name", func(t *testing.T) {
		stdout, stderr, code := runIntegrationHLGO(t,
			"account", "approve-agent",
			"--agent", mixedAddr,
			"--name", "should-be-cleared",
			"--revoke",
			"--confirm",
			"--dry-run",
		)
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		obj := parseJSONObject(t, stdout)

		requireFieldString(t, obj, "type", "approveAgent")
		requireFieldString(t, obj, "agentAddress", lowerAddr)
		if _, ok := obj["agentName"]; ok {
			t.Fatalf("agentName should be omitted for revoke payload, got %#v", obj["agentName"])
		}
		requireFieldPositiveInt64(t, obj, "nonce")
	})

	t.Run("set-abstraction dry-run payload", func(t *testing.T) {
		stdout, stderr, code := runIntegrationHLGO(t,
			"account", "set-abstraction",
			"--user", mixedAddr,
			"--abstraction", "disabled",
			"--dry-run",
		)
		assertNoSecretLeak(t, stdout, stderr)
		requireIntegrationExitCode(t, code, 0, stderr)
		obj := parseJSONObject(t, stdout)

		requireFieldString(t, obj, "type", "userSetAbstraction")
		requireFieldString(t, obj, "user", lowerAddr)
		requireFieldString(t, obj, "abstraction", "disabled")
		requireFieldString(t, obj, "signatureChainId", "0x66eee")
		requireFieldString(t, obj, "hyperliquidChain", "Testnet")
		requireFieldPositiveInt64(t, obj, "nonce")
	})
}

func TestIntegration_AccountValidationContracts(t *testing.T) {
	ensurePrivateKeyForAccountDryRun(t)
	const validAddr = "0x1111111111111111111111111111111111111111"

	tests := []struct {
		name            string
		args            []string
		wantCode        int
		wantErrorCode   string
		wantMessagePart string
	}{
		{
			name:            "transfer requires one direction flag",
			args:            []string{"account", "transfer", "--amount", "1", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "exactly one of --to-perp or --to-spot is required",
		},
		{
			name:            "transfer rejects both direction flags",
			args:            []string{"account", "transfer", "--amount", "1", "--to-perp", "--to-spot", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "exactly one of --to-perp or --to-spot is required",
		},
		{
			name:            "transfer rejects non-numeric amount",
			args:            []string{"account", "transfer", "--amount", "abc", "--to-perp", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "invalid amount",
		},
		{
			name:            "transfer rejects zero amount",
			args:            []string{"account", "transfer", "--amount", "0", "--to-perp", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "amount must be positive",
		},
		{
			name:            "class-transfer rejects negative amount",
			args:            []string{"account", "class-transfer", "--amount", "-1", "--to-spot", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "amount must be positive",
		},
		{
			name:            "withdraw requires confirm or dry-run",
			args:            []string{"account", "withdraw", "--destination", validAddr, "--amount", "1"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "requires --confirm",
		},
		{
			name:            "withdraw rejects invalid destination",
			args:            []string{"account", "withdraw", "--destination", "not-an-address", "--amount", "1", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "invalid destination address",
		},
		{
			name:            "send-asset requires confirm or dry-run",
			args:            []string{"account", "send-asset", "--destination", validAddr, "--token", "PURR:0x1", "--amount", "1"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "requires --confirm",
		},
		{
			name:            "send-asset requires token value",
			args:            []string{"account", "send-asset", "--destination", validAddr, "--token", " ", "--amount", "1", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "token is required",
		},
		{
			name:            "approve-agent revoke requires confirm",
			args:            []string{"account", "approve-agent", "--agent", validAddr, "--revoke"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "requires --confirm",
		},
		{
			name:            "approve-agent requires name unless revoke",
			args:            []string{"account", "approve-agent", "--agent", validAddr, "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "agent name is required unless --revoke is set",
		},
		{
			name:            "approve-agent rejects invalid address",
			args:            []string{"account", "approve-agent", "--agent", "bad", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "invalid agent address",
		},
		{
			name:            "set-abstraction rejects invalid user address",
			args:            []string{"account", "set-abstraction", "--user", "bad", "--abstraction", "disabled", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "invalid user address",
		},
		{
			name:            "set-abstraction rejects unsupported abstraction value",
			args:            []string{"account", "set-abstraction", "--user", validAddr, "--abstraction", "none", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "unsupported abstraction value",
		},
		{
			name:            "set-abstraction rejects empty abstraction",
			args:            []string{"account", "set-abstraction", "--user", validAddr, "--abstraction", "   ", "--dry-run"},
			wantCode:        1,
			wantErrorCode:   "VALIDATION_ERROR",
			wantMessagePart: "abstraction is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runIntegrationHLGO(t, tc.args...)
			assertNoSecretLeak(t, stdout, stderr)
			requireIntegrationExitCode(t, code, tc.wantCode, stderr)
			errObj := requireErrorCode(t, stderr, tc.wantErrorCode)

			msg, ok := errObj["error"].(string)
			if !ok {
				t.Fatalf("error field is missing or not a string: %#v", errObj["error"])
			}
			if !strings.Contains(msg, tc.wantMessagePart) {
				t.Fatalf("error message %q does not contain %q", msg, tc.wantMessagePart)
			}
		})
	}
}

func TestIntegration_AccountMissingPrivateKeyConfigError(t *testing.T) {
	t.Setenv("HL_TEST_PRIVATE_KEY_NOT_SET", "")

	cfgPath := filepath.Join(t.TempDir(), "integration-config.yaml")
	cfg := []byte("private_key_env: HL_TEST_PRIVATE_KEY_NOT_SET\nmetadata_ttl: 300\n")
	if err := os.WriteFile(cfgPath, cfg, 0600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}

	stdout, stderr, code := runIntegrationHLGO(t,
		"--config", cfgPath,
		"account", "transfer",
		"--amount", "1",
		"--to-perp",
		"--dry-run",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 2, stderr)
	errObj := requireErrorCode(t, stderr, "CONFIG_ERROR")
	requireFieldString(t, errObj, "error", "private key not set")

	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %#v", errObj["details"])
	}
	requireFieldString(t, details, "env_var", "HL_TEST_PRIVATE_KEY_NOT_SET")
}

func TestIntegration_AccountDryRunNonceFreshness(t *testing.T) {
	ensurePrivateKeyForAccountDryRun(t)

	stdout1, stderr1, code1 := runIntegrationHLGO(t,
		"account", "transfer",
		"--amount", "1.01",
		"--to-perp",
		"--dry-run",
	)
	assertNoSecretLeak(t, stdout1, stderr1)
	requireIntegrationExitCode(t, code1, 0, stderr1)
	obj1 := parseJSONObject(t, stdout1)
	n1 := requireFieldPositiveInt64(t, obj1, "nonce")

	time.Sleep(3 * time.Millisecond)

	stdout2, stderr2, code2 := runIntegrationHLGO(t,
		"account", "transfer",
		"--amount", "1.01",
		"--to-perp",
		"--dry-run",
	)
	assertNoSecretLeak(t, stdout2, stderr2)
	requireIntegrationExitCode(t, code2, 0, stderr2)
	obj2 := parseJSONObject(t, stdout2)
	n2 := requireFieldPositiveInt64(t, obj2, "nonce")

	if n2 <= n1 {
		t.Fatalf("second nonce = %d, want > first nonce %d", n2, n1)
	}
}

func TestIntegration_AccountLiveReversibleFlows(t *testing.T) {
	if strings.TrimSpace(os.Getenv("HL_TEST_PRIVATE_KEY")) == "" {
		t.Skip("skipping live account reversible flows: HL_TEST_PRIVATE_KEY is not set")
	}

	req := requiredLiveEnv(t, "HL_TEST_ACCOUNT_TRANSFER_AMOUNT")
	agentAddr := strings.TrimSpace(os.Getenv("HL_TEST_APPROVE_AGENT_ADDRESS"))
	if agentAddr == "" {
		agentAddr = newIntegrationAddress(t)
	}
	amount := req["HL_TEST_ACCOUNT_TRANSFER_AMOUNT"]
	agentName := fmt.Sprintf("hlgo%010d", time.Now().Unix()%10_000_000_000)
	if len(agentName) > 16 {
		t.Fatalf("generated agentName exceeds 16 chars: %q", agentName)
	}

	t.Log("step 1: approve then revoke agent")
	revokeNeeded := false
	defer func() {
		if !revokeNeeded {
			return
		}
		_, _, _ = runIntegrationHLGO(t,
			"account", "approve-agent",
			"--agent", agentAddr,
			"--revoke",
			"--confirm",
		)
	}()

	stdout, stderr, code := runIntegrationHLGOWithRetry(t,
		"account", "approve-agent",
		"--agent", agentAddr,
		"--name", agentName,
	)
	if code != 0 && strings.Contains(stderr, "Cannot use existing user address as agent") {
		agentAddr = newIntegrationAddress(t)
		stdout, stderr, code = runIntegrationHLGOWithRetry(t,
			"account", "approve-agent",
			"--agent", agentAddr,
			"--name", agentName,
		)
	}
	assertNoSecretLeak(t, stdout, stderr)
	if code != 0 {
		if strings.Contains(stderr, "Unexpected error connecting.") {
			t.Logf("approve-agent transient failure, continuing with transfer checks: %s", strings.TrimSpace(stderr))
		} else {
			requireIntegrationExitCode(t, code, 0, stderr)
		}
	} else {
		_ = parseJSONObject(t, stdout)
		revokeNeeded = true

		stdout, stderr, code = runIntegrationHLGOWithRetry(t,
			"account", "approve-agent",
			"--agent", agentAddr,
			"--revoke",
			"--confirm",
		)
		assertNoSecretLeak(t, stdout, stderr)
		if code != 0 && strings.Contains(stderr, "Unexpected error connecting.") {
			t.Logf("approve-agent revoke transient failure, deferred cleanup will retry: %s", strings.TrimSpace(stderr))
		} else {
			requireIntegrationExitCode(t, code, 0, stderr)
			_ = parseJSONObject(t, stdout)
			revokeNeeded = false
		}
	}

	t.Log("step 2: reversible transfer flow")
	transferReverted := false
	defer func() {
		if transferReverted {
			return
		}
		_, _, _ = runIntegrationHLGO(t,
			"account", "transfer",
			"--amount", amount,
			"--to-spot",
		)
	}()

	stdout, stderr, code = runIntegrationHLGOWithRetry(t,
		"account", "transfer",
		"--amount", amount,
		"--to-perp",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	stdout, stderr, code = runIntegrationHLGOWithRetry(t,
		"account", "transfer",
		"--amount", amount,
		"--to-spot",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)
	transferReverted = true

	t.Log("step 3: reversible class-transfer flow")
	classTransferReverted := false
	defer func() {
		if classTransferReverted {
			return
		}
		_, _, _ = runIntegrationHLGO(t,
			"account", "class-transfer",
			"--amount", amount,
			"--to-spot",
		)
	}()

	stdout, stderr, code = runIntegrationHLGOWithRetry(t,
		"account", "class-transfer",
		"--amount", amount,
		"--to-perp",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	stdout, stderr, code = runIntegrationHLGOWithRetry(t,
		"account", "class-transfer",
		"--amount", amount,
		"--to-spot",
	)
	assertNoSecretLeak(t, stdout, stderr)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)
	classTransferReverted = true
}

func TestIntegration_AccountLiveOneWayOperations(t *testing.T) {
	if strings.TrimSpace(os.Getenv("HL_TEST_PRIVATE_KEY")) == "" {
		t.Skip("skipping live account one-way operations: HL_TEST_PRIVATE_KEY is not set")
	}

	req := requiredLiveEnv(
		t,
		"HL_TEST_WITHDRAW_DESTINATION",
		"HL_TEST_WITHDRAW_AMOUNT",
		"HL_TEST_SEND_ASSET_DESTINATION",
		"HL_TEST_SEND_ASSET_TOKEN",
		"HL_TEST_SEND_ASSET_AMOUNT",
		"HL_TEST_SET_ABSTRACTION_USER",
		"HL_TEST_SET_ABSTRACTION_VALUE",
	)

	t.Log("step 1: live withdraw")
	stdout, stderr, code := runIntegrationHLGOWithRetry(t,
		"account", "withdraw",
		"--destination", req["HL_TEST_WITHDRAW_DESTINATION"],
		"--amount", req["HL_TEST_WITHDRAW_AMOUNT"],
		"--confirm",
	)
	assertNoSecretLeak(t, stdout, stderr)
	if code == 0 {
		_ = parseJSONObject(t, stdout)
	} else {
		errObj := parseErrorObject(t, stderr)
		gotCode, _ := errObj["code"].(string)
		if !slices.Contains([]string{"API_ERROR", "RATE_LIMIT", "NETWORK_ERROR", "VALIDATION_ERROR"}, gotCode) {
			t.Fatalf("unexpected error code for withdraw: %q stderr=%s", gotCode, stderr)
		}
	}

	t.Log("step 2: live send-asset")
	stdout, stderr, code = runIntegrationHLGOWithRetry(t,
		"account", "send-asset",
		"--destination", req["HL_TEST_SEND_ASSET_DESTINATION"],
		"--token", req["HL_TEST_SEND_ASSET_TOKEN"],
		"--amount", req["HL_TEST_SEND_ASSET_AMOUNT"],
		"--confirm",
	)
	assertNoSecretLeak(t, stdout, stderr)
	if code == 0 {
		_ = parseJSONObject(t, stdout)
	} else {
		errObj := parseErrorObject(t, stderr)
		gotCode, _ := errObj["code"].(string)
		if !slices.Contains([]string{"API_ERROR", "RATE_LIMIT", "NETWORK_ERROR", "VALIDATION_ERROR"}, gotCode) {
			t.Fatalf("unexpected error code for send-asset: %q stderr=%s", gotCode, stderr)
		}
	}

	t.Log("step 3: live set-abstraction")
	stdout, stderr, code = runIntegrationHLGOWithRetry(t,
		"account", "set-abstraction",
		"--user", req["HL_TEST_SET_ABSTRACTION_USER"],
		"--abstraction", req["HL_TEST_SET_ABSTRACTION_VALUE"],
	)
	assertNoSecretLeak(t, stdout, stderr)
	if code == 0 {
		_ = parseJSONObject(t, stdout)
	} else {
		errObj := parseErrorObject(t, stderr)
		gotCode, _ := errObj["code"].(string)
		if !slices.Contains([]string{"API_ERROR", "RATE_LIMIT", "NETWORK_ERROR", "VALIDATION_ERROR"}, gotCode) {
			t.Fatalf("unexpected error code for set-abstraction: %q stderr=%s", gotCode, stderr)
		}
	}
}
