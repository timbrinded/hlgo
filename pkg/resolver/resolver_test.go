package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/timbrinded/hlgo/pkg/client"
	"github.com/timbrinded/hlgo/pkg/output"
)

// --- Test fixtures ---

// testPerpMeta returns a realistic perp meta response.
// Array indices are the asset IDs: BTC=0, ETH=1, SOL=2.
func testPerpMeta() string {
	return `{
		"universe": [
			{"name": "BTC", "szDecimals": 5},
			{"name": "ETH", "szDecimals": 4},
			{"name": "SOL", "szDecimals": 2}
		]
	}`
}

// testSpotMeta returns a realistic spot meta response matching the real API shape.
// Top-level "tokens" is the registry; universe[].tokens are integer indices into it.
// The spot market index determines the asset ID: 10000 + market index.
func testSpotMeta() string {
	return `{
		"tokens": [
			{"name": "USDC", "index": 0, "szDecimals": 6},
			{"name": "PURR", "index": 1, "szDecimals": 0},
			{"name": "HFUN", "index": 2, "szDecimals": 3}
		],
		"universe": [
			{"name": "PURR/USDC", "index": 0, "tokens": [1, 0]},
			{"name": "HFUN/USDC", "index": 1, "tokens": [2, 0]}
		]
	}`
}

