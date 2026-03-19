package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/timbrinded/hlgo/pkg/output"
)

// perpMeta is the response shape from POST /info {"type":"meta"}.
type perpMeta struct {
	Universe []perpAsset `json:"universe"`
}

// perpAsset is a single entry in the perp universe array.
type perpAsset struct {
	Name       string `json:"name"`
	SzDecimals int    `json:"szDecimals"`
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

	var metadata perpMeta
	if err := json.Unmarshal(metaRaw, &metadata); err != nil {
		return output.NewCLIError(output.ErrAPI, "failed to parse HIP-3 metadata").
			WithDetails("dex", dex).
			WithDetails("cause", err.Error())
	}

	for index, asset := range metadata.Universe {
		upper := strings.ToUpper(asset.Name)
		r.perpMap[upper] = &AssetInfo{
			AssetID:       offset + index,
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
	request := map[string]string{"type": metaType}
	if dex != "" {
		request["dex"] = dex
	}
	return r.client.PostInfo(ctx, request)
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
		for key, value := range cliErr.Details {
			wrapped = wrapped.WithDetails(key, value)
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
