package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	lookupCoreMetaJSON = `{"universe":[{"name":"BTC","szDecimals":3},{"name":"ETH","szDecimals":4}]}`
	lookupSpotMetaJSON = `{"tokens":[{"name":"USDC","index":7,"szDecimals":6},{"name":"BTC","index":42,"szDecimals":5}],"universe":[{"name":"BTC/USDC","index":3,"tokens":[42,7]}]}`
	lookupPerpDexsJSON = `[null,{"name":"tngs","index":1,"numMarkets":1}]`
)

func TestInfoLookup_DryRun_Default(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "lookup", "BTC", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if result["query"] != "BTC" {
		t.Fatalf("query = %v, want BTC", result["query"])
	}
	if result["mode"] != "name" {
		t.Fatalf("mode = %v, want name", result["mode"])
	}
	if result["limit"] != float64(lookupDefaultLimit) {
		t.Fatalf("limit = %v, want %d", result["limit"], lookupDefaultLimit)
	}

	requests, ok := result["requests"].(map[string]any)
	if !ok {
		t.Fatalf("requests = %T, want object", result["requests"])
	}
	if _, ok := requests["core_perp_meta"]; !ok {
		t.Fatal("missing core_perp_meta request")
	}
	if _, ok := requests["spot_meta"]; !ok {
		t.Fatal("missing spot_meta request")
	}
	if _, ok := requests["perp_dexs"]; ok {
		t.Fatal("did not expect perp_dexs in default scope")
	}
	if _, ok := requests["hip3_meta"]; ok {
		t.Fatal("did not expect hip3_meta in default scope")
	}
}

func TestInfoLookup_DryRun_WithDex(t *testing.T) {
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "lookup", "charizard", "--dry-run", "--dex", "tngs"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	scope, ok := result["scope"].(map[string]any)
	if !ok {
		t.Fatalf("scope = %T, want object", result["scope"])
	}
	if scope["dex"] != "tngs" {
		t.Fatalf("scope.dex = %v, want tngs", scope["dex"])
	}

	requests, ok := result["requests"].(map[string]any)
	if !ok {
		t.Fatalf("requests = %T, want object", result["requests"])
	}
	if _, ok := requests["perp_dexs"]; !ok {
		t.Fatal("expected perp_dexs request")
	}
	hip3Reqs, ok := requests["hip3_meta"].([]any)
	if !ok || len(hip3Reqs) != 1 {
		t.Fatalf("hip3_meta = %T(len=%d), want array len 1", requests["hip3_meta"], len(hip3Reqs))
	}
}

func TestInfoLookup_RejectsDexAndAllDexes(t *testing.T) {
	_, _, run := newTestRootWithServer(t, "")

	err := run("info", "lookup", "BTC", "--dex", "tngs", "--all-dexes")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v, want mutually exclusive validation", err)
	}
}

func TestInfoLookup_Runtime_NameMatchForConfusingHIP3Symbol(t *testing.T) {
	srv := newLookupAPIServer(t, lookupCoreMetaJSON, lookupSpotMetaJSON, lookupPerpDexsJSON, map[string]string{
		"tngs": `{"universe":[{"name":"tngs:CHARIZARD-TGUSD","szDecimals":0}]}`,
	})
	defer srv.Close()

	t.Setenv("HLGO_API_URL", srv.URL)
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "lookup", "charizardusd", "--dex", "tngs"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result infoLookupResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if result.Mode != "name" {
		t.Fatalf("mode = %q, want name", result.Mode)
	}
	if result.Count == 0 {
		t.Fatal("count = 0, want at least one match")
	}
	if len(result.Matches) == 0 {
		t.Fatal("matches empty, want at least one match")
	}

	match := result.Matches[0]
	if match.Coin != "tngs:CHARIZARD-TGUSD" {
		t.Fatalf("match.coin = %q, want tngs:CHARIZARD-TGUSD", match.Coin)
	}
	if match.AssetID != 110000 {
		t.Fatalf("match.asset_id = %d, want 110000", match.AssetID)
	}
	if match.MatchType != "exact" {
		t.Fatalf("match.match_type = %q, want exact", match.MatchType)
	}
	if !lookupHasAlias(match.Aliases, "CHARIZARDUSD") {
		t.Fatalf("aliases = %v, want CHARIZARDUSD", match.Aliases)
	}
}

func TestInfoLookup_Runtime_IDAndNoMatch(t *testing.T) {
	srv := newLookupAPIServer(t, lookupCoreMetaJSON, lookupSpotMetaJSON, "", nil)
	defer srv.Close()

	t.Setenv("HLGO_API_URL", srv.URL)
	stdout, _, run := newTestRootWithServer(t, "")

	if err := run("info", "lookup", "10003"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var idResult infoLookupResult
	if err := json.Unmarshal(stdout.Bytes(), &idResult); err != nil {
		t.Fatalf("failed to parse ID output: %v", err)
	}
	if idResult.Mode != "id" {
		t.Fatalf("mode = %q, want id", idResult.Mode)
	}
	if idResult.Count != 1 {
		t.Fatalf("count = %d, want 1", idResult.Count)
	}
	if len(idResult.Matches) != 1 {
		t.Fatalf("matches len = %d, want 1", len(idResult.Matches))
	}
	if idResult.Matches[0].Coin != "BTC/USDC" {
		t.Fatalf("coin = %q, want BTC/USDC", idResult.Matches[0].Coin)
	}
	if idResult.Matches[0].MarketType != "spot" {
		t.Fatalf("market_type = %q, want spot", idResult.Matches[0].MarketType)
	}

	if err := run("info", "lookup", "999999"); err != nil {
		t.Fatalf("unexpected error for no-match query: %v", err)
	}

	var noMatchResult infoLookupResult
	if err := json.Unmarshal(stdout.Bytes(), &noMatchResult); err != nil {
		t.Fatalf("failed to parse no-match output: %v", err)
	}
	if noMatchResult.Count != 0 {
		t.Fatalf("count = %d, want 0", noMatchResult.Count)
	}
	if noMatchResult.Matches == nil {
		t.Fatal("matches is nil, want empty array")
	}
	if len(noMatchResult.Matches) != 0 {
		t.Fatalf("matches len = %d, want 0", len(noMatchResult.Matches))
	}
}

func newLookupAPIServer(t *testing.T, coreMeta, spotMeta, perpDexs string, hip3Meta map[string]string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		typ, _ := req["type"].(string)
		switch typ {
		case "meta":
			dex, _ := req["dex"].(string)
			dex = strings.ToLower(strings.TrimSpace(dex))
			if dex == "" {
				_, _ = w.Write([]byte(coreMeta))
				return
			}
			if raw, ok := hip3Meta[dex]; ok {
				_, _ = w.Write([]byte(raw))
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown dex"}`))
		case "spotMeta":
			_, _ = w.Write([]byte(spotMeta))
		case "perpDexs":
			if perpDexs == "" {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(perpDexs))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown type"}`))
		}
	}))
}

func lookupHasAlias(aliases []string, want string) bool {
	for _, alias := range aliases {
		if strings.EqualFold(alias, want) {
			return true
		}
	}
	return false
}
