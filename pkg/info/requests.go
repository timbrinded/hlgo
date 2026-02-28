// Package info provides request and response types for the Hyperliquid Info API.
package info

// AllMidsRequest fetches all mid-market prices.
type AllMidsRequest struct {
	Type string `json:"type"`
	Dex  string `json:"dex,omitempty"`
}

// MetaRequest fetches perp or spot universe metadata.
type MetaRequest struct {
	Type string `json:"type"`
	Dex  string `json:"dex,omitempty"`
}

// MetaAndCtxsRequest fetches metadata with asset contexts (e.g. mark price, funding).
type MetaAndCtxsRequest struct {
	Type string `json:"type"`
	Dex  string `json:"dex,omitempty"`
}

// L2BookRequest fetches the L2 order book for a coin.
type L2BookRequest struct {
	Type     string `json:"type"`
	Coin     string `json:"coin"`
	NSigFigs *int   `json:"nSigFigs,omitempty"`
	Mantissa *int   `json:"mantissa,omitempty"`
}

// RecentTradesRequest fetches recent trades for a coin.
type RecentTradesRequest struct {
	Type string `json:"type"`
	Coin string `json:"coin"`
}

// CandleSnapshotReq is the nested request object for candleSnapshot.
type CandleSnapshotReq struct {
	Coin      string `json:"coin"`
	Interval  string `json:"interval"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

// CandleSnapshotRequest fetches OHLCV candle data.
type CandleSnapshotRequest struct {
	Type string            `json:"type"`
	Req  CandleSnapshotReq `json:"req"`
}

// ClearinghouseStateRequest fetches perp account state.
type ClearinghouseStateRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
	Dex  string `json:"dex,omitempty"`
}

// SpotClearinghouseStateRequest fetches spot account state.
type SpotClearinghouseStateRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
}

// FrontendOpenOrdersRequest fetches open orders for a user.
type FrontendOpenOrdersRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
	Dex  string `json:"dex,omitempty"`
}

// UserFillsRequest fetches fill history for a user.
type UserFillsRequest struct {
	Type            string `json:"type"`
	User            string `json:"user"`
	StartTime       int64  `json:"startTime,omitempty"`
	EndTime         int64  `json:"endTime,omitempty"`
	AggregateByTime *bool  `json:"aggregateByTime,omitempty"`
}

// OrderStatusRequest fetches the status of a specific order.
// Oid can be int64 (numeric OID) or string (CLOID).
type OrderStatusRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
	Oid  any    `json:"oid"`
}

// UserRateLimitRequest fetches rate limit info for a user.
type UserRateLimitRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
}

// FundingHistoryRequest fetches historical funding rates.
type FundingHistoryRequest struct {
	Type      string `json:"type"`
	Coin      string `json:"coin"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime,omitempty"`
}

// PredictedFundingsRequest fetches predicted funding rates for all coins.
type PredictedFundingsRequest struct {
	Type string `json:"type"`
}

// PerpDexsRequest fetches the list of HIP-3 perp dexes.
type PerpDexsRequest struct {
	Type string `json:"type"`
}
