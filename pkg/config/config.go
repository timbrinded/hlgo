// Package config handles loading, validation, and access to hlgo configuration
// including wallet credentials and endpoint settings.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// DefaultConfigPath is the sentinel value for the default config location.
const DefaultConfigPath = "~/.hlgo/config.yaml"

// Config holds all resolved configuration for an hlgo invocation.
type Config struct {
	// Persisted fields (written to config file)
	AgentKeyEnv  string `mapstructure:"agent_key_env"  yaml:"agent_key_env"`
	MasterKeyEnv string `mapstructure:"master_key_env" yaml:"master_key_env"`
	DefaultDex   string `mapstructure:"default_dex"    yaml:"default_dex"`
	MetadataTTL  int    `mapstructure:"metadata_ttl"   yaml:"metadata_ttl"`

	// Runtime fields (resolved from flags/env, never persisted)
	Testnet bool   `mapstructure:"-" yaml:"-"`
	Format  string `mapstructure:"-" yaml:"-"`
	DryRun  bool   `mapstructure:"-" yaml:"-"`
	Quiet   bool   `mapstructure:"-" yaml:"-"`
	Dex     string `mapstructure:"-" yaml:"-"`
}

// Load resolves the config file, reads it, merges with flags/env, and returns
// a typed Config. The viper instance must have flags already bound.
func Load(v *viper.Viper) (*Config, error) {
	setDefaults(v)

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	configPath := v.GetString("config")
	if configPath != "" && configPath != DefaultConfigPath {
		v.SetConfigFile(configPath)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".hlgo"))
		}
		xdgConfig, err := os.UserConfigDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(xdgConfig, "hlgo"))
		}
	}

	if err := v.ReadInConfig(); err != nil {
		// Tolerate missing config files — both search-path miss
		// (ConfigFileNotFoundError) and explicit-path miss (os.ErrNotExist).
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	cfg.Testnet = v.GetBool("testnet")
	cfg.Format = v.GetString("format")
	cfg.DryRun = v.GetBool("dry-run")
	cfg.Quiet = v.GetBool("quiet")

	if dex := v.GetString("dex"); dex != "" {
		cfg.Dex = dex
	} else {
		cfg.Dex = cfg.DefaultDex
	}

	return &cfg, nil
}

// setDefaults registers default values for all config keys.
func setDefaults(v *viper.Viper) {
	v.SetDefault("agent_key_env", "HL_AGENT_KEY")
	v.SetDefault("master_key_env", "HL_MASTER_KEY")
	v.SetDefault("default_dex", "")
	v.SetDefault("metadata_ttl", 300)
}
