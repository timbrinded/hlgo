package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	tests := []struct {
		key  string
		want any
	}{
		{"private_key_env", "HL_PRIVATE_KEY"},
		{"account_address", ""},
		{"default_dex", ""},
		{"metadata_ttl", 300},
	}

	for _, tt := range tests {
		got := v.Get(tt.key)
		if got != tt.want {
			t.Errorf("default %s = %v, want %v", tt.key, got, tt.want)
		}
	}
}

// newTestViper creates a viper instance with standard runtime flag values set.
func newTestViper(configPath string) *viper.Viper {
	v := viper.New()
	v.Set("config", configPath)
	v.Set("testnet", false)
	v.Set("format", "json")
	v.Set("dry-run", false)
	v.Set("quiet", false)
	v.Set("dex", "")
	return v
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("private_key_env: MY_KEY\naccount_address: 0x1111111111111111111111111111111111111111\ndefault_dex: xyz\nmetadata_ttl: 120\n")
	if err := os.WriteFile(cfgPath, content, 0600); err != nil {
		t.Fatal(err)
	}

	v := newTestViper(cfgPath)
	v.Set("testnet", true)
	v.Set("format", "table")

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.PrivateKeyEnv != "MY_KEY" {
		t.Errorf("PrivateKeyEnv = %q, want %q", cfg.PrivateKeyEnv, "MY_KEY")
	}
	if cfg.DefaultDex != "xyz" {
		t.Errorf("DefaultDex = %q, want %q", cfg.DefaultDex, "xyz")
	}
	if cfg.AccountAddress != "0x1111111111111111111111111111111111111111" {
		t.Errorf("AccountAddress = %q, want %q", cfg.AccountAddress, "0x1111111111111111111111111111111111111111")
	}
	if cfg.MetadataTTL != 120 {
		t.Errorf("MetadataTTL = %d, want %d", cfg.MetadataTTL, 120)
	}
	if !cfg.Testnet {
		t.Error("Testnet should be true")
	}
	if cfg.Format != "table" {
		t.Errorf("Format = %q, want %q", cfg.Format, "table")
	}
}

func TestLoad_MissingConfigTolerated(t *testing.T) {
	v := newTestViper(filepath.Join(t.TempDir(), "nonexistent.yaml"))

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("expected no error for missing config, got: %v", err)
	}
	if cfg.PrivateKeyEnv != "HL_PRIVATE_KEY" {
		t.Errorf("PrivateKeyEnv = %q, want default %q", cfg.PrivateKeyEnv, "HL_PRIVATE_KEY")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(":\x00bad"), 0600); err != nil {
		t.Fatal(err)
	}

	v := newTestViper(cfgPath)

	_, err := Load(v)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_DexFlagOverridesDefault(t *testing.T) {
	v := newTestViper(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	v.Set("dex", "trove")

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Dex != "trove" {
		t.Errorf("Dex = %q, want %q", cfg.Dex, "trove")
	}
}

func TestLoad_DefaultDexUsedWhenNoFlag(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("default_dex: xyz\n"), 0600); err != nil {
		t.Fatal(err)
	}

	v := newTestViper(cfgPath)

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Dex != "xyz" {
		t.Errorf("Dex = %q, want %q (from default_dex)", cfg.Dex, "xyz")
	}
}

func TestLoad_XDGFallback(t *testing.T) {
	// Override HOME so ~/.hlgo doesn't exist, forcing XDG fallback.
	t.Setenv("HOME", t.TempDir())
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	hlgoDir := filepath.Join(xdgDir, "hlgo")
	os.MkdirAll(hlgoDir, 0700)
	cfgPath := filepath.Join(hlgoDir, "config.yaml")
	os.WriteFile(cfgPath, []byte("private_key_env: XDG_FOUND\n"), 0600)

	// Use default config path so discovery kicks in via AddConfigPath
	v := newTestViper(DefaultConfigPath)

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.PrivateKeyEnv != "XDG_FOUND" {
		t.Errorf("PrivateKeyEnv = %q, want %q (from XDG path)", cfg.PrivateKeyEnv, "XDG_FOUND")
	}
}
