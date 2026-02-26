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

// testSpotMeta returns a realistic spot meta response.
// The token index determines the asset ID: 10000 + index.
func testSpotMeta() string {
	return `{
		"universe": [
			{
				"name": "PURR/USDC",
				"index": 0,
				"tokens": [
					{"name": "PURR", "index": 1, "szDecimals": 0},
					{"name": "USDC", "index": 0, "szDecimals": 6}
				]
			},
			{
				"name": "HFUN/USDC",
				"index": 1,
				"tokens": [
					{"name": "HFUN", "index": 2, "szDecimals": 3},
					{"name": "USDC", "index": 0, "szDecimals": 6}
				]
			}
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
	if info.AssetID != 10001 {
		t.Errorf("AssetID = %d, want 10001 (10000 + index 1)", info.AssetID)
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

func TestResolveSpotCoin_HFUN(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	info, err := r.ResolveAsset(ctx, "HFUN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.AssetID != 10002 {
		t.Errorf("AssetID = %d, want 10002 (10000 + index 2)", info.AssetID)
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
	if info.AssetID != 10001 {
		t.Errorf("AssetID = %d, want 10001", info.AssetID)
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
		{"PURR", 10001, true},
		{"HFUN", 10002, true},
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

func TestSpotAssetIDFormula_10000PlusIndex(t *testing.T) {
	r := newPreloadedResolver(t)
	ctx := context.Background()

	// PURR token has index=1, HFUN token has index=2.
	expected := map[string]int{"PURR": 10001, "HFUN": 10002}
	for coin, wantID := range expected {
		info, err := r.ResolveAsset(ctx, coin)
		if err != nil {
			t.Errorf("ResolveAsset(%q) error: %v", coin, err)
			continue
		}
		if info.AssetID != wantID {
			t.Errorf("%s AssetID = %d, want %d (10000 + token index)", coin, info.AssetID, wantID)
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
		"universe":[{
			"name":"BTC/USDC","index":0,
			"tokens":[{"name":"BTC","index":99,"szDecimals":8},{"name":"USDC","index":0,"szDecimals":6}]
		}]
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

func TestSpotMarketWithEmptyTokens(t *testing.T) {
	cacheDir := t.TempDir()
	now := time.Now()
	dc := newDiskCache(cacheDir)

	dc.write("meta.json", []byte(`{"universe":[]}`), now)
	// A spot market with no tokens should be skipped without error.
	dc.write("spot_meta.json", []byte(`{
		"universe":[{
			"name":"EMPTY/USDC","index":0,
			"tokens":[]
		}]
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
