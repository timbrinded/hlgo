package info

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// MidsResult is a map of coin name to mid-market price string.
type MidsResult map[string]string

// ParseMidsResult unmarshals raw JSON into a MidsResult.
func ParseMidsResult(raw json.RawMessage) (MidsResult, error) {
	var mids MidsResult
	if err := json.Unmarshal(raw, &mids); err != nil {
		return nil, fmt.Errorf("parsing mids: %w", err)
	}
	return mids, nil
}

func (MidsResult) Headers() []string { return []string{"COIN", "MID"} }

func (m MidsResult) Rows() [][]string {
	coins := make([]string, 0, len(m))
	for coin := range m {
		coins = append(coins, coin)
	}
	sort.Strings(coins)
	rows := make([][]string, 0, len(coins))
	for _, coin := range coins {
		rows = append(rows, []string{coin, m[coin]})
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
	var book BookResult
	if err := json.Unmarshal(raw, &book); err != nil {
		return nil, fmt.Errorf("parsing book: %w", err)
	}
	return &book, nil
}

func (*BookResult) Headers() []string { return []string{"SIDE", "PRICE", "SIZE", "COUNT"} }

func (b *BookResult) Rows() [][]string {
	var rows [][]string
	if len(b.Levels) > 0 {
		for _, level := range b.Levels[0].Levels {
			rows = append(rows, []string{"bid", level.Px, level.Sz, fmt.Sprintf("%d", level.N)})
		}
	}
	if len(b.Levels) > 1 {
		for _, level := range b.Levels[1].Levels {
			rows = append(rows, []string{"ask", level.Px, level.Sz, fmt.Sprintf("%d", level.N)})
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
	var trades TradesResult
	if err := json.Unmarshal(raw, &trades); err != nil {
		return nil, fmt.Errorf("parsing trades: %w", err)
	}
	return trades, nil
}

func (TradesResult) Headers() []string { return []string{"TIME", "COIN", "SIDE", "PRICE", "SIZE"} }

func (t TradesResult) Rows() [][]string {
	rows := make([][]string, 0, len(t))
	for _, trade := range t {
		rows = append(rows, []string{
			formatTimestamp(trade.Time),
			trade.Coin, trade.Side, trade.Px, trade.Sz,
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
	var candles CandlesResult
	if err := json.Unmarshal(raw, &candles); err != nil {
		return nil, fmt.Errorf("parsing candles: %w", err)
	}
	return candles, nil
}

func (CandlesResult) Headers() []string {
	return []string{"TIME", "OPEN", "HIGH", "LOW", "CLOSE", "VOLUME"}
}

func (c CandlesResult) Rows() [][]string {
	rows := make([][]string, 0, len(c))
	for _, candle := range c {
		rows = append(rows, []string{
			formatTimestamp(candle.OpenTime),
			candle.O, candle.H, candle.L, candle.C, candle.V,
		})
	}
	return rows
}

// formatTimestamp formats a Unix millisecond timestamp as RFC3339.
func formatTimestamp(ms int64) string {
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
