// Package resolver maps human-readable coin names to Hyperliquid integer asset IDs
// across perp and spot markets.
//
// It fetches metadata from the Hyperliquid /info API, caches it to disk with
// a configurable TTL, and resolves coins case-insensitively. Numeric string
// passthroughs (e.g. "1") return the literal asset ID without a metadata lookup.
package resolver

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/timbrinded/hlgo/pkg/output"
)

// InfoFetcher abstracts the subset of client.Client that the resolver needs.
// This allows testing with mocks instead of requiring a real HTTP client.
type InfoFetcher interface {
	PostInfo(ctx context.Context, request any) (json.RawMessage, error)
}

// Resolver maps human-readable coin names to Hyperliquid asset IDs.
type Resolver interface {
	// ResolveAsset resolves a coin name (e.g. "ETH", "PURR") to its asset info.
	// For numeric strings (e.g. "1"), returns a passthrough with the numeric ID.
	ResolveAsset(ctx context.Context, coin string) (*AssetInfo, error)
}

// AssetInfo contains the resolved asset metadata.
//
// When Passthrough is true, the asset was resolved from a raw numeric ID string.
// In this case SzDecimals and IsSpot are zero-value defaults — callers must supply
// their own metadata for wire formatting and market-type decisions.
type AssetInfo struct {
	AssetID     int
	Coin        string
	SzDecimals  int
	IsSpot      bool
	Passthrough bool // true when resolved from a numeric ID string (metadata unknown)
}

// perpMeta is the response shape from POST /info {"type":"meta"}.
type perpMeta struct {
	Universe []perpAsset `json:"universe"`
}

// perpAsset is a single entry in the perp universe array.
type perpAsset struct {
	Name       string `json:"name"`
	SzDecimals int    `json:"szDecimals"`
}

// spotMeta is the response shape from POST /info {"type":"spotMeta"}.
type spotMeta struct {
	Universe []spotMarket `json:"universe"`
}

// spotMarket is a single spot market in the spot universe array.
type spotMarket struct {
	Name   string      `json:"name"`
	Index  int         `json:"index"`
	Tokens []spotToken `json:"tokens"`
}

// spotToken is a token within a spot market.
type spotToken struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	SzDecimals int    `json:"szDecimals"`
}

// CachingResolver implements Resolver with a disk-backed, TTL-based cache.
// It is safe for concurrent use.
type CachingResolver struct {
	client  InfoFetcher
	cache   *diskCache
	ttl     time.Duration
	mu      sync.RWMutex
	perpMap map[string]*AssetInfo // upper-case coin → info
	spotMap map[string]*AssetInfo // upper-case coin → info
	loaded  bool
	nowFunc func() time.Time // injectable clock for testing
}

// NewResolver creates a CachingResolver that fetches metadata via client,
// caches to cacheDir with the given TTL.
func NewResolver(c InfoFetcher, cacheDir string, ttl time.Duration) *CachingResolver {
	return &CachingResolver{
		client:  c,
		cache:   newDiskCache(cacheDir),
		ttl:     ttl,
		perpMap: make(map[string]*AssetInfo),
		spotMap: make(map[string]*AssetInfo),
		nowFunc: time.Now,
	}
}

// ResolveAsset resolves a coin name to its asset info. Numeric strings are
// passed through directly. All coin name comparisons are case-insensitive.
func (r *CachingResolver) ResolveAsset(ctx context.Context, coin string) (*AssetInfo, error) {
	// Numeric passthrough: "1" → asset ID 1.
	// Callers must check Passthrough and supply their own SzDecimals/IsSpot
	// for wire formatting — these defaults are not authoritative.
	if trimmed := strings.TrimSpace(coin); trimmed != "" {
		if id, err := strconv.Atoi(trimmed); err == nil {
			if id < 0 {
				return nil, output.NewCLIError(output.ErrValidation, "invalid numeric asset ID: must be non-negative").
					WithDetails("coin", coin).
					WithDetails("hint", "use a non-negative numeric asset ID or a valid coin name (e.g. BTC, ETH)")
			}
			return &AssetInfo{
				AssetID:     id,
				Coin:        trimmed,
				SzDecimals:  0,
				IsSpot:      false,
				Passthrough: true,
			}, nil
		}
	}

	if err := r.ensureLoaded(ctx); err != nil {
		return nil, err
	}

	upper := strings.ToUpper(coin)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, ok := r.perpMap[upper]; ok {
		return info, nil
	}
	if info, ok := r.spotMap[upper]; ok {
		return info, nil
	}

	return nil, output.NewCLIError(output.ErrValidation, "unknown coin: "+coin).
		WithDetails("coin", coin).
		WithDetails("hint", "use a valid coin name (e.g. BTC, ETH) or a numeric asset ID")
}

