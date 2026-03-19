package cmd

import (
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
	for _, match := range t.result.Matches {
		rows = append(rows, []string{
			strconv.Itoa(match.AssetID),
			match.Coin,
			match.MarketType,
			match.Dex,
			strconv.Itoa(match.SzDecimals),
			strings.Join(match.Aliases, ","),
			match.MatchType,
		})
	}
	return rows
}

type lookupAssetRecord struct {
	Coin       string
	AssetID    int
	MarketType string
	Dex        string
	SzDecimals int
	Aliases    []string
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
			dexExplicit := cmd.Flags().Changed("dex")

			dex := strings.ToLower(strings.TrimSpace(cfg.Dex))
			if dexExplicit && allDexes {
				return output.NewCLIError(output.ErrValidation, "--dex and --all-dexes are mutually exclusive")
			}
			if limit <= 0 {
				return output.NewCLIError(output.ErrValidation, "limit must be positive").
					WithDetails("value", limit)
			}

			scope := infoLookupScope{CorePerp: true, Spot: true, Dex: dex, AllDexes: allDexes}

			mode := "name"
			if _, err := strconv.Atoi(query); err == nil {
				mode = "id"
			}

			if cfg.DryRun {
				return printResult(cmd, cfg, mustMarshal(infoLookupDryRunPayload(query, mode, scope, limit)), nil)
			}

			records, err := fetchInfoLookupRecords(cmd, cfg, dex, allDexes)
			if err != nil {
				return err
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

func infoLookupDryRunPayload(query, mode string, scope infoLookupScope, limit int) map[string]any {
	requests := map[string]any{"core_perp_meta": info.MetaRequest{Type: "meta"}, "spot_meta": info.MetaRequest{Type: "spotMeta"}}
	if scope.Dex != "" || scope.AllDexes {
		requests["perp_dexs"] = info.PerpDexsRequest{Type: "perpDexs"}
	}
	if scope.Dex != "" {
		requests["hip3_meta"] = []info.MetaRequest{{Type: "meta", Dex: scope.Dex}}
	}
	if scope.AllDexes {
		requests["hip3_meta"] = []map[string]string{{"type": "meta", "dex": "<each dex from perpDexs>"}}
	}
	return map[string]any{"query": query, "mode": mode, "scope": scope, "limit": limit, "requests": requests}
}

func fetchInfoLookupRecords(cmd *cobra.Command, cfg *config.Config, dex string, allDexes bool) ([]lookupAssetRecord, error) {
	ic := buildInfoClient(cfg)
	records := make([]lookupAssetRecord, 0, 512)
	corePerps, err := fetchCorePerpLookupRecords(cmd.Context(), ic)
	if err != nil {
		return nil, err
	}
	records = append(records, corePerps...)

	spotRecords, err := fetchSpotLookupRecords(cmd.Context(), ic)
	if err != nil {
		return nil, err
	}
	records = append(records, spotRecords...)
	if allDexes {
		hip3Records, err := fetchAllHip3LookupRecords(cmd.Context(), ic)
		if err != nil {
			return nil, err
		}
		records = append(records, hip3Records...)
	} else if dex != "" {
		hip3Records, err := fetchSingleHip3LookupRecords(cmd.Context(), ic, dex)
		if err != nil {
			return nil, err
		}
		records = append(records, hip3Records...)
	}
	return records, nil
}
