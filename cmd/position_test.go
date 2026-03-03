package cmd

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestPositionLeverage_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("position", "leverage",
		"--coin", "ETH", "--leverage", "5", "--mode", "cross", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "updateLeverage" {
		t.Errorf("type = %v, want updateLeverage", result["type"])
	}
	if result["asset"] != float64(1) {
		t.Errorf("asset = %v, want 1", result["asset"])
	}
	if result["isCross"] != true {
		t.Errorf("isCross = %v, want true", result["isCross"])
	}
	if result["leverage"] != float64(5) {
		t.Errorf("leverage = %v, want 5", result["leverage"])
	}
}

func TestPositionLeverage_RequiredFlags(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("position", "leverage", "--dry-run")
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestPositionLeverage_InvalidMode(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("position", "leverage",
		"--coin", "ETH", "--leverage", "5", "--mode", "hedge", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestPositionLeverage_ModeCaseInsensitive(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("position", "leverage",
		"--coin", "ETH", "--leverage", "5", "--mode", "Cross", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error for mixed-case mode: %v", err)
	}
}

func TestPositionLeverage_InvalidOnBehalfOf(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("position", "leverage",
		"--coin", "ETH", "--leverage", "5", "--mode", "cross",
		"--on-behalf-of", "not-a-hex-address", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid on-behalf-of address")
	}
}

func TestPositionLeverage_InvalidLeverage(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("position", "leverage",
		"--coin", "ETH", "--leverage", "0", "--mode", "cross", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for leverage < 1")
	}
}

func TestPositionMargin_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("position", "margin",
		"--coin", "BTC", "--side", "buy", "--amount", "100.5", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "updateIsolatedMargin" {
		t.Errorf("type = %v, want updateIsolatedMargin", result["type"])
	}
	if result["asset"] != float64(0) {
		t.Errorf("asset = %v, want 0", result["asset"])
	}
	if result["isBuy"] != true {
		t.Errorf("isBuy = %v, want true", result["isBuy"])
	}
	if result["ntli"] != float64(100500000) {
		t.Errorf("ntli = %v, want 100500000", result["ntli"])
	}
}

func TestPositionMargin_InvalidOnBehalfOf(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("position", "margin",
		"--coin", "BTC", "--side", "buy", "--amount", "100",
		"--on-behalf-of", "not-a-hex-address", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid on-behalf-of address")
	}
}

func TestPositionMargin_RequiredFlags(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("position", "margin", "--dry-run")
	if err == nil {
		t.Fatal("expected error for missing required flags")
	}
}

func TestPositionMargin_InvalidSide(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("position", "margin",
		"--coin", "BTC", "--side", "long", "--amount", "100", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error for invalid side")
	}
}

func TestPositionSubcommands_AllRegistered(t *testing.T) {
	root := NewRootCommand(BuildInfo{Version: "test"})
	var posCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Use == "position" {
			posCmd = c
			break
		}
	}
	if posCmd == nil {
		t.Fatal("position command not found")
	}

	want := []string{"leverage", "margin"}
	cmds := make(map[string]bool)
	for _, sub := range posCmd.Commands() {
		cmds[sub.Name()] = true
	}
	for _, name := range want {
		if !cmds[name] {
			t.Errorf("subcommand %q not registered on position", name)
		}
	}
}
