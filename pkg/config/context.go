package config

import "context"

type ctxKey struct{}

// WithContext returns a new context carrying cfg.
func WithContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, ctxKey{}, cfg)
}

// FromContext extracts the Config from ctx, or nil if not present.
func FromContext(ctx context.Context) *Config {
	cfg, _ := ctx.Value(ctxKey{}).(*Config)
	return cfg
}
