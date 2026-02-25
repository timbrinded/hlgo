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
		{"empty", "", ""},
		{"no 0x prefix", "abcdef1234567890", "abcd...7890"},
		// Edge cases: body too short for head/tail split
		{"0x 8-char body", "0xabcdefgh", "0xabcd..."},
		{"0x 9-char body", "0xabcdefghi", "0xabcd...fghi"},
		{"non-0x 8-char", "abcdefgh", "abcd..."},
		{"non-0x 9-char", "abcdefghi", "abcd...fghi"},
		{"non-0x 3-char", "abc", "****"},
		{"non-0x 4-char", "abcd", "abcd..."},
		{"0x only", "0x", "0x****"},
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
