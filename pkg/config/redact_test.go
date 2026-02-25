package config

import "testing"

func TestRedactKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"normal hex", "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", "0xabcd...7890"},
		{"short key", "0xabc", "0x****"},
		{"exactly 10 chars", "0xabcdefgh", "0xabcd...efgh"},
		{"empty", "", ""},
		{"no 0x prefix", "abcdef1234567890", "abcd...7890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactKey(tt.key)
			if got != tt.want {
				t.Errorf("RedactKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
