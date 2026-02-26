package info

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// annualHours is the number of hours per year, used for APR calculation from hourly rates.
var annualHours = decimal.NewFromInt(8760)

// MidsResult is a map of coin name to mid-market price string.
type MidsResult map[string]string

// ParseMidsResult unmarshals raw JSON into a MidsResult.
func ParseMidsResult(raw json.RawMessage) (MidsResult, error) {
	var m MidsResult
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parsing mids: %w", err)
	}
	return m, nil
}

func (MidsResult) Headers() []string { return []string{"COIN", "MID"} }

func (m MidsResult) Rows() [][]string {
	coins := make([]string, 0, len(m))
	for c := range m {
		coins = append(coins, c)
	}
	sort.Strings(coins)
	rows := make([][]string, 0, len(coins))
	for _, c := range coins {
		rows = append(rows, []string{c, m[c]})
	}
	return rows
}

// BookLevel represents a single price level in the order book.
type BookLevel struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}

// BookSide is a list of price levels on one side of the book.
type BookSide struct {
	Levels []BookLevel `json:"-"`
}

// UnmarshalJSON handles the API's nested array format for book sides.
func (bs *BookSide) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &bs.Levels)
}

// BookResult represents the L2 order book response.
type BookResult struct {
	Coin   string     `json:"coin"`
	Levels []BookSide `json:"levels"`
}

// ParseBookResult unmarshals raw JSON into a BookResult.
func ParseBookResult(raw json.RawMessage) (*BookResult, error) {
	var b BookResult
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("parsing book: %w", err)
	}
	return &b, nil
}

func (*BookResult) Headers() []string { return []string{"SIDE", "PRICE", "SIZE", "COUNT"} }

func (b *BookResult) Rows() [][]string {
	var rows [][]string
	if len(b.Levels) > 0 {
		for _, lvl := range b.Levels[0].Levels {
			rows = append(rows, []string{"bid", lvl.Px, lvl.Sz, fmt.Sprintf("%d", lvl.N)})
		}
	}
	if len(b.Levels) > 1 {
		for _, lvl := range b.Levels[1].Levels {
			rows = append(rows, []string{"ask", lvl.Px, lvl.Sz, fmt.Sprintf("%d", lvl.N)})
		}
	}
	return rows
}

// Trade represents a single recent trade.
type Trade struct {
	Coin string `json:"coin"`
	Side string `json:"side"`
	Px   string `json:"px"`
	Sz   string `json:"sz"`
	Time int64  `json:"time"`
	Hash string `json:"hash"`
	Tid  int64  `json:"tid"`
}

// TradesResult is a list of recent trades.
type TradesResult []Trade

// ParseTradesResult unmarshals raw JSON into a TradesResult.
func ParseTradesResult(raw json.RawMessage) (TradesResult, error) {
	var t TradesResult
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("parsing trades: %w", err)
	}
	return t, nil
}

func (TradesResult) Headers() []string { return []string{"TIME", "COIN", "SIDE", "PRICE", "SIZE"} }

func (t TradesResult) Rows() [][]string {
	rows := make([][]string, 0, len(t))
	for _, tr := range t {
		rows = append(rows, []string{
			formatTimestamp(tr.Time),
			tr.Coin, tr.Side, tr.Px, tr.Sz,
		})
	}
	return rows
}

// Candle represents a single OHLCV candle.
type Candle struct {
	T int64  `json:"t"` // open time in ms
	O string `json:"o"` // open
	H string `json:"h"` // high
	L string `json:"l"` // low
	C string `json:"c"` // close
	V string `json:"v"` // volume
}

// CandlesResult is a list of candles.
type CandlesResult []Candle

// ParseCandlesResult unmarshals raw JSON into a CandlesResult.
func ParseCandlesResult(raw json.RawMessage) (CandlesResult, error) {
	var c CandlesResult
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parsing candles: %w", err)
	}
	return c, nil
}

func (CandlesResult) Headers() []string {
	return []string{"TIME", "OPEN", "HIGH", "LOW", "CLOSE", "VOLUME"}
}

