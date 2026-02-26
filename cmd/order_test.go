package cmd

import (
	"encoding/json"
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

	want := []string{"place", "market", "cancel", "cancel-all"}
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