// newTestMetaServer creates a test server that returns perp and spot metadata.
func newTestMetaServer(perpResp, spotResp string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		switch req["type"] {
		case "meta":
			fmt.Fprint(w, perpResp)
		case "spotMeta":
			fmt.Fprint(w, spotResp)
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"unknown type: %s"}`, req["type"])
		}
	}))
}

// newTestResolver creates a CachingResolver backed by a test server with
// the standard test fixtures and a temp directory for cache.
func newTestResolver(t *testing.T) (*CachingResolver, *httptest.Server) {
	t.Helper()
	srv := newTestMetaServer(testPerpMeta(), testSpotMeta())
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	c := client.NewClient(srv.URL)
	r := NewResolver(c, cacheDir, 5*time.Minute)
	return r, srv
}

// newPreloadedResolver creates a resolver with pre-populated disk cache
// so tests can run without making any API calls.
func newPreloadedResolver(t *testing.T) *CachingResolver {
	t.Helper()
	cacheDir := t.TempDir()

	now := time.Now()
	dc := newDiskCache(cacheDir)
	dc.write("meta.json", []byte(testPerpMeta()), now)
	dc.write("spot_meta.json", []byte(testSpotMeta()), now)

	// Client points to a server that will fail if called.
	c := client.NewClient("http://127.0.0.1:1") // unreachable
	r := NewResolver(c, cacheDir, 5*time.Minute)
	return r
}

// --- Tests ---

func TestResolveKnownPerpCoin(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "ETH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 1 {
		t.Errorf("AssetID = %d, want 1", info.AssetID)
	}
	if info.Coin != "ETH" {
		t.Errorf("Coin = %q, want %q", info.Coin, "ETH")
	}
	if info.CanonicalCoin != "ETH" {
		t.Errorf("CanonicalCoin = %q, want %q", info.CanonicalCoin, "ETH")
	}
	if info.SzDecimals != 4 {
		t.Errorf("SzDecimals = %d, want 4", info.SzDecimals)
	}
	if info.IsSpot {
		t.Error("IsSpot = true, want false")
	}
	if info.Passthrough {
		t.Error("Passthrough = true, want false for named coin")
	}
}

func TestResolveKnownPerpCoin_BTC(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 0 {
		t.Errorf("AssetID = %d, want 0 (first in universe)", info.AssetID)
	}
	if info.SzDecimals != 5 {
		t.Errorf("SzDecimals = %d, want 5", info.SzDecimals)
	}
}

func TestResolveKnownPerpCoin_SOL(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "SOL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 2 {
		t.Errorf("AssetID = %d, want 2 (third in universe)", info.AssetID)
	}
	if info.SzDecimals != 2 {
		t.Errorf("SzDecimals = %d, want 2", info.SzDecimals)
	}
}

func TestResolveKnownSpotCoin(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "PURR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10000 {
		t.Errorf("AssetID = %d, want 10000 (10000 + market index 0)", info.AssetID)
	}
	if info.Coin != "PURR" {
		t.Errorf("Coin = %q, want %q", info.Coin, "PURR")
	}
	if info.CanonicalCoin != "PURR/USDC" {
		t.Errorf("CanonicalCoin = %q, want %q", info.CanonicalCoin, "PURR/USDC")
	}
	if info.SzDecimals != 0 {
		t.Errorf("SzDecimals = %d, want 0", info.SzDecimals)
	}
	if !info.IsSpot {
		t.Error("IsSpot = false, want true")
	}
}

func TestResolveSpotCoin_HFUN(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "HFUN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10001 {
		t.Errorf("AssetID = %d, want 10001 (10000 + market index 1)", info.AssetID)
	}
	if info.SzDecimals != 3 {
		t.Errorf("SzDecimals = %d, want 3", info.SzDecimals)
	}
	if !info.IsSpot {
		t.Error("IsSpot = false, want true")
	}
}

func TestCaseInsensitiveMatching(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	variants := []string{"eth", "Eth", "eTH", "ETH", "eTh"}
	for _, v := range variants {
		info, err := r.ResolveAsset(ctx, v)
		if err != nil {
			t.Errorf("ResolveAsset(%q) error: %v", v, err)
			continue
		}
		if info.AssetID != 1 {
			t.Errorf("ResolveAsset(%q) AssetID = %d, want 1", v, info.AssetID)
		}
	}
}

func TestCaseInsensitiveSpotMatching(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "purr")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10000 {
		t.Errorf("AssetID = %d, want 10000", info.AssetID)
	}
	if !info.IsSpot {
		t.Error("IsSpot = false, want true")
	}
}

func TestNumericPassthrough(t *testing.T) {
	// No server needed — numeric passthrough skips API entirely.
	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 1 {
		t.Errorf("AssetID = %d, want 1", info.AssetID)
	}
	if info.Coin != "1" {
		t.Errorf("Coin = %q, want %q", info.Coin, "1")
	}
	if info.CanonicalCoin != "1" {
		t.Errorf("CanonicalCoin = %q, want %q", info.CanonicalCoin, "1")
	}
	if info.SzDecimals != 0 {
		t.Errorf("SzDecimals = %d, want 0", info.SzDecimals)
	}
	if info.IsSpot {
		t.Error("IsSpot = true, want false for numeric passthrough")
	}
	if !info.Passthrough {
		t.Error("Passthrough = false, want true for numeric passthrough")
	}
}

func TestNumericPassthrough_Zero(t *testing.T) {
	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 0 {
		t.Errorf("AssetID = %d, want 0", info.AssetID)
	}
}

func TestNumericPassthrough_LargeID(t *testing.T) {
	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "10005")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10005 {
		t.Errorf("AssetID = %d, want 10005", info.AssetID)
	}
}

func TestNumericPassthrough_NegativeID_Rejected(t *testing.T) {
	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "-1")
	if err == nil {
		t.Fatal("expected error for negative asset ID")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrValidation)
	}
}

func TestNumericPassthrough_WhitespaceTrimmed(t *testing.T) {
	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, " 42 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 42 {
		t.Errorf("AssetID = %d, want 42", info.AssetID)
	}
	if info.Coin != "42" {
		t.Errorf("Coin = %q, want %q (trimmed)", info.Coin, "42")
	}
}

func TestResolveSpotByMarketName(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	// Resolve by market name "PURR/USDC" should return same info as "PURR".
	info, err := r.ResolveAsset(ctx, "PURR/USDC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10000 {
		t.Errorf("AssetID = %d, want 10000", info.AssetID)
	}
	if !info.IsSpot {
		t.Error("IsSpot = false, want true")
	}

	// Case-insensitive market name.
	info2, err := r.ResolveAsset(ctx, "hfun/usdc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info2.AssetID != 10001 {
		t.Errorf("AssetID = %d, want 10001", info2.AssetID)
	}
}

func TestResolveSpotByFriendlyPairAlias_FromTokenNames(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	dc.write("meta.json", []byte(`{"universe":[{"name":"BTC","szDecimals":5}]}`), now)
	dc.write("spot_meta.json", []byte(`{
		"tokens":[
			{"name":"USDC","szDecimals":6,"index":0},
			{"name":"UETH","szDecimals":4,"index":221,"fullName":"Unit Ethereum"}
		],
		"universe":[
			{"name":"@151","index":151,"tokens":[221,0],"isCanonical":true}
		]
	}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "UETH/USDC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10151 {
		t.Errorf("AssetID = %d, want 10151", info.AssetID)
	}
	if !info.IsSpot {
		t.Error("IsSpot = false, want true")
	}

	// Friendly alias strips the unit prefix.
	info, err = r.ResolveAsset(ctx, "ETH/USDC")
	if err != nil {
		t.Fatalf("unexpected error for ETH/USDC: %v", err)
	}
	if info.AssetID != 10151 {
		t.Errorf("AssetID = %d, want 10151", info.AssetID)
	}
	if !info.IsSpot {
		t.Error("IsSpot = false, want true")
	}
}

