package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/timbrinded/hlgo/pkg/config"
)

func TestNewRootCommand_DoesNotPanic(t *testing.T) {
	cmd := NewRootCommand(BuildInfo{Version: "test"})
	if cmd == nil {
		t.Fatal("NewRootCommand returned nil")
	}
	if cmd.Use != "hlgo" {
		t.Errorf("expected Use = %q, got %q", "hlgo", cmd.Use)
	}
}

func TestVersionCommand_OutputsVersion(t *testing.T) {
	want := BuildInfo{
		Version: "1.2.3",
		Commit:  "abc1234",
		Date:    "2026-02-25T08:00:00Z",
	}

	root := NewRootCommand(want)
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got BuildInfo
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("version output is not valid JSON: %v", err)
	}
	if got != want {
		t.Errorf("version output = %+v, want %+v", got, want)
	}
}

func TestSubcommands_Registered(t *testing.T) {
	root := NewRootCommand(BuildInfo{Version: "dev"})

	want := []string{"version", "info", "order", "position", "agent", "account", "config"}
	cmds := make(map[string]bool)
	for _, sub := range root.Commands() {
		cmds[sub.Use] = true
	}

	for _, name := range want {
		if !cmds[name] {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}

func TestGlobalFlags_Registered(t *testing.T) {
	root := NewRootCommand(BuildInfo{Version: "dev"})

	flags := []struct {
		name string
		def  string
	}{
		{"format", "json"},
		{"testnet", "false"},
		{"config", "~/.hlgo/config.yaml"},
		{"quiet", "false"},
		{"dry-run", "false"},
		{"dex", ""},
	}

	for _, f := range flags {
		pf := root.PersistentFlags().Lookup(f.name)
		if pf == nil {
			t.Errorf("flag --%s not registered", f.name)
			continue
		}
		if pf.DefValue != f.def {
			t.Errorf("flag --%s default = %q, want %q", f.name, pf.DefValue, f.def)
		}
	}
}

func TestConfigLoading_InjectsContext(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("agent_key_env: TEST_AGENT_KEY\nmetadata_ttl: 60\n")
	if err := os.WriteFile(cfgPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand(BuildInfo{Version: "test"})
	var gotCfg *config.Config
	root.AddCommand(&cobra.Command{
		Use: "test-ctx",
		RunE: func(cmd *cobra.Command, args []string) error {
			gotCfg = config.FromContext(cmd.Context())
			return nil
		},
	})

	root.SetArgs([]string{"--config", cfgPath, "test-ctx"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if gotCfg == nil {
		t.Fatal("config not injected into context")
	}
	if gotCfg.AgentKeyEnv != "TEST_AGENT_KEY" {
		t.Errorf("AgentKeyEnv = %q, want %q", gotCfg.AgentKeyEnv, "TEST_AGENT_KEY")
	}
}

func TestConfigLoading_SkippedForVersion(t *testing.T) {
	root := NewRootCommand(BuildInfo{Version: "1.0.0"})
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("version should not require config: %v", err)
	}
}

func TestConfigLoading_SkippedForHelp(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(":\x00bad"), 0600)

	root := NewRootCommand(BuildInfo{Version: "test"})
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"help", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("help should not fail with bad config: %v", err)
	}
}
