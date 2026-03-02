package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/timbrinded/hlgo/pkg/output"
)

func TestAccountSubcommands_AllRegistered(t *testing.T) {
	root := NewRootCommand("test")
	var accountCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Use == "account" {
			accountCmd = c
			break
		}
	}
	if accountCmd == nil {
		t.Fatal("account command not found")
	}

	want := []string{"transfer", "withdraw", "class-transfer", "send-asset", "approve-agent", "set-abstraction"}
	cmds := make(map[string]bool)
	for _, sub := range accountCmd.Commands() {
		cmds[sub.Name()] = true
	}
	for _, name := range want {
		if !cmds[name] {
			t.Errorf("subcommand %q not registered on account", name)
		}
	}
}

func TestAccountTransfer_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("account", "transfer",
		"--amount", "100.25", "--to-perp", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "usdClassTransfer" {
		t.Errorf("type = %v, want usdClassTransfer", result["type"])
	}
	if result["amount"] != "100.25" {
		t.Errorf("amount = %v, want 100.25", result["amount"])
	}
}

func TestAccountTransfer_InvalidDirection(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("account", "transfer",
		"--amount", "1", "--dry-run",
	)
	if err == nil {
		t.Fatal("expected error when neither --to-perp nor --to-spot is set")
	}
}

func TestAccountWithdraw_RequiresConfirm(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("account", "withdraw",
		"--destination", "0x1111111111111111111111111111111111111111",
		"--amount", "1",
	)
	if err == nil {
		t.Fatal("expected confirmation error")
	}
}

func TestAccountWithdraw_DryRunSkipsConfirm(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("account", "withdraw",
		"--destination", "0x1111111111111111111111111111111111111111",
		"--amount", "1",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "withdraw3" {
		t.Errorf("type = %v, want withdraw3", result["type"])
	}
}

func TestAccountSendAsset_RequiresConfirm(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("account", "send-asset",
		"--destination", "0x1111111111111111111111111111111111111111",
		"--token", "PURR:0x1",
		"--amount", "1",
	)
	if err == nil {
		t.Fatal("expected confirmation error")
	}
}

func TestAccountSendAsset_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("account", "send-asset",
		"--destination", "0x1111111111111111111111111111111111111111",
		"--token", "PURR:0x1",
		"--amount", "1.5",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "spotSend" {
		t.Errorf("type = %v, want spotSend", result["type"])
	}
	if result["token"] != "PURR:0x1" {
		t.Errorf("token = %v, want PURR:0x1", result["token"])
	}
}

func TestAccountApproveAgent_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("account", "approve-agent",
		"--agent", "0x1111111111111111111111111111111111111111",
		"--name", "test-agent",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "approveAgent" {
		t.Errorf("type = %v, want approveAgent", result["type"])
	}
	if result["agentAddress"] != "0x1111111111111111111111111111111111111111" {
		t.Errorf("agentAddress = %v", result["agentAddress"])
	}
}

func TestAccountSetAbstraction_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("account", "set-abstraction",
		"--user", "0x1111111111111111111111111111111111111111",
		"--abstraction", "none",
		"--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "userSetAbstraction" {
		t.Errorf("type = %v, want userSetAbstraction", result["type"])
	}
}

func TestAccountClassTransfer_DryRun(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	err := run("account", "class-transfer",
		"--amount", "7.5", "--to-spot", "--dry-run",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}
	if result["type"] != "classTransfer" {
		t.Errorf("type = %v, want classTransfer", result["type"])
	}
}

func TestAccountMissingMasterKey_ConfigError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("agent_key_env: TEST_HL_KEY\nmaster_key_env: NOT_SET_MASTER\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_HL_KEY", "0x0123456789012345678901234567890123456789012345678901234567890123")

	root := NewRootCommand("test")
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{
		"--config", cfgPath,
		"account", "transfer",
		"--amount", "1",
		"--to-perp",
		"--dry-run",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected config error when master key env var is missing")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrConfig {
		t.Errorf("code = %q, want %q", cliErr.Code, output.ErrConfig)
	}
}
