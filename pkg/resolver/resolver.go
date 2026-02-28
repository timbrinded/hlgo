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
	"errors"
	"sort"
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

type spotPairCandidate struct {
	info        *AssetInfo
	marketName  string
	marketIndex int
	canonical   bool
}

// perpMeta is the response shape from POST /info {"type":"meta"}.
type perpMeta struct {
	Universe []perpAsset `json:"universe"`
}

type perpDex struct {
	Name string `json:"name"`
}

// perpAsset is a single entry in the perp universe array.
type perpAsset struct {
	Name       string `json:"name"`
	SzDecimals int    `json:"szDecimals"`
}

// spotMeta is the response shape from POST /info {"type":"spotMeta"}.
// The real API returns a two-level structure: a flat top-level "tokens" registry
// and universe entries that reference tokens by integer index.
type spotMeta struct {
	Universe []spotMarket `json:"universe"`
	Tokens   []spotToken  `json:"tokens"` // top-level token registry
}

// spotMarket is a single spot market in the spot universe array.
type spotMarket struct {
	Name        string `json:"name"`
	Index       int    `json:"index"`
	Tokens      []int  `json:"tokens"`      // indices into spotMeta.Tokens
	IsCanonical bool   `json:"isCanonical"` // optional on some API responses
}

// spotToken is a token within a spot market.
type spotToken struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	SzDecimals int    `json:"szDecimals"`
	FullName   string `json:"fullName"`
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
	return &CachingResolver{
		client:          c,
		cache:           newDiskCache(cacheDir),
		ttl:             ttl,
		perpMap:         make(map[string]*AssetInfo),
		spotMap:         make(map[string]*AssetInfo),
		spotPairAliases: make(map[string][]spotPairCandidate),
		loadedHIP3Dex:   make(map[string]bool),
		nowFunc:         time.Now,
		coreTimeout:     defaultCoreMetadataTimeout,
		hip3Timeout:     defaultHIP3MetadataTimeout,
	}
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
			return &AssetInfo{
				AssetID:       id,
				Coin:          trimmed,
				CanonicalCoin: trimmed,
				SzDecimals:    0,
				IsSpot:        false,
				Passthrough:   true,
			}, nil
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

func (r *CachingResolver) loadCoreMetadataLocked(ctx context.Context) error {
	now := r.nowFunc()

	// Try loading from disk cache first.
	perpData, perpFresh := r.cache.read("meta.json", r.ttl, now)
	spotData, spotFresh := r.cache.read("spot_meta.json", r.ttl, now)

	if perpFresh && spotFresh {
		if err := r.buildMaps(perpData, spotData); err == nil {
			r.loadedCore = true
			return nil
		}
		// Corrupt cache — fall through to fetch.
	}

	// Fetch from API.
	perpData, err := r.fetchMetaWithTimeout(ctx, "perp_meta", "meta", "", r.coreTimeout)
	if err != nil {
		return err
	}
	spotData, err = r.fetchMetaWithTimeout(ctx, "spot_meta", "spotMeta", "", r.coreTimeout)
	if err != nil {
		return err
	}

	if err := r.buildMaps(perpData, spotData); err != nil {
		return err
	}

	// Write cache (best-effort — failure here is non-fatal).
	r.cache.write("meta.json", perpData, now)
	r.cache.write("spot_meta.json", spotData, now)

	r.loadedCore = true
	return nil
}

func (r *CachingResolver) loadHIP3DexLocked(ctx context.Context, dex string) error {
	offsets, err := r.fetchPerpDexOffsetsWithTimeout(ctx)
	if err != nil {
		return err
	}

	offset, ok := offsets[dex]
	if !ok {
		return output.NewCLIError(output.ErrValidation, "unknown HIP-3 dex: "+dex).
			WithDetails("dex", dex)
	}

	metaRaw, err := r.fetchMetaWithTimeout(ctx, "hip3_meta", "meta", dex, r.hip3Timeout)
	if err != nil {
		return err
	}

	var pm perpMeta
	if err := json.Unmarshal(metaRaw, &pm); err != nil {
		return output.NewCLIError(output.ErrAPI, "failed to parse HIP-3 metadata").
			WithDetails("dex", dex).
			WithDetails("cause", err.Error())
	}

	for i, asset := range pm.Universe {
		upper := strings.ToUpper(asset.Name)
		r.perpMap[upper] = &AssetInfo{
			AssetID:       offset + i,
			Coin:          asset.Name,
			CanonicalCoin: asset.Name,
			SzDecimals:    asset.SzDecimals,
			IsSpot:        false,
		}
	}

	return nil
}

