package info

import (
	"errors"
	"testing"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/output"
)

func TestResolveUserAddress_ExplicitValid(t *testing.T) {
	addr, err := ResolveUserAddress("0x14791697260E4c9A71f18484C9f997B308e59325", &config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "0x14791697260E4c9A71f18484C9f997B308e59325" {
		t.Errorf("got %q, want explicit address", addr)
	}
}

func TestResolveUserAddress_ExplicitInvalid(t *testing.T) {
	tests := []string{
		"not-an-address",
		"0x123",
		"14791697260E4c9A71f18484C9f997B308e59325",   // missing 0x
		"0xGGGG1697260E4c9A71f18484C9f997B308e59325", // invalid hex
	}
	for _, addr := range tests {
		_, err := ResolveUserAddress(addr, &config.Config{})
		if err == nil {
			t.Errorf("expected error for address %q", addr)
			continue
		}
		var cliErr *output.CLIError
		if !errors.As(err, &cliErr) {
			t.Errorf("expected *output.CLIError for %q, got %T", addr, err)
			continue
		}
		if cliErr.Code != output.ErrValidation {
			t.Errorf("expected VALIDATION_ERROR for %q, got %s", addr, cliErr.Code)
		}
	}
}

func TestResolveUserAddress_FromAgentKey(t *testing.T) {
	// Use the well-known test key.
	t.Setenv("TEST_AGENT_KEY", "0x0123456789012345678901234567890123456789012345678901234567890123")

	cfg := &config.Config{AgentKeyEnv: "TEST_AGENT_KEY"}
	addr, err := ResolveUserAddress("", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "0x14791697260E4c9A71f18484C9f997B308e59325" {
		t.Errorf("got %q, want test key address", addr)
	}
}

func TestResolveUserAddress_NoAddressAvailable(t *testing.T) {
	t.Setenv("HL_AGENT_KEY", "")

	cfg := &config.Config{AgentKeyEnv: "HL_AGENT_KEY"}
	_, err := ResolveUserAddress("", cfg)
	if err == nil {
		t.Fatal("expected error when no address available")
	}
	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrConfig {
		t.Errorf("expected CONFIG_ERROR, got %s", cliErr.Code)
	}
}
