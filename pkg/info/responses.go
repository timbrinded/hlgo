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
	Time   int64      `json:"time"`
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
	CloseTime int64  `json:"T"` // close time in ms
	C         string `json:"c"` // close
	H         string `json:"h"` // high
	Interval  string `json:"i"` // interval
	L         string `json:"l"` // low
	N         int64  `json:"n"` // trade count
	O         string `json:"o"` // open
	S         string `json:"s"` // symbol
	OpenTime  int64  `json:"t"` // open time in ms
	V         string `json:"v"` // volume
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
			formatTimestamp(cd.OpenTime),
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
		Type   string `json:"type"`
		Value  int    `json:"value"`
		RawUSD string `json:"rawUsd,omitempty"`
	} `json:"leverage"`
}

// AssetPosition wraps a position with its type field.
type AssetPosition struct {
	Type     string   `json:"type"`
	Position Position `json:"position"`
}

// MarginSummary represents account-level margin metrics.
type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalMarginUsed string `json:"totalMarginUsed"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUSD     string `json:"totalRawUsd"`
}

// StateResult represents the clearinghouse state response.
type StateResult struct {
	AssetPositions             []AssetPosition `json:"assetPositions"`
	MarginSummary              MarginSummary   `json:"marginSummary"`
	CrossMarginSummary         MarginSummary   `json:"crossMarginSummary"`
	CrossMaintenanceMarginUsed string          `json:"crossMaintenanceMarginUsed"`
	Withdrawable               string          `json:"withdrawable"`
	Time                       int64           `json:"time"`
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
	Coin             string `json:"coin"`
	Side             string `json:"side"`
	LimitPx          string `json:"limitPx"`
	Sz               string `json:"sz"`
	Oid              int64  `json:"oid"`
	Timestamp        int64  `json:"timestamp"`
	OrderType        string `json:"orderType"`
	Cloid            string `json:"cloid,omitempty"`
	IsPositionTpsl   bool   `json:"isPositionTpsl"`
	IsTrigger        bool   `json:"isTrigger"`
	OrigSz           string `json:"origSz"`
	ReduceOnly       bool   `json:"reduceOnly"`
	TriggerCondition string `json:"triggerCondition"`
	TriggerPx        string `json:"triggerPx"`
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
	Coin          string  `json:"coin"`
	Side          string  `json:"side"`
	Px            string  `json:"px"`
	Sz            string  `json:"sz"`
	Time          int64   `json:"time"`
	Fee           string  `json:"fee"`
	Oid           int64   `json:"oid"`
	StartPosition string  `json:"startPosition"`
	ClosedPnl     string  `json:"closedPnl"`
	Crossed       bool    `json:"crossed"`
	Dir           string  `json:"dir"`
	Hash          string  `json:"hash"`
	FeeToken      string  `json:"feeToken"`
	BuilderFee    *string `json:"builderFee,omitempty"`
	Tid           int64   `json:"tid"`
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

// PredictedFundingVenueDetails holds venue funding details.
type PredictedFundingVenueDetails struct {
	FundingRate     string `json:"fundingRate"`
	NextFundingTime int64  `json:"nextFundingTime"`
}

// PredictedFundingVenue represents one venue entry for a coin.
type PredictedFundingVenue struct {
	Venue   string
	Details PredictedFundingVenueDetails
}

// PredictedFundingCoin represents one coin entry with all venue predictions.
type PredictedFundingCoin struct {
	Coin   string
	Venues []PredictedFundingVenue
}

// PredictedFundingsResult wraps the nested predicted funding response.
// API shape:
// [
//
//	["AVAX", [["BinPerp", {...}], ["HlPerp", {...}]]],
//	...
//
// ]
type PredictedFundingsResult []PredictedFundingCoin

// ParsePredictedFundingsResult unmarshals raw JSON into a PredictedFundingsResult.
func ParsePredictedFundingsResult(raw json.RawMessage) (PredictedFundingsResult, error) {
	var outer []json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("parsing predicted fundings: %w", err)
	}

	result := make(PredictedFundingsResult, 0, len(outer))
	for i, coinEntryRaw := range outer {
		var coinEntry []json.RawMessage
		if err := json.Unmarshal(coinEntryRaw, &coinEntry); err != nil {
			return nil, fmt.Errorf("parsing predicted fundings coin entry %d: %w", i, err)
		}
		if len(coinEntry) != 2 {
			return nil, fmt.Errorf("parsing predicted fundings coin entry %d: expected length 2, got %d", i, len(coinEntry))
		}

		var coin string
		if err := json.Unmarshal(coinEntry[0], &coin); err != nil {
			return nil, fmt.Errorf("parsing predicted fundings coin name %d: %w", i, err)
		}

		var venuesRaw []json.RawMessage
		if err := json.Unmarshal(coinEntry[1], &venuesRaw); err != nil {
			return nil, fmt.Errorf("parsing predicted fundings venues for %s: %w", coin, err)
		}

		venues := make([]PredictedFundingVenue, 0, len(venuesRaw))
		for j, venueEntryRaw := range venuesRaw {
			var venueEntry []json.RawMessage
			if err := json.Unmarshal(venueEntryRaw, &venueEntry); err != nil {
				return nil, fmt.Errorf("parsing predicted fundings venue entry %s[%d]: %w", coin, j, err)
			}
			if len(venueEntry) != 2 {
				return nil, fmt.Errorf("parsing predicted fundings venue entry %s[%d]: expected length 2, got %d", coin, j, len(venueEntry))
			}

			var venue string
			if err := json.Unmarshal(venueEntry[0], &venue); err != nil {
				return nil, fmt.Errorf("parsing predicted fundings venue name %s[%d]: %w", coin, j, err)
			}

			var details PredictedFundingVenueDetails
			if err := json.Unmarshal(venueEntry[1], &details); err != nil {
				return nil, fmt.Errorf("parsing predicted fundings venue details %s[%d]: %w", coin, j, err)
			}

			venues = append(venues, PredictedFundingVenue{
				Venue:   venue,
				Details: details,
			})
		}

		result = append(result, PredictedFundingCoin{
			Coin:   coin,
			Venues: venues,
		})
	}

	return result, nil
}

func (PredictedFundingsResult) Headers() []string {
	return []string{"COIN", "VENUE", "PREDICTED_RATE", "APR"}
}

func (p PredictedFundingsResult) Rows() [][]string {
	rows := make([][]string, 0, len(p))
	for _, coinEntry := range p {
		for _, venueEntry := range coinEntry.Venues {
			rate := venueEntry.Details.FundingRate
			apr := computeAPR(rate)
			rows = append(rows, []string{coinEntry.Coin, venueEntry.Venue, rate, apr})
		}
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