// fetchMeta sends a typed info request and returns the raw JSON bytes.
func (r *CachingResolver) fetchMeta(ctx context.Context, metaType, dex string) ([]byte, error) {
	req := map[string]string{"type": metaType}
	if dex != "" {
		req["dex"] = dex
	}
	return r.client.PostInfo(ctx, req)
}

func (r *CachingResolver) fetchMetaWithTimeout(ctx context.Context, stage, metaType, dex string, timeout time.Duration) ([]byte, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	raw, err := r.fetchMeta(requestCtx, metaType, dex)
	if err == nil {
		return raw, nil
	}

	if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
		return nil, output.NewCLIError(output.ErrNetwork, "metadata request timed out").
			WithDetails("stage", stage).
			WithDetails("type", metaType).
			WithDetails("dex", dex).
			WithDetails("timeout_ms", timeout.Milliseconds())
	}

	return nil, wrapMetadataError(err, stage, metaType, dex, timeout)
}

func (r *CachingResolver) fetchPerpDexOffsetsWithTimeout(ctx context.Context) (map[string]int, error) {
	requestCtx, cancel := context.WithTimeout(ctx, r.hip3Timeout)
	defer cancel()

	offsets, err := r.fetchPerpDexOffsets(requestCtx)
	if err == nil {
		return offsets, nil
	}

	if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
		return nil, output.NewCLIError(output.ErrNetwork, "metadata request timed out").
			WithDetails("stage", "hip3_perp_dexs").
			WithDetails("type", "perpDexs").
			WithDetails("timeout_ms", r.hip3Timeout.Milliseconds())
	}

	return nil, wrapMetadataError(err, "hip3_perp_dexs", "perpDexs", "", r.hip3Timeout)
}

func wrapMetadataError(err error, stage, metaType, dex string, timeout time.Duration) error {
	var cliErr *output.CLIError
	if errors.As(err, &cliErr) {
		wrapped := output.NewCLIError(cliErr.Code, cliErr.Message).
			WithDetails("stage", stage).
			WithDetails("type", metaType).
			WithDetails("dex", dex).
			WithDetails("timeout_ms", timeout.Milliseconds())
		for k, v := range cliErr.Details {
			wrapped = wrapped.WithDetails(k, v)
		}
		return wrapped
	}

	return output.NewCLIError(output.ErrNetwork, "metadata request failed").
		WithDetails("stage", stage).
		WithDetails("type", metaType).
		WithDetails("dex", dex).
		WithDetails("timeout_ms", timeout.Milliseconds()).
		WithDetails("cause", err.Error())
}

func parseHIP3Dex(coin string) string {
	idx := strings.Index(coin, ":")
	if idx <= 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(coin[:idx]))
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
			AssetID:       i, // perp asset ID = array index
			Coin:          asset.Name,
			CanonicalCoin: asset.Name,
			SzDecimals:    asset.SzDecimals,
			IsSpot:        false,
		}
	}

	// Build lookup from token index → spotToken for resolving market references.
	tokenByIndex := make(map[int]spotToken, len(sm.Tokens))
	for _, tok := range sm.Tokens {
		tokenByIndex[tok.Index] = tok
	}

	spotMap := make(map[string]*AssetInfo, len(sm.Universe))
	spotPairAliases := make(map[string][]spotPairCandidate, len(sm.Universe))
	for _, market := range sm.Universe {
		if len(market.Tokens) == 0 {
			continue
		}
		// The first element is the base token index of the spot pair.
		baseIdx := market.Tokens[0]
		token, ok := tokenByIndex[baseIdx]
		if !ok {
			return output.NewCLIError(output.ErrAPI, "spot market references unknown token index").
				WithDetails("market", market.Name).
				WithDetails("tokenIndex", strconv.Itoa(baseIdx))
		}
		info := &AssetInfo{
			AssetID:       10000 + market.Index, // spot asset ID = 10000 + spot market index
			Coin:          token.Name,
			CanonicalCoin: market.Name,
			SzDecimals:    token.SzDecimals,
			IsSpot:        true,
		}
		// Allow resolution by both base token ("PURR") and full market name ("PURR/USDC").
		baseKey := strings.ToUpper(token.Name)
		spotMap[baseKey] = info
		marketKey := strings.ToUpper(market.Name)
		spotMap[marketKey] = info

		if len(market.Tokens) < 2 {
			continue
		}
		quoteIdx := market.Tokens[1]
		quoteToken, ok := tokenByIndex[quoteIdx]
		if !ok {
			return output.NewCLIError(output.ErrAPI, "spot market references unknown quote token index").
				WithDetails("market", market.Name).
				WithDetails("tokenIndex", strconv.Itoa(quoteIdx))
		}

		candidate := spotPairCandidate{
			info:        info,
			marketName:  market.Name,
			marketIndex: market.Index,
			canonical:   market.IsCanonical,
		}
		baseNames := append([]string{token.Name}, unitTokenAliases(token)...)
		quoteNames := append([]string{quoteToken.Name}, unitTokenAliases(quoteToken)...)

		seenAliasKeys := make(map[string]struct{}, len(baseNames)*len(quoteNames))
		for _, baseName := range baseNames {
			for _, quoteName := range quoteNames {
				key := strings.ToUpper(baseName + "/" + quoteName)
				if _, seen := seenAliasKeys[key]; seen {
					continue
				}
				seenAliasKeys[key] = struct{}{}
				spotPairAliases[key] = append(spotPairAliases[key], candidate)
			}
		}
	}

	r.perpMap = perpMap
	r.spotMap = spotMap
	r.spotPairAliases = spotPairAliases
	return nil
}

