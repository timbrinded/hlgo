package config

import (
	"context"
	"testing"
)

func TestContext_RoundTrip(t *testing.T) {
	cfg := &Config{PrivateKeyEnv: "TEST_KEY", MetadataTTL: 600}
	ctx := WithContext(context.Background(), cfg)
	got := FromContext(ctx)

	if got == nil {
		t.Fatal("FromContext returned nil")
	}
	if got.PrivateKeyEnv != "TEST_KEY" {
		t.Errorf("PrivateKeyEnv = %q, want %q", got.PrivateKeyEnv, "TEST_KEY")
	}
	if got.MetadataTTL != 600 {
		t.Errorf("MetadataTTL = %d, want %d", got.MetadataTTL, 600)
	}
}

func TestFromContext_Missing(t *testing.T) {
	got := FromContext(context.Background())
	if got != nil {
		t.Errorf("expected nil from empty context, got %+v", got)
	}
}