func TestResolveSpotByFriendlyPairAlias_MultipleUnitPrefixes(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	dc.write("meta.json", []byte(`{"universe":[{"name":"BTC","szDecimals":5}]}`), now)
	dc.write("spot_meta.json", []byte(`{
		"tokens":[
			{"name":"USDC","szDecimals":6,"index":0},
			{"name":"UUVIRT","szDecimals":4,"index":10,"fullName":"Unit Virtual"}
		],
		"universe":[
			{"name":"@1","index":1,"tokens":[10,0],"isCanonical":true}
		]
	}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	for _, pair := range []string{"UUVIRT/USDC", "UVIRT/USDC", "VIRT/USDC"} {
		info, err := r.ResolveAsset(ctx, pair)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", pair, err)
		}
		if info.AssetID != 10001 {
			t.Fatalf("%s asset_id = %d, want 10001", pair, info.AssetID)
		}
	}
}

func TestResolveSpotByFriendlyPairAlias_NonUnitTokenDoesNotStripPrefix(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	dc.write("meta.json", []byte(`{"universe":[{"name":"BTC","szDecimals":5}]}`), now)
	dc.write("spot_meta.json", []byte(`{
		"tokens":[
			{"name":"USDC","szDecimals":6,"index":0},
			{"name":"UTEST","szDecimals":4,"index":11,"fullName":"Utility Test"}
		],
		"universe":[
			{"name":"@2","index":2,"tokens":[11,0],"isCanonical":true}
		]
	}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "TEST/USDC")
	if err == nil {
		t.Fatal("expected TEST/USDC to fail: token is not marked as Unit")
	}
}

func TestResolveSpotFriendlyPairAlias_PrefersUniqueCanonical(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	dc.write("meta.json", []byte(`{"universe":[{"name":"BTC","szDecimals":5}]}`), now)
	dc.write("spot_meta.json", []byte(`{
		"tokens":[
			{"name":"USDC","szDecimals":6,"index":0},
			{"name":"UETH","szDecimals":4,"index":221}
		],
		"universe":[
			{"name":"@151","index":151,"tokens":[221,0],"isCanonical":false},
			{"name":"@235","index":235,"tokens":[221,0],"isCanonical":true}
		]
	}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "UETH/USDC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10235 {
		t.Errorf("AssetID = %d, want 10235 (canonical market index 235)", info.AssetID)
	}
}

func TestResolveSpotFriendlyPairAlias_AmbiguousReturnsCandidates(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	dc.write("meta.json", []byte(`{"universe":[{"name":"BTC","szDecimals":5}]}`), now)
	dc.write("spot_meta.json", []byte(`{
		"tokens":[
			{"name":"USDC","szDecimals":6,"index":0},
			{"name":"UETH","szDecimals":4,"index":221}
		],
		"universe":[
			{"name":"@151","index":151,"tokens":[221,0],"isCanonical":false},
			{"name":"@235","index":235,"tokens":[221,0],"isCanonical":false}
		]
	}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "UETH/USDC")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrValidation)
	}
	if cliErr.Details["reason"] != "ambiguous_spot_pair" {
		t.Errorf("details[reason] = %v, want ambiguous_spot_pair", cliErr.Details["reason"])
	}

	candidatesRaw, ok := cliErr.Details["candidates"].([]string)
	if !ok {
		t.Fatalf("details[candidates] type = %T, want []string", cliErr.Details["candidates"])
	}
	if len(candidatesRaw) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidatesRaw))
	}
	if !containsString(candidatesRaw, "@151") || !containsString(candidatesRaw, "@235") {
		t.Errorf("candidates = %v, want @151 and @235", candidatesRaw)
	}

	hint, ok := cliErr.Details["hint"].(string)
	if !ok {
		t.Fatalf("details[hint] type = %T, want string", cliErr.Details["hint"])
	}
	if !strings.Contains(hint, "market index") {
		t.Errorf("hint = %q, want market index guidance", hint)
	}
}