func (c CandlesResult) Rows() [][]string {
	rows := make([][]string, 0, len(c))
	for _, cd := range c {
		rows = append(rows, []string{
			formatTimestamp(cd.T),
			cd.O, cd.H, cd.L, cd.C, cd.V,
		})
	}
	return rows
}

// Position represents a single perp position in clearinghouse state.
type Position struct {
	Coin          string `json:"coin"`
	Szi           string `json:"szi"`
	EntryPx       string `json:"entryPx"`
	UnrealizedPnl string `json:"unrealizedPnl"`
	LiquidationPx string `json:"liquidationPx,omitempty"`
	Leverage      struct {
		Type  string `json:"type"`
		Value int    `json:"value"`
	} `json:"leverage"`
}

// AssetPosition wraps a position with its type field.
type AssetPosition struct {
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

// StateResult represents the clearinghouse state response.
type StateResult struct {
	AssetPositions []AssetPosition `json:"assetPositions"`
	MarginSummary  json.RawMessage `json:"marginSummary"`
}

// ParseStateResult unmarshals raw JSON into a StateResult.
func ParseStateResult(raw json.RawMessage) (*StateResult, error) {
	var s StateResult
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	return &s, nil
}

func (*StateResult) Headers() []string {
	return []string{"COIN", "SIZE", "ENTRY_PX", "PNL", "LIQ_PRICE", "LEVERAGE"}
}

func (s *StateResult) Rows() [][]string {
	rows := make([][]string, 0, len(s.AssetPositions))
	for _, ap := range s.AssetPositions {
		p := ap.Position
		lev := fmt.Sprintf("%s(%d)", p.Leverage.Type, p.Leverage.Value)
		rows = append(rows, []string{
			p.Coin, p.Szi, p.EntryPx, p.UnrealizedPnl, p.LiquidationPx, lev,
		})
	}
	return rows
}

// OpenOrder represents a single open order.
type OpenOrder struct {
	Coin      string `json:"coin"`
	Side      string `json:"side"`
	LimitPx   string `json:"limitPx"`
	Sz        string `json:"sz"`
	Oid       int64  `json:"oid"`
	Timestamp int64  `json:"timestamp"`
	OrderType string `json:"orderType"`
	Cloid     string `json:"cloid,omitempty"`
}

// OpenOrdersResult is a list of open orders.
type OpenOrdersResult []OpenOrder

// ParseOpenOrdersResult unmarshals raw JSON into an OpenOrdersResult.
func ParseOpenOrdersResult(raw json.RawMessage) (OpenOrdersResult, error) {
	var o OpenOrdersResult
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, fmt.Errorf("parsing open orders: %w", err)
	}
	return o, nil
}

func (OpenOrdersResult) Headers() []string {
	return []string{"COIN", "SIDE", "PRICE", "SIZE", "TYPE", "TIME", "OID"}
}

func (o OpenOrdersResult) Rows() [][]string {
	rows := make([][]string, 0, len(o))
	for _, ord := range o {
		rows = append(rows, []string{
			ord.Coin, ord.Side, ord.LimitPx, ord.Sz, ord.OrderType,
			formatTimestamp(ord.Timestamp),
			fmt.Sprintf("%d", ord.Oid),
		})
	}
	return rows
}

// Fill represents a single trade fill.
type Fill struct {
	Coin          string `json:"coin"`
	Side          string `json:"side"`
	Px            string `json:"px"`
	Sz            string `json:"sz"`
	Time          int64  `json:"time"`
	Fee           string `json:"fee"`
	Oid           int64  `json:"oid"`
	StartPosition string `json:"startPosition"`
	ClosedPnl     string `json:"closedPnl"`
}

// FillsResult is a list of fills.
type FillsResult []Fill

// ParseFillsResult unmarshals raw JSON into a FillsResult.
func ParseFillsResult(raw json.RawMessage) (FillsResult, error) {
	var f FillsResult
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parsing fills: %w", err)
	}
	return f, nil
}

func (FillsResult) Headers() []string {
	return []string{"TIME", "COIN", "SIDE", "PRICE", "SIZE", "FEE", "OID"}
}

