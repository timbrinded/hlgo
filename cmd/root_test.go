package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewRootCommand_DoesNotPanic(t *testing.T) {
	cmd := NewRootCommand("test")
	if cmd == nil {
		t.Fatal("NewRootCommand returned nil")
	}
	if cmd.Use != "hlgo" {
		t.Errorf("expected Use = %q, got %q", "hlgo", cmd.Use)
	}
}

func TestVersionCommand_OutputsVersion(t *testing.T) {
	const want = "1.2.3"

	root := NewRootCommand(want)
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != want {
		t.Errorf("version output = %q, want %q", got, want)
	}
}

func TestSubcommands_Registered(t *testing.T) {
	root := NewRootCommand("dev")

	want := []string{"version", "info", "order", "position", "account", "config"}
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
	root := NewRootCommand("dev")

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