func TestUnknownCoin_ReturnsValidationError(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "FAKECOIN")
	if err == nil {
		t.Fatal("expected error for unknown coin")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrValidation)
	}
	if cliErr.Details["coin"] != "FAKECOIN" {
		t.Errorf("details[coin] = %v, want %q", cliErr.Details["coin"], "FAKECOIN")
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestUnitTokenAliases(t *testing.T) {
	tests := []struct {
		name  string
		token spotToken
		want  []string
	}{
		{
			name:  "single unit prefix",
			token: spotToken{Name: "UETH", FullName: "Unit Ethereum"},
			want:  []string{"ETH"},
		},
		{
			name:  "multiple unit prefixes",
			token: spotToken{Name: "UUENA", FullName: "Unit Ethena"},
			want:  []string{"UENA", "ENA"},
		},
		{
			name:  "unit token without U prefix",
			token: spotToken{Name: "AURA", FullName: "Unit AURA"},
			want:  nil,
		},
		{
			name:  "non unit token with U prefix",
			token: spotToken{Name: "USDC", FullName: "USD Coin"},
			want:  nil,
		},
		{
			name:  "missing full name",
			token: spotToken{Name: "UETH"},
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unitTokenAliases(tc.token)
			if len(got) != len(tc.want) {
				t.Fatalf("len(aliases) = %d, want %d (%v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("aliases[%d] = %q, want %q (%v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

func TestMultipleCoinsFromSameCache(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	coins := []struct {
		name    string
		assetID int
		isSpot  bool
	}{
		{"BTC", 0, false},
		{"ETH", 1, false},
		{"SOL", 2, false},
		{"PURR", 10000, true},
		{"HFUN", 10001, true},
	}

	for _, tc := range coins {
		info, err := r.ResolveAsset(ctx, tc.name)
		if err != nil {
			t.Errorf("ResolveAsset(%q) error: %v", tc.name, err)
			continue
		}
		if info.AssetID != tc.assetID {
			t.Errorf("ResolveAsset(%q) AssetID = %d, want %d", tc.name, info.AssetID, tc.assetID)
		}
		if info.IsSpot != tc.isSpot {
			t.Errorf("ResolveAsset(%q) IsSpot = %v, want %v", tc.name, info.IsSpot, tc.isSpot)
		}
	}
}

func TestPerpAssetIDFormula_IsArrayIndex(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	// Verify each perp gets its array index as asset ID.
	expected := map[string]int{"BTC": 0, "ETH": 1, "SOL": 2}
	for coin, wantID := range expected {
		info, err := r.ResolveAsset(ctx, coin)
		if err != nil {
			t.Errorf("ResolveAsset(%q) error: %v", coin, err)
			continue
		}
		if info.AssetID != wantID {
			t.Errorf("%s AssetID = %d, want %d (array index)", coin, info.AssetID, wantID)
		}
	}
}

func TestResolveHIP3PerpCoin_WithDexOffset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"bad request"}`)
			return
		}

		reqType, _ := req["type"].(string)
		dex, _ := req["dex"].(string)

		w.Header().Set("Content-Type", "application/json")
		switch {
		case reqType == "meta" && dex == "":
			fmt.Fprint(w, `{"universe":[{"name":"BTC","szDecimals":5}]}`)
		case reqType == "spotMeta":
			fmt.Fprint(w, `{"tokens":[{"name":"USDC","index":0,"szDecimals":6}],"universe":[{"name":"@0","index":0,"tokens":[0,0]}]}`)
		case reqType == "perpDexs":
			fmt.Fprint(w, `[null,{"name":"xyz"}]`)
		case reqType == "meta" && dex == "xyz":
			fmt.Fprint(w, `{"universe":[{"name":"xyz:XYZ100","szDecimals":4},{"name":"xyz:TSLA","szDecimals":3}]}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"unexpected request type=%s dex=%s"}`, reqType, dex)
		}
	}))
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "xyz:TSLA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// HIP-3 offset for first builder dex is 110000 + index.
	if info.AssetID != 110001 {
		t.Errorf("AssetID = %d, want 110001", info.AssetID)
	}
	if info.SzDecimals != 3 {
		t.Errorf("SzDecimals = %d, want 3", info.SzDecimals)
	}
	if info.IsSpot {
		t.Error("IsSpot = true, want false")
	}
}

func TestResolveHIP3PerpCoin_LoadsRequestedDexOnly(t *testing.T) {
	var abcMetaCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"bad request"}`)
			return
		}

		reqType, _ := req["type"].(string)
		dex, _ := req["dex"].(string)

		w.Header().Set("Content-Type", "application/json")
		switch {
		case reqType == "meta" && dex == "":
			fmt.Fprint(w, `{"universe":[{"name":"BTC","szDecimals":5}]}`)
		case reqType == "spotMeta":
			fmt.Fprint(w, `{"tokens":[{"name":"USDC","index":0,"szDecimals":6}],"universe":[{"name":"@0","index":0,"tokens":[0,0]}]}`)
		case reqType == "perpDexs":
			fmt.Fprint(w, `[null,{"name":"abc"},{"name":"xyz"}]`)
		case reqType == "meta" && dex == "abc":
			atomic.AddInt32(&abcMetaCalls, 1)
			fmt.Fprint(w, `{"universe":[{"name":"abc:ONE","szDecimals":3}]}`)
		case reqType == "meta" && dex == "xyz":
			fmt.Fprint(w, `{"universe":[{"name":"xyz:TSLA","szDecimals":3}]}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"unexpected request type=%s dex=%s"}`, reqType, dex)
		}
	}))
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "xyz:TSLA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// xyz is in position 2 => 110000 + (2-1)*10000 + 0 = 120000.
	if info.AssetID != 120000 {
		t.Fatalf("asset_id = %d, want 120000", info.AssetID)
	}
	if calls := atomic.LoadInt32(&abcMetaCalls); calls != 0 {
		t.Fatalf("abc dex metadata should not be fetched, got %d calls", calls)
	}
}

