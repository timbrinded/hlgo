//go:build e2e

package e2e

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

const agentSimulationTimeout = 3 * time.Minute

func TestE2E_AgentSimulation(t *testing.T) {
	agentKey := strings.TrimSpace(os.Getenv("HL_TEST_AGENT_KEY"))
	masterKey := strings.TrimSpace(os.Getenv("HL_TEST_MASTER_KEY"))
	if agentKey == "" || masterKey == "" {
		t.Skip("skipping agent simulation: set HL_TEST_AGENT_KEY and HL_TEST_MASTER_KEY")
	}

	t.Setenv("HL_AGENT_KEY", agentKey)
	t.Setenv("HL_MASTER_KEY", masterKey)

	start := time.Now()
	assertWithinTimeout := func(step string) {
		t.Helper()
		if elapsed := time.Since(start); elapsed > agentSimulationTimeout {
			t.Fatalf("agent simulation exceeded timeout at step %s: %s", step, elapsed)
		}
	}

	cloid := agentSimulationCloid(t)
	t.Logf("agent simulation cloid: %s", cloid)

	cleanup := func() {
		_, _, _ = runHlgo(t, "order", "cancel-all", "--format", "json")
	}
	defer cleanup()

	assertWithinTimeout("snapshot-pre")
	t.Log("step 1: agent snapshot")
	snapshotBefore := runStepObject(t, "agent snapshot", "agent", "snapshot", "--format", "json")
	if snapshotBefore["timestamp"] == nil {
		t.Fatalf("snapshot missing timestamp: %v", snapshotBefore)
	}

	assertWithinTimeout("mids")
	t.Log("step 2a: info mids")
	mids := runStepMids(t, "info", "mids", "--format", "json")
	ethMid := mids["ETH"]
	if ethMid == "" {
		t.Skip("skipping agent simulation: ETH mid unavailable on testnet")
	}

	assertWithinTimeout("funding")
	t.Log("step 2b: info funding --predicted")
	_ = runStepJSON(t, "info funding predicted", "info", "funding", "ETH", "--predicted", "--format", "json")

	assertWithinTimeout("leverage")
	t.Log("step 3: position leverage")
	_ = runStepObject(t, "position leverage", "position", "leverage", "--coin", "ETH", "--leverage", "5", "--format", "json")

	entry, tp, sl := restingBracketPrices(t, ethMid)

	assertWithinTimeout("bracket")
	t.Log("step 4: place agent bracket")
	_ = runStepObject(t, "agent bracket",
		"agent", "bracket",
		"--coin", "ETH",
		"--side", "buy",
		"--price", entry,
		"--size", "0.01",
		"--tp", tp,
		"--sl", sl,
		"--cloid", cloid,
		"--format", "json",
	)

	assertWithinTimeout("open-orders-after-place")
	t.Log("step 5: verify bracket appears in open orders")
	openOrdersAfterPlace := runStepArray(t, "open orders after place", "info", "open-orders", "--format", "json")
	if !containsOrderCloid(openOrdersAfterPlace, cloid) {
		t.Fatalf("bracket order with cloid %s not found in open orders", cloid)
	}

	assertWithinTimeout("snapshot-pnl")
	t.Log("step 6: snapshot + pnl")
	_ = runStepObject(t, "agent snapshot post-trade", "agent", "snapshot", "--format", "json")
	_ = runStepObject(t, "agent pnl", "agent", "pnl", "--format", "json")

	assertWithinTimeout("cancel-all")
	t.Log("step 7: cancel all")
	_ = runStepJSON(t, "order cancel-all", "order", "cancel-all", "--format", "json")

	assertWithinTimeout("open-orders-final")
	t.Log("step 8: verify clean open orders")
	openOrdersFinal := runStepArray(t, "open orders final", "info", "open-orders", "--format", "json")
	if containsOrderCloid(openOrdersFinal, cloid) {
		t.Fatalf("order with cloid %s still present after cancel-all", cloid)
	}
}

func runStepJSON(t *testing.T, name string, args ...string) any {
	t.Helper()
	stdout, stderr, exitCode := runHlgo(t, args...)
	if exitCode != 0 {
		t.Fatalf("%s failed (exit %d): %s", name, exitCode, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("%s wrote unexpected stderr: %s", name, stderr)
	}

	var payload any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("%s output is not valid JSON: %v\nraw: %s", name, err, stdout)
	}
	return payload
}

func runStepObject(t *testing.T, name string, args ...string) map[string]any {
	t.Helper()
	payload := runStepJSON(t, name, args...)
	obj, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("%s output = %T, want object", name, payload)
	}
	return obj
}

func runStepArray(t *testing.T, name string, args ...string) []map[string]any {
	t.Helper()
	payload := runStepJSON(t, name, args...)
	raw, ok := payload.([]any)
	if !ok {
		t.Fatalf("%s output = %T, want array", name, payload)
	}
	out := make([]map[string]any, 0, len(raw))
	for i, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("%s[%d] = %T, want object", name, i, item)
		}
		out = append(out, row)
	}
	return out
}

func runStepMids(t *testing.T, args ...string) map[string]string {
	t.Helper()
	payload := runStepJSON(t, "info mids", args...)
	obj, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("mids output = %T, want object", payload)
	}
	mids := make(map[string]string, len(obj))
	for k, v := range obj {
		if s, ok := v.(string); ok {
			mids[k] = s
		}
	}
	return mids
}

func containsOrderCloid(orders []map[string]any, cloid string) bool {
	for _, order := range orders {
		got, ok := order["cloid"].(string)
		if ok && got == cloid {
			return true
		}
	}
	return false
}

func restingBracketPrices(t *testing.T, midPrice string) (entry, tp, sl string) {
	t.Helper()

	mid, err := decimal.NewFromString(midPrice)
	if err != nil {
		t.Fatalf("invalid ETH mid %q: %v", midPrice, err)
	}
	if mid.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("invalid non-positive ETH mid %s", mid.String())
	}

	// Keep entry comfortably below market to reduce fill probability during the test.
	entryPx := mid.Mul(decimal.NewFromFloat(0.8)).Round(2)
	tpPx := entryPx.Mul(decimal.NewFromFloat(1.05)).Round(2)
	slPx := entryPx.Mul(decimal.NewFromFloat(0.95)).Round(2)
	return entryPx.String(), tpPx.String(), slPx.String()
}

func agentSimulationCloid(t *testing.T) string {
	t.Helper()

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("failed to generate cloid: %v", err)
	}
	return fmt.Sprintf("0x%s", hex.EncodeToString(raw[:]))
}
