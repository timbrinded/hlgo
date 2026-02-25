package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInit_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCommand("test")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"config", "init", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "agent_key_env") {
		t.Error("config file missing agent_key_env")
	}

	var out map[string]string
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("stdout not valid JSON: %v", err)
	}
	if out["path"] != cfgPath {
		t.Errorf("output path = %q, want %q", out["path"], cfgPath)
	}
}

func TestConfigInit_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("existing"), 0600)

	root := NewRootCommand("test")
	root.SetArgs([]string{"config", "init", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when config exists without --force")
	}
}

func TestConfigInit_Force(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("old"), 0600)

	root := NewRootCommand("test")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"config", "init", "--config", cfgPath, "--force"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error with --force: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	if string(data) == "old" {
		t.Error("config file was not overwritten")
	}
}

func TestConfigInit_CustomValues(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCommand("test")
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{
		"config", "init",
		"--config", cfgPath,
		"--agent-key-env", "CUSTOM_AGENT",
		"--master-key-env", "CUSTOM_MASTER",
		"--default-dex", "xyz",
		"--metadata-ttl", "60",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	content := string(data)
	if !strings.Contains(content, "CUSTOM_AGENT") {
		t.Error("custom agent key env not written")
	}
	if !strings.Contains(content, "CUSTOM_MASTER") {
		t.Error("custom master key env not written")
	}
}

func TestConfigInit_WarnsOnMissingEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCommand("test")
	stderr := new(bytes.Buffer)
	root.SetErr(stderr)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"config", "init", "--config", cfgPath})

	root.Execute()

	if !strings.Contains(stderr.String(), "not set") {
		t.Errorf("expected warning about unset env var, got stderr: %q", stderr.String())
	}
}