func TestResolveAsset_NonHIP3CoinSkipsPerpDexs(t *testing.T) {
	var perpDexCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		reqType, _ := req["type"].(string)
		w.Header().Set("Content-Type", "application/json")
		switch reqType {
		case "meta":
			fmt.Fprint(w, testPerpMeta())
		case "spotMeta":
			fmt.Fprint(w, testSpotMeta())
		case "perpDexs":
			atomic.AddInt32(&perpDexCalls, 1)
			fmt.Fprint(w, `[null,{"name":"xyz"}]`)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "ETH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 1 {
		t.Fatalf("asset_id = %d, want 1", info.AssetID)
	}
	if calls := atomic.LoadInt32(&perpDexCalls); calls != 0 {
		t.Fatalf("perpDexs should not be fetched for non-HIP3 coins, got %d calls", calls)
	}
}

func TestResolveAsset_CoreMetadataTimeoutFailsFast(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		reqType, _ := req["type"].(string)
		w.Header().Set("Content-Type", "application/json")
		switch reqType {
		case "meta":
			time.Sleep(250 * time.Millisecond)
			fmt.Fprint(w, testPerpMeta())
		case "spotMeta":
			fmt.Fprint(w, testSpotMeta())
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	r.coreTimeout = 50 * time.Millisecond
	ctx := context.Background()

	start := time.Now()
	_, err := r.ResolveAsset(ctx, "ETH")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("resolver timed out too slowly: %s", elapsed)
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrNetwork {
		t.Fatalf("error code = %s, want %s", cliErr.Code, output.ErrNetwork)
	}
	if cliErr.Details["stage"] != "perp_meta" {
		t.Fatalf("details[stage] = %v, want perp_meta", cliErr.Details["stage"])
	}
}

func TestResolveAsset_HIP3TimeoutIsExplicit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		reqType, _ := req["type"].(string)
		w.Header().Set("Content-Type", "application/json")
		switch reqType {
		case "meta":
			fmt.Fprint(w, testPerpMeta())
		case "spotMeta":
			fmt.Fprint(w, testSpotMeta())
		case "perpDexs":
			time.Sleep(250 * time.Millisecond)
			fmt.Fprint(w, `[null,{"name":"xyz"}]`)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	r.hip3Timeout = 50 * time.Millisecond
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "xyz:TSLA")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrNetwork {
		t.Fatalf("error code = %s, want %s", cliErr.Code, output.ErrNetwork)
	}
	if cliErr.Details["stage"] != "hip3_perp_dexs" {
		t.Fatalf("details[stage] = %v, want hip3_perp_dexs", cliErr.Details["stage"])
	}
}

