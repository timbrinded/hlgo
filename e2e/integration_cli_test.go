//go:build integration

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var integrationBinaryPath string

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
		cfg := []byte("agent_key_env: HL_TEST_AGENT_KEY\nmaster_key_env: HL_TEST_MASTER_KEY\nmetadata_ttl: 300\n")
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

func findOpenOrderByCloid(orders []map[string]any, cloid string) (map[string]any, bool) {
	for _, ord := range orders {
		if got, ok := ord["cloid"].(string); ok && got == cloid {
			return ord, true
		}
	}
	return nil, false
}

func TestIntegration_OrderLifecycle(t *testing.T) {
	if os.Getenv("HL_TEST_AGENT_KEY") == "" {
		t.Skip("skipping order lifecycle integration: HL_TEST_AGENT_KEY is not set")
	}

	cloid := fmt.Sprintf("hlgo-int-%d", time.Now().UnixNano())
	price1 := "1"
	price2 := "2"

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
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	t.Log("step 2: verify order appears in open-orders")
	stdout, stderr, code = runIntegrationHLGO(t, "info", "open-orders")
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
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	t.Log("step 5: verify modified price in open-orders")
	stdout, stderr, code = runIntegrationHLGO(t, "info", "open-orders")
	requireIntegrationExitCode(t, code, 0, stderr)
	orders = parseJSONArray(t, stdout)
	openOrder, found = findOpenOrderByCloid(orders, cloid)
	if !found {
		t.Fatalf("expected modified order with cloid %q in open-orders", cloid)
	}
	gotPx, _ := openOrder["limitPx"].(string)
	if gotPx != price2 {
		t.Fatalf("limitPx = %q, want %q", gotPx, price2)
	}

	t.Log("step 6: cancel order by cloid")
	stdout, stderr, code = runIntegrationHLGO(t,
		"order", "cancel",
		"--coin", "BTC",
		"--cloid", cloid,
	)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	t.Log("step 7: verify order is gone from open-orders")
	stdout, stderr, code = runIntegrationHLGO(t, "info", "open-orders")
	requireIntegrationExitCode(t, code, 0, stderr)
	orders = parseJSONArray(t, stdout)
	if _, found := findOpenOrderByCloid(orders, cloid); found {
		t.Fatalf("order with cloid %q should be canceled but is still open", cloid)
	}

	t.Log("step 8: verify order-status query after cancel")
	stdout, stderr, code = runIntegrationHLGO(t, "info", "order-status", cloid)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)
}

func TestIntegration_AccountUserSigned(t *testing.T) {
	if os.Getenv("HL_TEST_MASTER_KEY") == "" {
		t.Skip("skipping account integration: HL_TEST_MASTER_KEY is not set")
	}

	agentAddr := strings.TrimSpace(os.Getenv("HL_TEST_APPROVE_AGENT_ADDRESS"))
	if agentAddr != "" {
		t.Log("running approve-agent approve/revoke flow")

		stdout, stderr, code := runIntegrationHLGO(t,
			"account", "approve-agent",
			"--agent", agentAddr,
			"--name", "hlgo-integration",
		)
		requireIntegrationExitCode(t, code, 0, stderr)
		_ = parseJSONObject(t, stdout)

		stdout, stderr, code = runIntegrationHLGO(t,
			"account", "approve-agent",
			"--agent", agentAddr,
			"--revoke",
			"--confirm",
		)
		requireIntegrationExitCode(t, code, 0, stderr)
		_ = parseJSONObject(t, stdout)

		return
	}

	amount := strings.TrimSpace(os.Getenv("HL_TEST_ACCOUNT_TRANSFER_AMOUNT"))
	if amount == "" {
		t.Skip("skipping account integration: set HL_TEST_APPROVE_AGENT_ADDRESS for approve/revoke or HL_TEST_ACCOUNT_TRANSFER_AMOUNT for reversible transfer fallback")
	}

	t.Log("running reversible transfer fallback flow")
	reverted := false
	defer func() {
		if reverted {
			return
		}
		_, _, _ = runIntegrationHLGO(t,
			"account", "transfer",
			"--amount", amount,
			"--to-spot",
		)
	}()

	stdout, stderr, code := runIntegrationHLGO(t,
		"account", "transfer",
		"--amount", amount,
		"--to-perp",
	)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)

	stdout, stderr, code = runIntegrationHLGO(t,
		"account", "transfer",
		"--amount", amount,
		"--to-spot",
	)
	requireIntegrationExitCode(t, code, 0, stderr)
	_ = parseJSONObject(t, stdout)
	reverted = true
}
