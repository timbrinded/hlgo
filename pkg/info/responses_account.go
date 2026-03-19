package info

import (
	"encoding/json"
	"fmt"
)

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
	var state StateResult
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	return &state, nil
}

func (*StateResult) Headers() []string {
	return []string{"COIN", "SIZE", "ENTRY_PX", "PNL", "LIQ_PRICE", "LEVERAGE"}
}

func (s *StateResult) Rows() [][]string {
	rows := make([][]string, 0, len(s.AssetPositions))
	for _, assetPosition := range s.AssetPositions {
		position := assetPosition.Position
		leverage := fmt.Sprintf("%s(%d)", position.Leverage.Type, position.Leverage.Value)
		rows = append(rows, []string{
			position.Coin,
			position.Szi,
			position.EntryPx,
			position.UnrealizedPnl,
			position.LiquidationPx,
			leverage,
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
	var orders OpenOrdersResult
	if err := json.Unmarshal(raw, &orders); err != nil {
		return nil, fmt.Errorf("parsing open orders: %w", err)
	}
	return orders, nil
}

func (OpenOrdersResult) Headers() []string {
	return []string{"COIN", "SIDE", "PRICE", "SIZE", "TYPE", "TIME", "OID"}
}

func (o OpenOrdersResult) Rows() [][]string {
	rows := make([][]string, 0, len(o))
	for _, order := range o {
		rows = append(rows, []string{
			order.Coin,
			order.Side,
			order.LimitPx,
			order.Sz,
			order.OrderType,
			formatTimestamp(order.Timestamp),
			fmt.Sprintf("%d", order.Oid),
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
	var fills FillsResult
	if err := json.Unmarshal(raw, &fills); err != nil {
		return nil, fmt.Errorf("parsing fills: %w", err)
	}
	return fills, nil
}

func (FillsResult) Headers() []string {
	return []string{"TIME", "COIN", "SIDE", "PRICE", "SIZE", "FEE", "OID"}
}

func (f FillsResult) Rows() [][]string {
	rows := make([][]string, 0, len(f))
	for _, fill := range f {
		rows = append(rows, []string{
			formatTimestamp(fill.Time),
			fill.Coin,
			fill.Side,
			fill.Px,
			fill.Sz,
			fill.Fee,
			fmt.Sprintf("%d", fill.Oid),
		})
	}
	return rows
}