func TestSpotAssetIDFormula_10000PlusIndex(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	// PURR market has index=0, HFUN market has index=1.
	expected := map[string]int{"PURR": 10000, "HFUN": 10001}
	for coin, wantID := range expected {
		info, err := r.ResolveAsset(ctx, coin)
		if err != nil {
			t.Errorf("ResolveAsset(%q) error: %v", coin, err)
			continue
		}
		if info.AssetID != wantID {
			t.Errorf("%s AssetID = %d, want %d (10000 + spot market index)", coin, info.AssetID, wantID)
		}
	}
}

func TestResolverWithEmptyMetadata(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)
	dc.write("meta.json", []byte(`{"universe":[]}`), now)
	dc.write("spot_meta.json", []byte(`{"universe":[]}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "BTC")
	if err == nil {
		t.Fatal("expected error when metadata is empty")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrValidation)
	}
}

func TestAPIErrorPropagated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"bad request"}`)
	}))
	defer srv.Close()

	c := client.NewClient(srv.URL, client.WithRetries(0))
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "BTC")
	if err == nil {
		t.Fatal("expected error from API")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrAPI {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrAPI)
	}
}

func TestCacheExpiry(t *testing.T) {
	cacheDir := t.TempDir()
	past := time.Now().Add(-10 * time.Minute)
	dc := newDiskCache(cacheDir)
	dc.write("meta.json", []byte(testPerpMeta()), past)
	dc.write("spot_meta.json", []byte(testSpotMeta()), past)

	// The expired cache should trigger an API fetch. Provide a working server.
	srv := newTestMetaServer(testPerpMeta(), testSpotMeta())
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "ETH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 1 {
		t.Errorf("AssetID = %d, want 1", info.AssetID)
	}
}

func TestCacheFreshHit_NoAPICalls(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)
	dc.write("meta.json", []byte(testPerpMeta()), now)
	dc.write("spot_meta.json", []byte(testSpotMeta()), now)

	// Server that panics if called — proves cache is serving.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("API should not be called when cache is fresh")
	}))
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 0 {
		t.Errorf("AssetID = %d, want 0", info.AssetID)
	}
}

func TestFetchFromAPI_WhenNoCacheExists(t *testing.T) {
	r, _ := newTestResolver(t)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "SOL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 2 {
		t.Errorf("AssetID = %d, want 2", info.AssetID)
	}
}

func TestCacheWrittenAfterAPIFetch(t *testing.T) {
	r, _ := newTestResolver(t)
	ctx := context.Background()

	// First call fetches from API and writes cache.
	_, err := r.ResolveAsset(ctx, "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache files were created.
	metaPath := filepath.Join(r.cache.dir, "meta.json")
	spotPath := filepath.Join(r.cache.dir, "spot_meta.json")

	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("meta.json cache file not created")
	}
	if _, err := os.Stat(spotPath); os.IsNotExist(err) {
		t.Error("spot_meta.json cache file not created")
	}
}

