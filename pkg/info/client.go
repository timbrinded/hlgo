package info

import (
	"context"
	"encoding/json"

	"github.com/timbrinded/hlgo/pkg/client"
)

// InfoClient wraps a client.Client with typed methods for each info endpoint.
type InfoClient struct {
	c *client.Client
}

// NewInfoClient creates an InfoClient wrapping the given HTTP client.
func NewInfoClient(c *client.Client) *InfoClient {
	return &InfoClient{c: c}
}

// AllMids fetches all mid-market prices.
func (ic *InfoClient) AllMids(ctx context.Context, dex string) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, AllMidsRequest{Type: "allMids", Dex: dex})
}

// Meta fetches perp universe metadata.
func (ic *InfoClient) Meta(ctx context.Context, dex string) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, MetaRequest{Type: "meta", Dex: dex})
}

// SpotMeta fetches spot universe metadata.
func (ic *InfoClient) SpotMeta(ctx context.Context) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, MetaRequest{Type: "spotMeta"})
}

// MetaAndAssetCtxs fetches perp metadata with asset contexts.
func (ic *InfoClient) MetaAndAssetCtxs(ctx context.Context, dex string) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, MetaAndCtxsRequest{Type: "metaAndAssetCtxs", Dex: dex})
}

// SpotMetaAndAssetCtxs fetches spot metadata with asset contexts.
func (ic *InfoClient) SpotMetaAndAssetCtxs(ctx context.Context) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, MetaAndCtxsRequest{Type: "spotMetaAndAssetCtxs"})
}

// L2Book fetches the L2 order book for a coin.
func (ic *InfoClient) L2Book(ctx context.Context, coin string, nSigFigs, mantissa *int) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, L2BookRequest{
		Type:     "l2Book",
		Coin:     coin,
		NSigFigs: nSigFigs,
		Mantissa: mantissa,
	})
}

// RecentTrades fetches recent trades for a coin.
func (ic *InfoClient) RecentTrades(ctx context.Context, coin string) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, RecentTradesRequest{Type: "recentTrades", Coin: coin})
}

// CandleSnapshot fetches OHLCV candle data.
func (ic *InfoClient) CandleSnapshot(ctx context.Context, coin, interval string, startTime, endTime int64) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, CandleSnapshotRequest{
		Type: "candleSnapshot",
		Req: CandleSnapshotReq{
			Coin:      coin,
			Interval:  interval,
			StartTime: startTime,
			EndTime:   endTime,
		},
	})
}

// ClearinghouseState fetches perp account state for a user.
func (ic *InfoClient) ClearinghouseState(ctx context.Context, user, dex string) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, ClearinghouseStateRequest{
		Type: "clearinghouseState",
		User: user,
		Dex:  dex,
	})
}

// SpotClearinghouseState fetches spot account state for a user.
func (ic *InfoClient) SpotClearinghouseState(ctx context.Context, user string) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, SpotClearinghouseStateRequest{
		Type: "spotClearinghouseState",
		User: user,
	})
}

// FrontendOpenOrders fetches open orders for a user.
func (ic *InfoClient) FrontendOpenOrders(ctx context.Context, user, dex string) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, FrontendOpenOrdersRequest{
		Type: "frontendOpenOrders",
		User: user,
		Dex:  dex,
	})
}

// UserFills fetches fill history for a user.
func (ic *InfoClient) UserFills(ctx context.Context, user string, aggregateByTime *bool) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, UserFillsRequest{
		Type:            "userFills",
		User:            user,
		AggregateByTime: aggregateByTime,
	})
}

// UserFillsByTime fetches fills for a user within a time range.
func (ic *InfoClient) UserFillsByTime(ctx context.Context, user string, startTime, endTime int64, aggregateByTime *bool) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, UserFillsRequest{
		Type:            "userFillsByTime",
		User:            user,
		StartTime:       startTime,
		EndTime:         endTime,
		AggregateByTime: aggregateByTime,
	})
}

// OrderStatus fetches the status of a specific order.
func (ic *InfoClient) OrderStatus(ctx context.Context, user string, oid any) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, OrderStatusRequest{
		Type: "orderStatus",
		User: user,
		Oid:  oid,
	})
}

// UserRateLimit fetches rate limit info for a user.
func (ic *InfoClient) UserRateLimit(ctx context.Context, user string) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, UserRateLimitRequest{
		Type: "userRateLimit",
		User: user,
	})
}

// FundingHistory fetches historical funding rates for a coin.
func (ic *InfoClient) FundingHistory(ctx context.Context, coin string, startTime, endTime int64) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, FundingHistoryRequest{
		Type:      "fundingHistory",
		Coin:      coin,
		StartTime: startTime,
		EndTime:   endTime,
	})
}

// PredictedFundings fetches predicted funding rates.
func (ic *InfoClient) PredictedFundings(ctx context.Context) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, PredictedFundingsRequest{Type: "predictedFundings"})
}

// PerpDexs fetches the list of HIP-3 perp dexes.
func (ic *InfoClient) PerpDexs(ctx context.Context) (json.RawMessage, error) {
	return ic.c.PostInfo(ctx, PerpDexsRequest{Type: "perpDexs"})
}