func (f FillsResult) Rows() [][]string {
	rows := make([][]string, 0, len(f))
	for _, fl := range f {
		rows = append(rows, []string{
			formatTimestamp(fl.Time),
			fl.Coin, fl.Side, fl.Px, fl.Sz, fl.Fee,
			fmt.Sprintf("%d", fl.Oid),
		})
	}
	return rows
}

// FundingEntry represents a single funding rate entry.
type FundingEntry struct {
	Coin        string `json:"coin"`
	FundingRate string `json:"fundingRate"`
	Premium     string `json:"premium"`
	Time        int64  `json:"time"`
}

// FundingResult is a list of funding entries.
type FundingResult []FundingEntry

// ParseFundingResult unmarshals raw JSON into a FundingResult.
func ParseFundingResult(raw json.RawMessage) (FundingResult, error) {
	var f FundingResult
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parsing funding: %w", err)
	}
	return f, nil
}

func (FundingResult) Headers() []string { return []string{"COIN", "TIME", "RATE", "APR"} }

func (f FundingResult) Rows() [][]string {
	rows := make([][]string, 0, len(f))
	for _, fe := range f {
		apr := computeAPR(fe.FundingRate)
		rows = append(rows, []string{
			fe.Coin, formatTimestamp(fe.Time), fe.FundingRate, apr,
		})
	}
	return rows
}

// PredictedFunding represents a single predicted funding entry.
type PredictedFunding struct {
	Coin  string `json:"coin"`
	Venue string `json:"venue"`
	Rate  string `json:"rate"`
}

// PredictedFundingsResult wraps the predicted fundings response.
// The API returns an array of [coin, venue, rate_string] arrays.
type PredictedFundingsResult [][]json.RawMessage

// ParsePredictedFundingsResult unmarshals raw JSON into a PredictedFundingsResult.
func ParsePredictedFundingsResult(raw json.RawMessage) (PredictedFundingsResult, error) {
	var p PredictedFundingsResult
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parsing predicted fundings: %w", err)
	}
	return p, nil
}

func (PredictedFundingsResult) Headers() []string {
	return []string{"COIN", "VENUE", "PREDICTED_RATE", "APR"}
}

func (p PredictedFundingsResult) Rows() [][]string {
	rows := make([][]string, 0, len(p))
	for _, entry := range p {
		if len(entry) < 3 {
			continue
		}
		var coin, venue, rate string
		json.Unmarshal(entry[0], &coin)  //nolint:errcheck // best-effort
		json.Unmarshal(entry[1], &venue) //nolint:errcheck // best-effort
		json.Unmarshal(entry[2], &rate)  //nolint:errcheck // best-effort
		apr := computeAPR(rate)
		rows = append(rows, []string{coin, venue, rate, apr})
	}
	return rows
}

// PerpDex represents a HIP-3 perp dex.
type PerpDex struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	NumMarkets int    `json:"numMarkets"`
}

// PerpDexsResult is a list of perp dexes.
type PerpDexsResult []PerpDex

// ParsePerpDexsResult unmarshals raw JSON into a PerpDexsResult.
func ParsePerpDexsResult(raw json.RawMessage) (PerpDexsResult, error) {
	var p PerpDexsResult
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("parsing perp dexs: %w", err)
	}
	return p, nil
}

func (PerpDexsResult) Headers() []string { return []string{"NAME", "INDEX", "NUM_MARKETS"} }

func (p PerpDexsResult) Rows() [][]string {
	rows := make([][]string, 0, len(p))
	for _, d := range p {
		rows = append(rows, []string{
			d.Name, fmt.Sprintf("%d", d.Index), fmt.Sprintf("%d", d.NumMarkets),
		})
	}
	return rows
}

// computeAPR converts an hourly funding rate string to an annualized percentage.
func computeAPR(rateStr string) string {
	rate, err := decimal.NewFromString(rateStr)
	if err != nil {
		return "N/A"
	}
	return rate.Mul(annualHours).String()
}

// formatTimestamp formats a Unix millisecond timestamp as RFC3339.
func formatTimestamp(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