// ensureLoaded loads metadata from cache or API if not already loaded.
//
// Once loaded is set, metadata is served from memory for the lifetime of this
// resolver instance. Per SOUL.md "Stateless Simplicity", the typical lifecycle
// is create → resolve → discard within a single CLI command, so TTL re-checking
// is unnecessary. If this resolver is ever reused across long-running contexts,
// this assumption would need revisiting.
func (r *CachingResolver) ensureLoaded(ctx context.Context) error {
	r.mu.RLock()
	if r.loaded {
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock.
	if r.loaded {
		return nil
	}

	now := r.nowFunc()

	// Try loading from disk cache first.
	perpData, perpFresh := r.cache.read("meta.json", r.ttl, now)
	spotData, spotFresh := r.cache.read("spot_meta.json", r.ttl, now)

	if perpFresh && spotFresh {
		if err := r.buildMaps(perpData, spotData); err == nil {
			r.loaded = true
			return nil
		}
		// Corrupt cache — fall through to fetch.
	}

	// Fetch from API.
	perpData, err := r.fetchMeta(ctx, "meta")
	if err != nil {
		return err
	}
	spotData, err = r.fetchMeta(ctx, "spotMeta")
	if err != nil {
		return err
	}

	if err := r.buildMaps(perpData, spotData); err != nil {
		return err
	}

	// Write cache (best-effort — failure here is non-fatal).
	r.cache.write("meta.json", perpData, now)
	r.cache.write("spot_meta.json", spotData, now)

	r.loaded = true
	return nil
}

// fetchMeta sends a typed info request and returns the raw JSON bytes.
func (r *CachingResolver) fetchMeta(ctx context.Context, metaType string) ([]byte, error) {
	raw, err := r.client.PostInfo(ctx, map[string]string{"type": metaType})
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// buildMaps parses perp and spot metadata and populates the lookup maps.
func (r *CachingResolver) buildMaps(perpData, spotData []byte) error {
	var pm perpMeta
	if err := json.Unmarshal(perpData, &pm); err != nil {
		return output.NewCLIError(output.ErrAPI, "failed to parse perp metadata").
			WithDetails("cause", err.Error())
	}

	var sm spotMeta
	if err := json.Unmarshal(spotData, &sm); err != nil {
		return output.NewCLIError(output.ErrAPI, "failed to parse spot metadata").
			WithDetails("cause", err.Error())
	}

	perpMap := make(map[string]*AssetInfo, len(pm.Universe))
	for i, asset := range pm.Universe {
		upper := strings.ToUpper(asset.Name)
		perpMap[upper] = &AssetInfo{
			AssetID:    i, // perp asset ID = array index
			Coin:       asset.Name,
			SzDecimals: asset.SzDecimals,
			IsSpot:     false,
		}
	}

	spotMap := make(map[string]*AssetInfo, len(sm.Universe))
	for _, market := range sm.Universe {
		if len(market.Tokens) == 0 {
			continue
		}
		// The first token is the base token of the spot pair.
		token := market.Tokens[0]
		info := &AssetInfo{
			AssetID:    10000 + token.Index, // spot asset ID = 10000 + index
			Coin:       token.Name,
			SzDecimals: token.SzDecimals,
			IsSpot:     true,
		}
		// Allow resolution by both base token ("PURR") and full market name ("PURR/USDC").
		baseKey := strings.ToUpper(token.Name)
		spotMap[baseKey] = info
		marketKey := strings.ToUpper(market.Name)
		spotMap[marketKey] = info
	}

	r.perpMap = perpMap
	r.spotMap = spotMap
	return nil
}
