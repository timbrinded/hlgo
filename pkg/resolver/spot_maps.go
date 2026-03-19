package resolver

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/timbrinded/hlgo/pkg/output"
)

type spotPairCandidate struct {
	info        *AssetInfo
	marketName  string
	marketIndex int
	canonical   bool
}

type perpDex struct {
	Name string `json:"name"`
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

// buildMaps parses perp and spot metadata and populates the lookup maps.
func (r *CachingResolver) buildMaps(perpData, spotData []byte) error {
	var perpMetadata perpMeta
	if err := json.Unmarshal(perpData, &perpMetadata); err != nil {
		return output.NewCLIError(output.ErrAPI, "failed to parse perp metadata").
			WithDetails("cause", err.Error())
	}

	var spotMetadata spotMeta
	if err := json.Unmarshal(spotData, &spotMetadata); err != nil {
		return output.NewCLIError(output.ErrAPI, "failed to parse spot metadata").
			WithDetails("cause", err.Error())
	}

	perpMap := make(map[string]*AssetInfo, len(perpMetadata.Universe))
	for index, asset := range perpMetadata.Universe {
		upper := strings.ToUpper(asset.Name)
		perpMap[upper] = &AssetInfo{
			AssetID:       index,
			Coin:          asset.Name,
			CanonicalCoin: asset.Name,
			SzDecimals:    asset.SzDecimals,
			IsSpot:        false,
		}
	}

	// Build lookup from token index → spotToken for resolving market references.
	tokenByIndex := make(map[int]spotToken, len(spotMetadata.Tokens))
	for _, token := range spotMetadata.Tokens {
		tokenByIndex[token.Index] = token
	}

	spotMap, spotPairAliases, err := buildSpotMaps(spotMetadata.Universe, tokenByIndex)
	if err != nil {
		return err
	}

	r.perpMap = perpMap
	r.spotMap = spotMap
	r.spotPairAliases = spotPairAliases
	return nil
}

func buildSpotMaps(markets []spotMarket, tokenByIndex map[int]spotToken) (map[string]*AssetInfo, map[string][]spotPairCandidate, error) {
	spotMap := make(map[string]*AssetInfo, len(markets))
	spotPairAliases := make(map[string][]spotPairCandidate, len(markets))
	for _, market := range markets {
		if len(market.Tokens) == 0 {
			continue
		}
		baseIdx := market.Tokens[0]
		token, ok := tokenByIndex[baseIdx]
		if !ok {
			return nil, nil, output.NewCLIError(output.ErrAPI, "spot market references unknown token index").
				WithDetails("market", market.Name).
				WithDetails("tokenIndex", strconv.Itoa(baseIdx))
		}

		info := &AssetInfo{AssetID: 10000 + market.Index, Coin: token.Name, CanonicalCoin: market.Name, SzDecimals: token.SzDecimals, IsSpot: true}
		spotMap[strings.ToUpper(token.Name)] = info
		spotMap[strings.ToUpper(market.Name)] = info
		if len(market.Tokens) < 2 {
			continue
		}

		quoteIdx := market.Tokens[1]
		quoteToken, ok := tokenByIndex[quoteIdx]
		if !ok {
			return nil, nil, output.NewCLIError(output.ErrAPI, "spot market references unknown quote token index").
				WithDetails("market", market.Name).
				WithDetails("tokenIndex", strconv.Itoa(quoteIdx))
		}

		candidate := spotPairCandidate{info: info, marketName: market.Name, marketIndex: market.Index, canonical: market.IsCanonical}
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
	return spotMap, spotPairAliases, nil
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

		var dex perpDex
		if err := json.Unmarshal(entry, &dex); err != nil {
			continue
		}
		if dex.Name == "" {
			continue
		}

		offsets[strings.ToLower(dex.Name)] = 110000 + (i-1)*10000
	}

	return offsets, nil
}