func unitTokenAliases(token spotToken) []string {
	fullName := strings.TrimSpace(token.FullName)
	if !strings.HasPrefix(strings.ToUpper(fullName), "UNIT ") {
		return nil
	}

	name := strings.ToUpper(token.Name)
	if len(name) < 2 || !strings.HasPrefix(name, "U") {
		return nil
	}

	aliases := make([]string, 0, len(name)-1)
	seen := make(map[string]struct{}, len(name)-1)
	for stripped := name; len(stripped) > 1 && strings.HasPrefix(stripped, "U"); {
		stripped = stripped[1:]
		if _, ok := seen[stripped]; ok {
			continue
		}
		seen[stripped] = struct{}{}
		aliases = append(aliases, stripped)
	}

	return aliases
}

func resolveSpotPairAlias(coin string, candidates []spotPairCandidate) (*AssetInfo, error) {
	if len(candidates) == 1 {
		return candidates[0].info, nil
	}

	var canonical spotPairCandidate
	canonicalCount := 0
	for _, candidate := range candidates {
		if candidate.canonical {
			canonical = candidate
			canonicalCount++
		}
	}
	if canonicalCount == 1 {
		return canonical.info, nil
	}

	candidateMarkets := make([]string, 0, len(candidates))
	candidateHints := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidateMarkets = append(candidateMarkets, candidate.marketName)
		candidateHints = append(candidateHints, candidate.marketName+" (market index "+strconv.Itoa(candidate.marketIndex)+")")
	}
	sort.Strings(candidateMarkets)
	sort.Strings(candidateHints)

	return nil, output.NewCLIError(output.ErrValidation, "ambiguous spot pair: "+coin).
		WithDetails("coin", coin).
		WithDetails("reason", "ambiguous_spot_pair").
		WithDetails("candidates", candidateMarkets).
		WithDetails("hint", "use an explicit spot market symbol from spot metadata (e.g. "+candidateHints[0]+")")
}

// fetchPerpDexOffsets returns dex name to asset offset mapping.
// Per the official Python SDK, builder-deployed dexes are offset by:
// 110000 + i*10000, where i is the position in perpDexs()[1:].
func (r *CachingResolver) fetchPerpDexOffsets(ctx context.Context) (map[string]int, error) {
	raw, err := r.client.PostInfo(ctx, map[string]string{"type": "perpDexs"})
	if err != nil {
		return nil, err
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}

	offsets := make(map[string]int)
	for i, entry := range entries {
		// Index 0 is reserved for the validator perp dex.
		if i == 0 {
			continue
		}
		if string(entry) == "null" {
			continue
		}

		var d perpDex
		if err := json.Unmarshal(entry, &d); err != nil {
			continue
		}
		if d.Name == "" {
			continue
		}

		offsets[strings.ToLower(d.Name)] = 110000 + (i-1)*10000
	}

	return offsets, nil
}
