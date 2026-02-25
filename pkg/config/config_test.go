package config

import (
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
		{"agent_key_env", "HL_AGENT_KEY"},
		{"master_key_env", "HL_MASTER_KEY"},
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
