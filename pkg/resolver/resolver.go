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

const (
	defaultCoreMetadataTimeout = 3 * time.Second
	defaultHIP3MetadataTimeout = 2 * time.Second
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
	AssetID       int
	Coin          string
	CanonicalCoin string
	SzDecimals    int
	IsSpot        bool
	Passthrough   bool // true when resolved from a numeric ID string (metadata unknown)
}

// CachingResolver implements Resolver with a disk-backed, TTL-based cache.
// It is safe for concurrent use.
type CachingResolver struct {
	client          InfoFetcher
	cache           *diskCache
	ttl             time.Duration
	mu              sync.RWMutex
	perpMap         map[string]*AssetInfo          // upper-case coin → info
	spotMap         map[string]*AssetInfo          // upper-case coin → info
	spotPairAliases map[string][]spotPairCandidate // upper-case BASE/QUOTE → spot candidates
	loadedCore      bool
	loadedHIP3Dex   map[string]bool // lower-case dex name → loaded in perpMap
	nowFunc         func() time.Time
	coreTimeout     time.Duration
	hip3Timeout     time.Duration
}

// NewResolver creates a CachingResolver that fetches metadata via client,
// caches to cacheDir with the given TTL.
func NewResolver(c InfoFetcher, cacheDir string, ttl time.Duration) *CachingResolver {
	return &CachingResolver{client: c, cache: newDiskCache(cacheDir), ttl: ttl, perpMap: make(map[string]*AssetInfo), spotMap: make(map[string]*AssetInfo), spotPairAliases: make(map[string][]spotPairCandidate), loadedHIP3Dex: make(map[string]bool), nowFunc: time.Now, coreTimeout: defaultCoreMetadataTimeout, hip3Timeout: defaultHIP3MetadataTimeout}
}

// ResolveAsset resolves a coin name to its asset info. Numeric strings are
// passed through directly. All coin name comparisons are case-insensitive.
func (r *CachingResolver) ResolveAsset(ctx context.Context, coin string) (*AssetInfo, error) {
	trimmed := strings.TrimSpace(coin)

	// Numeric passthrough: "1" → asset ID 1.
	// Callers must check Passthrough and supply their own SzDecimals/IsSpot
	// for wire formatting — these defaults are not authoritative.
	if trimmed != "" {
		if id, err := strconv.Atoi(trimmed); err == nil {
			if id < 0 {
				return nil, output.NewCLIError(output.ErrValidation, "invalid numeric asset ID: must be non-negative").
					WithDetails("coin", coin).
					WithDetails("hint", "use a non-negative numeric asset ID or a valid coin name (e.g. BTC, ETH)")
			}
			return &AssetInfo{AssetID: id, Coin: trimmed, CanonicalCoin: trimmed, SzDecimals: 0, IsSpot: false, Passthrough: true}, nil
		}
	}

	targetHIP3Dex := parseHIP3Dex(trimmed)
	if err := r.ensureLoaded(ctx, targetHIP3Dex); err != nil {
		return nil, err
	}

	upper := strings.ToUpper(trimmed)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, ok := r.perpMap[upper]; ok {
		return info, nil
	}
	if info, ok := r.spotMap[upper]; ok {
		return info, nil
	}
	if candidates, ok := r.spotPairAliases[upper]; ok {
		return resolveSpotPairAlias(coin, candidates)
	}

	return nil, output.NewCLIError(output.ErrValidation, "unknown coin: "+coin).
		WithDetails("coin", coin).
		WithDetails("hint", "use a valid coin name (e.g. BTC, ETH), spot pair (e.g. ETH/USDC), or a numeric asset ID")
}

// ensureLoaded loads metadata from cache or API if not already loaded.
//
// Once loadedCore is set, metadata is served from memory for the lifetime of this
// resolver instance. Per SOUL.md "Stateless Simplicity", the typical lifecycle
// is create → resolve → discard within a single CLI command, so TTL re-checking
// is unnecessary. If this resolver is ever reused across long-running contexts,
// this assumption would need revisiting.
func (r *CachingResolver) ensureLoaded(ctx context.Context, targetHIP3Dex string) error {
	dexKey := strings.ToLower(strings.TrimSpace(targetHIP3Dex))

	r.mu.RLock()
	if r.loadedCore && (dexKey == "" || r.loadedHIP3Dex[dexKey]) {
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock.
	if r.loadedCore && (dexKey == "" || r.loadedHIP3Dex[dexKey]) {
		return nil
	}

	if !r.loadedCore {
		if err := r.loadCoreMetadataLocked(ctx); err != nil {
			return err
		}
	}

	if dexKey != "" && !r.loadedHIP3Dex[dexKey] {
		if err := r.loadHIP3DexLocked(ctx, dexKey); err != nil {
			return err
		}
		r.loadedHIP3Dex[dexKey] = true
	}

	return nil
}
