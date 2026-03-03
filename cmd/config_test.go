package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInit_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCommand(BuildInfo{Version: "test"})
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
	if !strings.Contains(content, "private_key_env") {
		t.Error("config file missing private_key_env")
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

	root := NewRootCommand(BuildInfo{Version: "test"})
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

	root := NewRootCommand(BuildInfo{Version: "test"})
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

	root := NewRootCommand(BuildInfo{Version: "test"})
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{
		"config", "init",
		"--config", cfgPath,
		"--private-key-env", "CUSTOM_KEY",
		"--default-dex", "xyz",
		"--metadata-ttl", "60",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	data, _ := os.ReadFile(cfgPath)
	content := string(data)
	if !strings.Contains(content, "CUSTOM_KEY") {
		t.Error("custom private key env not written")
	}
}

func TestConfigInit_WarnsOnMissingEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	root := NewRootCommand(BuildInfo{Version: "test"})
	stderr := new(bytes.Buffer)
	root.SetErr(stderr)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"config", "init", "--config", cfgPath})

	root.Execute()

	if !strings.Contains(stderr.String(), "not set") {
		t.Errorf("expected warning about unset env var, got stderr: %q", stderr.String())
	}
}

func TestConfigShow_Output(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("private_key_env: TEST_KEY\nmetadata_ttl: 120\n"), 0600)
	t.Setenv("TEST_KEY", "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")

	root := NewRootCommand(BuildInfo{Version: "test"})
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"config", "show", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("stdout not valid JSON: %v\nraw: %s", err, buf.String())
	}

	if out["private_key_set"] != true {
		t.Error("private_key_set should be true")
	}

	preview, _ := out["private_key_preview"].(string)
	if !strings.Contains(preview, "...") {
		t.Errorf("expected redacted preview, got %q", preview)
	}
	// Full key must not appear in output
	if strings.Contains(buf.String(), "1234567890abcdef1234567890abcdef1234567890abcdef12") {
		t.Error("full key material appears in output")
	}
}

func TestConfigTest_AllGood(t *testing.T) {
	// Mock server returning valid mids JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"BTC":"95000","ETH":"3400"}`)
	}))
	defer srv.Close()
	t.Setenv("HLGO_API_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("private_key_env: TEST_KEY\n"), 0600)
	t.Setenv("TEST_KEY", "0xdeadbeef")

	root := NewRootCommand(BuildInfo{Version: "test"})
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"config", "test", "--config", cfgPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("stdout not valid JSON: %v", err)
	}
	if out["config_readable"] != true {
		t.Error("config_readable should be true")
	}
	if out["private_key_env_set"] != true {
		t.Error("private_key_env_set should be true")
	}

	conn, ok := out["connectivity"].(map[string]any)
	if !ok {
		t.Fatal("connectivity field missing or not a map")
	}
	if conn["status"] != "ok" {
		t.Errorf("connectivity status = %v, want ok", conn["status"])
	}
	if coins, ok := conn["coins"].(float64); !ok || coins < 1 {
		t.Errorf("connectivity coins = %v, want >= 1", conn["coins"])
	}
}

func TestConfigTest_NoFileReportsNotReadable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TEST_KEY", "0xdeadbeef")

	root := NewRootCommand(BuildInfo{Version: "test"})
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"config", "test"})

	// Will error because private key env defaults to HL_PRIVATE_KEY which isn't set,
	// but we care about config_readable in the JSON output.
	root.Execute()

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("stdout not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if out["config_readable"] != false {
		t.Error("config_readable should be false when no config file exists")
	}
}

func TestConfigTest_MissingEnvVar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"BTC":"95000"}`)
	}))
	defer srv.Close()
	t.Setenv("HLGO_API_URL", srv.URL)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("private_key_env: DEFINITELY_NOT_SET_XYZ\n"), 0600)

	root := NewRootCommand(BuildInfo{Version: "test"})
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"config", "test", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when private key env var is not set")
	}
}