func TestPerpPriorityOverSpot(t *testing.T) {
	// If a coin name appears in both perp and spot, perp wins.
	// This tests the lookup order in ResolveAsset.
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	// BTC exists in perps with asset ID 0.
	dc.write("meta.json", []byte(`{"universe":[{"name":"BTC","szDecimals":5}]}`), now)
	// BTC also appears as a spot token.
	dc.write("spot_meta.json", []byte(`{
		"tokens":[{"name":"USDC","index":0,"szDecimals":6},{"name":"BTC","index":99,"szDecimals":8}],
		"universe":[{"name":"BTC/USDC","index":0,"tokens":[99,0]}]
	}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Perp should win.
	if info.AssetID != 0 {
		t.Errorf("AssetID = %d, want 0 (perp should take priority over spot)", info.AssetID)
	}
	if info.IsSpot {
		t.Error("IsSpot = true, want false (perp priority)")
	}
}

func TestResolverImplementsInterface(t *testing.T) {
	// Compile-time check that CachingResolver implements Resolver.
	var _ Resolver = (*CachingResolver)(nil)
}

func TestCorruptCacheFallsBackToAPI(t *testing.T) {
	cacheDir := t.TempDir()

	// Write corrupt cache files.
	metaPath := filepath.Join(cacheDir, "meta.json")
	spotPath := filepath.Join(cacheDir, "spot_meta.json")
	os.MkdirAll(cacheDir, 0o700)
	os.WriteFile(metaPath, []byte(`not valid json`), 0o600)
	os.WriteFile(spotPath, []byte(`not valid json`), 0o600)

	srv := newTestMetaServer(testPerpMeta(), testSpotMeta())
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "ETH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 1 {
		t.Errorf("AssetID = %d, want 1", info.AssetID)
	}
}

func TestDiskCache_ReadWrite(t *testing.T) {
	dir := t.TempDir()
	dc := newDiskCache(dir)
	now := time.Now()

	data := []byte(`{"universe":[{"name":"BTC","szDecimals":5}]}`)
	dc.write("meta.json", data, now)

	got, fresh := dc.read("meta.json", 5*time.Minute, now)
	if !fresh {
		t.Fatal("expected cache to be fresh")
	}
	if string(got) != string(data) {
		t.Errorf("cache data = %s, want %s", got, data)
	}
}

func TestDiskCache_Expired(t *testing.T) {
	dir := t.TempDir()
	dc := newDiskCache(dir)
	past := time.Now().Add(-10 * time.Minute)

	dc.write("meta.json", []byte(`{}`), past)

	_, fresh := dc.read("meta.json", 5*time.Minute, time.Now())
	if fresh {
		t.Error("expected cache to be expired")
	}
}

func TestDiskCache_MissingFile(t *testing.T) {
	dir := t.TempDir()
	dc := newDiskCache(dir)

	_, fresh := dc.read("nonexistent.json", 5*time.Minute, time.Now())
	if fresh {
		t.Error("expected false for missing file")
	}
}

func TestBuildMaps_RealAPIFormat(t *testing.T) {
	// The real Hyperliquid API returns spotMeta with a two-level structure:
	// - top-level "tokens" array: full token metadata objects
	// - universe[].tokens: integer indices into the top-level array
	// This test uses the exact shape returned by the live API.
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	dc.write("meta.json", []byte(`{"universe":[{"name":"BTC","szDecimals":5}]}`), now)
	dc.write("spot_meta.json", []byte(`{
		"tokens":[
			{"name":"USDC","szDecimals":6,"index":0},
			{"name":"PURR","szDecimals":0,"index":1}
		],
		"universe":[
			{"name":"PURR/USDC","index":0,"tokens":[1,0]}
		]
	}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "PURR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10000 {
		t.Errorf("AssetID = %d, want 10000 (10000 + market index 0)", info.AssetID)
	}
	if info.Coin != "PURR" {
		t.Errorf("Coin = %q, want %q", info.Coin, "PURR")
	}
	if info.SzDecimals != 0 {
		t.Errorf("SzDecimals = %d, want 0", info.SzDecimals)
	}
	if !info.IsSpot {
		t.Error("IsSpot = false, want true")
	}
}

func TestBuildMaps_TokenIndexOutOfBounds(t *testing.T) {
	// If a market references a token index not in the top-level array, buildMaps should error.
	// Use a server that returns the bad data (so the cache-fallthrough also fails).
	badSpot := `{"tokens":[{"name":"USDC","szDecimals":6,"index":0}],"universe":[{"name":"BAD/USDC","index":0,"tokens":[99,0]}]}`
	srv := newTestMetaServer(`{"universe":[]}`, badSpot)
	defer srv.Close()

	c := client.NewClient(srv.URL)
	r := NewResolver(c, t.TempDir(), 5*time.Minute)
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "BAD")
	if err == nil {
		t.Fatal("expected error for out-of-bounds token index")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrAPI {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrAPI)
	}
}

func TestSpotMarketWithEmptyTokens(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	dc.write("meta.json", []byte(`{"universe":[]}`), now)
	// A spot market with no tokens should be skipped without error.
	dc.write("spot_meta.json", []byte(`{
		"tokens":[{"name":"USDC","index":0,"szDecimals":6}],
		"universe":[{"name":"EMPTY/USDC","index":0,"tokens":[]}]
	}`), now)

	c := client.NewClient("http://127.0.0.1:1")
	r := NewResolver(c, cacheDir, 5*time.Minute)
	ctx := context.Background()

	_, err := r.ResolveAsset(ctx, "EMPTY")
	if err == nil {
		t.Fatal("expected error for nonexistent coin")
	}

	var cliErr *output.CLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *output.CLIError, got %T", err)
	}
	if cliErr.Code != output.ErrValidation {
		t.Errorf("error code = %s, want %s", cliErr.Code, output.ErrValidation)
	}
}
