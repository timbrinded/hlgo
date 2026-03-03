package cmd

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/timbrinded/hlgo/pkg/config"
	"github.com/timbrinded/hlgo/pkg/info"
	"github.com/timbrinded/hlgo/pkg/output"
)

const lookupDefaultLimit = 20

type infoLookupScope struct {
	CorePerp bool   `json:"core_perp"`
	Spot     bool   `json:"spot"`
	Dex      string `json:"dex,omitempty"`
	AllDexes bool   `json:"all_dexes"`
}

type infoLookupMatch struct {
	Coin       string   `json:"coin"`
	AssetID    int      `json:"asset_id"`
	MarketType string   `json:"market_type"`
	Dex        string   `json:"dex,omitempty"`
	SzDecimals int      `json:"sz_decimals"`
	Aliases    []string `json:"aliases"`
	MatchType  string   `json:"match_type"`
}

type infoLookupResult struct {
	Query   string            `json:"query"`
	Mode    string            `json:"mode"`
	Scope   infoLookupScope   `json:"scope"`
	Limit   int               `json:"limit"`
	Count   int               `json:"count"`
	Matches []infoLookupMatch `json:"matches"`
}

type infoLookupTable struct {
	result *infoLookupResult
}

func (infoLookupTable) Headers() []string {
	return []string{"ASSET_ID", "COIN", "TYPE", "DEX", "SZ_DECIMALS", "ALIASES", "MATCH"}
}

func (t infoLookupTable) Rows() [][]string {
	if t.result == nil {
		return nil
	}
	rows := make([][]string, 0, len(t.result.Matches))
	for _, m := range t.result.Matches {
		rows = append(rows, []string{
			strconv.Itoa(m.AssetID),
			m.Coin,
			m.MarketType,
			m.Dex,
			strconv.Itoa(m.SzDecimals),
			strings.Join(m.Aliases, ","),
			m.MatchType,
		})
	}
	return rows
}

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

type lookupAssetRecord struct {
	Coin       string
	AssetID    int
	MarketType string
	Dex        string
	SzDecimals int
	Aliases    []string
}

type lookupScoredMatch struct {
	match infoLookupMatch
	rank  int
}

func newInfoLookupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lookup <query>",
		Short: "Lookup canonical asset identifiers by name fragment or asset ID",
		Long: `Lookup helps disambiguate coin names and numeric IDs across core perp, spot,
and optional HIP-3 dex scopes. Returns canonical --coin values and asset IDs.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			query := strings.TrimSpace(args[0])
			allDexes, _ := cmd.Flags().GetBool("all-dexes") //nolint:errcheck // known flag
			limit, _ := cmd.Flags().GetInt("limit")         //nolint:errcheck // known flag

			dex := strings.ToLower(strings.TrimSpace(cfg.Dex))

			if dex != "" && allDexes {
				return output.NewCLIError(output.ErrValidation, "--dex and --all-dexes are mutually exclusive")
			}
			if limit <= 0 {
				return output.NewCLIError(output.ErrValidation, "limit must be positive").
					WithDetails("value", limit)
			}

			scope := infoLookupScope{
				CorePerp: true,
				Spot:     true,
				Dex:      dex,
				AllDexes: allDexes,
			}

			mode := "name"
			if _, err := strconv.Atoi(query); err == nil {
				mode = "id"
			}

			if cfg.DryRun {
				requests := map[string]any{
					"core_perp_meta": info.MetaRequest{Type: "meta"},
					"spot_meta":      info.MetaRequest{Type: "spotMeta"},
				}
				if dex != "" || allDexes {
					requests["perp_dexs"] = info.PerpDexsRequest{Type: "perpDexs"}
				}
				if dex != "" {
					requests["hip3_meta"] = []info.MetaRequest{{Type: "meta", Dex: dex}}
				}
				if allDexes {
					requests["hip3_meta"] = []map[string]string{{"type": "meta", "dex": "<each dex from perpDexs>"}}
				}

				return printResult(cmd, cfg, mustMarshal(map[string]any{
					"query":    query,
					"mode":     mode,
					"scope":    scope,
					"limit":    limit,
					"requests": requests,
				}), nil)
			}

			ic := buildInfoClient(cfg)
			records := make([]lookupAssetRecord, 0, 512)

			corePerps, err := fetchCorePerpLookupRecords(cmd.Context(), ic)
			if err != nil {
				return err
			}
			records = append(records, corePerps...)

			spotRecords, err := fetchSpotLookupRecords(cmd.Context(), ic)
			if err != nil {
				return err
			}
			records = append(records, spotRecords...)

			if allDexes {
				hip3Records, err := fetchAllHip3LookupRecords(cmd.Context(), ic)
				if err != nil {
					return err
				}
				records = append(records, hip3Records...)
			} else if dex != "" {
				hip3Records, err := fetchSingleHip3LookupRecords(cmd.Context(), ic, dex)
				if err != nil {
					return err
				}
				records = append(records, hip3Records...)
			}

			matches := scoreLookupMatches(records, query, mode, limit)
			result := &infoLookupResult{
				Query:   query,
				Mode:    mode,
				Scope:   scope,
				Limit:   limit,
				Count:   len(matches),
				Matches: matches,
			}

			return printResult(cmd, cfg, mustMarshal(result), infoLookupTable{result: result})
		},
	}

	cmd.Flags().Bool("all-dexes", false, "include/search all HIP-3 dexes (slower)")
	cmd.Flags().Int("limit", lookupDefaultLimit, "maximum number of matches to return")
	return cmd
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

func buildLookupAliases(coin string) []string {
	trimmed := strings.TrimSpace(coin)
	if trimmed == "" {
		return []string{}
	}

	aliases := make([]string, 0, 8)
	candidate := trimmed
	if idx := strings.Index(candidate, ":"); idx > 0 && idx+1 < len(candidate) {
		suffix := strings.TrimSpace(candidate[idx+1:])
		if suffix != "" {
			aliases = append(aliases, suffix)
			candidate = suffix
		}
	}

	base, quote, ok := splitMarketSymbol(candidate)
	if ok {
		aliases = append(aliases,
			base,
			base+"/"+quote,
			base+"-"+quote,
			base+quote,
		)
		if strings.HasSuffix(strings.ToUpper(quote), "USD") {
			aliases = append(aliases, base+"USD")
		}
	}

	return dedupeLookupAliases(aliases, coin)
}

func splitMarketSymbol(s string) (base string, quote string, ok bool) {
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		base = strings.TrimSpace(parts[0])
		quote = strings.TrimSpace(parts[1])
		if base != "" && quote != "" {
			return base, quote, true
		}
	}

	if parts := strings.SplitN(s, "-", 2); len(parts) == 2 {
		base = strings.TrimSpace(parts[0])
		quote = strings.TrimSpace(parts[1])
		if base != "" && quote != "" {
			return base, quote, true
		}
	}

	return "", "", false
}

func unitTokenAliases(token lookupSpotToken) []string {
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

func dedupeLookupAliases(aliases []string, coin string) []string {
	seen := make(map[string]struct{}, len(aliases))
	out := make([]string, 0, len(aliases))
	coinLower := strings.ToLower(coin)
	for _, alias := range aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if key == coinLower {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func scoreLookupMatches(records []lookupAssetRecord, query, mode string, limit int) []infoLookupMatch {
	if len(records) == 0 {
		return []infoLookupMatch{}
	}

	var scored []lookupScoredMatch
	if mode == "id" {
		id, err := strconv.Atoi(query)
		if err != nil {
			return []infoLookupMatch{}
		}
		scored = scoreIDMatches(records, id)
	} else {
		scored = scoreNameMatches(records, query)
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].rank != scored[j].rank {
			return scored[i].rank < scored[j].rank
		}
		leftType := marketTypeSortRank(scored[i].match.MarketType)
		rightType := marketTypeSortRank(scored[j].match.MarketType)
		if leftType != rightType {
			return leftType < rightType
		}
		if scored[i].match.Dex != scored[j].match.Dex {
			return scored[i].match.Dex < scored[j].match.Dex
		}
		if scored[i].match.Coin != scored[j].match.Coin {
			return scored[i].match.Coin < scored[j].match.Coin
		}
		return scored[i].match.AssetID < scored[j].match.AssetID
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	out := make([]infoLookupMatch, 0, len(scored))
	for _, s := range scored {
		out = append(out, s.match)
	}
	return out
}

func marketTypeSortRank(marketType string) int {
	switch marketType {
	case "perp":
		return 0
	case "spot":
		return 1
	default:
		return 2
	}
}

func scoreIDMatches(records []lookupAssetRecord, id int) []lookupScoredMatch {
	out := make([]lookupScoredMatch, 0, 2)
	for _, r := range records {
		if r.AssetID != id {
			continue
		}
		out = append(out, lookupScoredMatch{
			match: infoLookupMatch{
				Coin:       r.Coin,
				AssetID:    r.AssetID,
				MarketType: r.MarketType,
				Dex:        r.Dex,
				SzDecimals: r.SzDecimals,
				Aliases:    r.Aliases,
				MatchType:  "id",
			},
			rank: 0,
		})
	}
	return out
}

func scoreNameMatches(records []lookupAssetRecord, query string) []lookupScoredMatch {
	q := strings.ToLower(strings.TrimSpace(query))
	qNorm := normalizeLookupToken(q)
	out := make([]lookupScoredMatch, 0, 16)

	for _, r := range records {
		rank := -1
		matchType := ""
		for _, token := range append([]string{r.Coin}, r.Aliases...) {
			candidate := strings.ToLower(strings.TrimSpace(token))
			if candidate == "" {
				continue
			}
			candidateNorm := normalizeLookupToken(candidate)

			score, label := scoreLookupToken(candidate, candidateNorm, q, qNorm)
			if score == -1 {
				continue
			}
			if rank == -1 || score < rank {
				rank = score
				matchType = label
			}
			if rank == 0 {
				break
			}
		}

		if rank == -1 {
			continue
		}

		out = append(out, lookupScoredMatch{
			match: infoLookupMatch{
				Coin:       r.Coin,
				AssetID:    r.AssetID,
				MarketType: r.MarketType,
				Dex:        r.Dex,
				SzDecimals: r.SzDecimals,
				Aliases:    r.Aliases,
				MatchType:  matchType,
			},
			rank: rank,
		})
	}

	return out
}

func scoreLookupToken(candidate, candidateNorm, query, queryNorm string) (int, string) {
	switch {
	case candidate == query:
		return 0, "exact"
	case queryNorm != "" && candidateNorm == queryNorm:
		return 1, "normalized_exact"
	case strings.HasPrefix(candidate, query):
		return 2, "prefix"
	case queryNorm != "" && strings.HasPrefix(candidateNorm, queryNorm):
		return 3, "normalized_prefix"
	case strings.Contains(candidate, query):
		return 4, "contains"
	case queryNorm != "" && strings.Contains(candidateNorm, queryNorm):
		return 5, "normalized_contains"
	case len(queryNorm) >= 3 && isSubsequence(queryNorm, candidateNorm):
		return 6, "subsequence"
	default:
		return -1, ""
	}
}

func normalizeLookupToken(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

func isSubsequence(needle, haystack string) bool {
	if needle == "" {
		return true
	}
	j := 0
	for i := 0; i < len(haystack) && j < len(needle); i++ {
		if haystack[i] == needle[j] {
			j++
		}
	}
	return j == len(needle)
}
