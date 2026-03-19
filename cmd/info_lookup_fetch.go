package cmd

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

type lookupPerpMeta struct {
	Universe []lookupPerpAsset `json:"universe"`
}

type lookupPerpAsset struct {
	Name       string `json:"name"`
	SzDecimals int    `json:"szDecimals"`
}

type lookupSpotMeta struct {
	Universe []lookupSpotMarket `json:"universe"`
	Tokens   []lookupSpotToken  `json:"tokens"`
}

type lookupSpotMarket struct {
	Name   string `json:"name"`
	Index  int    `json:"index"`
	Tokens []int  `json:"tokens"`
}

type lookupSpotToken struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	SzDecimals int    `json:"szDecimals"`
	FullName   string `json:"fullName"`
}

type lookupDexName struct {
	Name string `json:"name"`
}

func fetchCorePerpLookupRecords(ctx context.Context, ic *info.InfoClient) ([]lookupAssetRecord, error) {
	raw, err := ic.Meta(ctx, "")
	if err != nil {
		return nil, err
	}

	var meta lookupPerpMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "failed to parse core perp metadata").
			WithDetails("cause", err.Error())
	}

	out := make([]lookupAssetRecord, 0, len(meta.Universe))
	for i, asset := range meta.Universe {
		out = append(out, lookupAssetRecord{
			Coin:       asset.Name,
			AssetID:    i,
			MarketType: "perp",
			SzDecimals: asset.SzDecimals,
			Aliases:    buildLookupAliases(asset.Name),
		})
	}
	return out, nil
}

func fetchSpotLookupRecords(ctx context.Context, ic *info.InfoClient) ([]lookupAssetRecord, error) {
	raw, err := ic.SpotMeta(ctx)
	if err != nil {
		return nil, err
	}

	var meta lookupSpotMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "failed to parse spot metadata").
			WithDetails("cause", err.Error())
	}

	tokenByIndex := make(map[int]lookupSpotToken, len(meta.Tokens))
	for _, tok := range meta.Tokens {
		tokenByIndex[tok.Index] = tok
	}

	out := make([]lookupAssetRecord, 0, len(meta.Universe))
	for _, market := range meta.Universe {
		szDecimals := 0
		aliases := buildLookupAliases(market.Name)

		if len(market.Tokens) > 0 {
			baseIdx := market.Tokens[0]
			if tok, ok := tokenByIndex[baseIdx]; ok {
				aliases = append(aliases, tok.Name)
				aliases = append(aliases, unitTokenAliases(tok)...)
				szDecimals = tok.SzDecimals
			}
		}

		out = append(out, lookupAssetRecord{
			Coin:       market.Name,
			AssetID:    10000 + market.Index,
			MarketType: "spot",
			SzDecimals: szDecimals,
			Aliases:    dedupeLookupAliases(aliases, market.Name),
		})
	}
	return out, nil
}

func fetchSingleHip3LookupRecords(ctx context.Context, ic *info.InfoClient, dex string) ([]lookupAssetRecord, error) {
	offsets, err := fetchHip3DexOffsets(ctx, ic)
	if err != nil {
		return nil, err
	}
	offset, ok := offsets[dex]
	if !ok {
		return nil, output.NewCLIError(output.ErrValidation, "unknown HIP-3 dex: "+dex).
			WithDetails("dex", dex)
	}

	return fetchHip3MetaRecords(ctx, ic, dex, offset)
}

func fetchAllHip3LookupRecords(ctx context.Context, ic *info.InfoClient) ([]lookupAssetRecord, error) {
	offsets, err := fetchHip3DexOffsets(ctx, ic)
	if err != nil {
		return nil, err
	}
	dexes := make([]string, 0, len(offsets))
	for dex := range offsets {
		dexes = append(dexes, dex)
	}
	sort.Strings(dexes)

	out := make([]lookupAssetRecord, 0, len(dexes)*16)
	for _, dex := range dexes {
		records, err := fetchHip3MetaRecords(ctx, ic, dex, offsets[dex])
		if err != nil {
			return nil, err
		}
		out = append(out, records...)
	}
	return out, nil
}

func fetchHip3MetaRecords(ctx context.Context, ic *info.InfoClient, dex string, offset int) ([]lookupAssetRecord, error) {
	raw, err := ic.Meta(ctx, dex)
	if err != nil {
		return nil, err
	}

	var meta lookupPerpMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "failed to parse HIP-3 metadata").
			WithDetails("dex", dex).
			WithDetails("cause", err.Error())
	}

	out := make([]lookupAssetRecord, 0, len(meta.Universe))
	for i, asset := range meta.Universe {
		out = append(out, lookupAssetRecord{
			Coin:       asset.Name,
			AssetID:    offset + i,
			MarketType: "perp",
			Dex:        dex,
			SzDecimals: asset.SzDecimals,
			Aliases:    buildLookupAliases(asset.Name),
		})
	}
	return out, nil
}

func fetchHip3DexOffsets(ctx context.Context, ic *info.InfoClient) (map[string]int, error) {
	raw, err := ic.PerpDexs(ctx)
	if err != nil {
		return nil, err
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "failed to parse perp dex list").
			WithDetails("cause", err.Error())
	}

	offsets := make(map[string]int)
	for i, entry := range entries {
		if i == 0 || strings.TrimSpace(string(entry)) == "null" {
			continue
		}

		var dex lookupDexName
		if err := json.Unmarshal(entry, &dex); err != nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(dex.Name))
		if name == "" {
			continue
		}
		offsets[name] = 110000 + (i-1)*10000
	}

	return offsets, nil
}
