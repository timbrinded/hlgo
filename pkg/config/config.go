// Package config handles loading, validation, and access to hlgo configuration
// including wallet credentials and endpoint settings.
package config

import "github.com/spf13/viper"

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

// setDefaults registers default values for all config keys.
func setDefaults(v *viper.Viper) {
	v.SetDefault("agent_key_env", "HL_AGENT_KEY")
	v.SetDefault("master_key_env", "HL_MASTER_KEY")
	v.SetDefault("default_dex", "")
	v.SetDefault("metadata_ttl", 300)
}
