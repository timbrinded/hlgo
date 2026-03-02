package info

import (
	"encoding/json"
	"testing"
)

func TestParseMidsResult(t *testing.T) {
	raw := json.RawMessage(`{"BTC":"95123.5","ETH":"3412.1"}`)
	m, err := ParseMidsResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["BTC"] != "95123.5" {
		t.Errorf("BTC = %q, want %q", m["BTC"], "95123.5")
	}
	if m["ETH"] != "3412.1" {
		t.Errorf("ETH = %q, want %q", m["ETH"], "3412.1")
	}

	headers := m.Headers()
	if len(headers) != 2 || headers[0] != "COIN" || headers[1] != "MID" {
		t.Errorf("Headers() = %v, want [COIN MID]", headers)
	}

	rows := m.Rows()
	if len(rows) != 2 {
		t.Fatalf("Rows() len = %d, want 2", len(rows))
	}
	// Rows are sorted alphabetically by coin.
	if rows[0][0] != "BTC" {
		t.Errorf("first row coin = %q, want BTC", rows[0][0])
	}
}

func TestParseBookResult(t *testing.T) {
	raw := json.RawMessage(`{
		"coin": "BTC",
		"time": 1700000000000,
		"levels": [
			[{"px":"95000","sz":"1.5","n":3}],
			[{"px":"95100","sz":"0.8","n":1}]
		]
	}`)
	b, err := ParseBookResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Coin != "BTC" {
		t.Errorf("Coin = %q, want BTC", b.Coin)
	}
	if b.Time != 1700000000000 {
		t.Errorf("Time = %d, want 1700000000000", b.Time)
	}
	rows := b.Rows()
	if len(rows) != 2 {
		t.Fatalf("Rows() len = %d, want 2", len(rows))
	}
	if rows[0][0] != "bid" || rows[0][1] != "95000" {
		t.Errorf("first row = %v, want [bid 95000 ...]", rows[0])
	}
	if rows[1][0] != "ask" || rows[1][1] != "95100" {
		t.Errorf("second row = %v, want [ask 95100 ...]", rows[1])
	}
}

func TestParseTradesResult(t *testing.T) {
	raw := json.RawMessage(`[{"coin":"BTC","side":"B","px":"95000","sz":"0.1","time":1700000000000,"hash":"0xabc","tid":1}]`)
	tr, err := ParseTradesResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tr) != 1 {
		t.Fatalf("len = %d, want 1", len(tr))
	}
	rows := tr.Rows()
	if rows[0][1] != "BTC" || rows[0][3] != "95000" {
		t.Errorf("row = %v", rows[0])
	}
}

func TestParseCandlesResult(t *testing.T) {
	raw := json.RawMessage(`[{"T":1700003599999,"c":"95500","h":"96000","i":"1h","l":"94000","n":42,"o":"95000","s":"BTC","t":1700000000000,"v":"100.5"}]`)
	c, err := ParseCandlesResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	headers := c.Headers()
	if len(headers) != 6 {
		t.Errorf("Headers() len = %d, want 6", len(headers))
	}
	rows := c.Rows()
	if len(rows) != 1 {
		t.Fatalf("Rows() len = %d, want 1", len(rows))
	}
	if rows[0][1] != "95000" || rows[0][5] != "100.5" {
		t.Errorf("row = %v", rows[0])
	}
	if c[0].CloseTime != 1700003599999 {
		t.Errorf("CloseTime = %d, want 1700003599999", c[0].CloseTime)
	}
	if c[0].OpenTime != 1700000000000 {
		t.Errorf("OpenTime = %d, want 1700000000000", c[0].OpenTime)
	}
	if c[0].Interval != "1h" {
		t.Errorf("Interval = %q, want 1h", c[0].Interval)
	}
	if c[0].S != "BTC" {
		t.Errorf("S = %q, want BTC", c[0].S)
	}
	if c[0].N != 42 {
		t.Errorf("N = %d, want 42", c[0].N)
	}
}

func TestParseStateResult(t *testing.T) {
	raw := json.RawMessage(`{
		"assetPositions": [
			{
				"type": "oneWay",
				"position": {
					"coin": "BTC",
					"szi": "0.5",
					"entryPx": "90000",
					"unrealizedPnl": "2500",
					"liquidationPx": "80000",
					"leverage": {"type": "cross", "value": 10}
				}
			}
		],
		"marginSummary": {"accountValue":"1","totalMarginUsed":"2","totalNtlPos":"3","totalRawUsd":"4"},
		"crossMarginSummary": {"accountValue":"5","totalMarginUsed":"6","totalNtlPos":"7","totalRawUsd":"8"},
		"crossMaintenanceMarginUsed": "9",
		"withdrawable": "10",
		"time": 1700000000000
	}`)
	s, err := ParseStateResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.AssetPositions) != 1 {
		t.Fatalf("positions len = %d, want 1", len(s.AssetPositions))
	}
	rows := s.Rows()
	if rows[0][0] != "BTC" || rows[0][1] != "0.5" {
		t.Errorf("row = %v", rows[0])
	}
	if s.CrossMaintenanceMarginUsed != "9" {
		t.Errorf("CrossMaintenanceMarginUsed = %q, want 9", s.CrossMaintenanceMarginUsed)
	}
	if s.Withdrawable != "10" {
		t.Errorf("Withdrawable = %q, want 10", s.Withdrawable)
	}
	if s.Time != 1700000000000 {
		t.Errorf("Time = %d, want 1700000000000", s.Time)
	}
}

func TestParseOpenOrdersResult(t *testing.T) {
	raw := json.RawMessage(`[{"coin":"BTC","side":"B","limitPx":"90000","sz":"0.1","oid":12345,"timestamp":1700000000000,"orderType":"Limit"}]`)
	o, err := ParseOpenOrdersResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(o) != 1 {
		t.Fatalf("len = %d, want 1", len(o))
	}
	rows := o.Rows()
	if rows[0][6] != "12345" {
		t.Errorf("OID = %q, want 12345", rows[0][6])
	}
}

func TestParseFillsResult(t *testing.T) {
	raw := json.RawMessage(`[{"coin":"BTC","side":"B","px":"95000","sz":"0.1","time":1700000000000,"fee":"0.5","oid":1,"startPosition":"0","closedPnl":"0","crossed":false,"dir":"Open Long","hash":"0xabc","feeToken":"USDC","builderFee":"0.1","tid":9}]`)
	f, err := ParseFillsResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows := f.Rows()
	if len(rows) != 1 || rows[0][1] != "BTC" {
		t.Errorf("row = %v", rows[0])
	}
	if f[0].Hash != "0xabc" {
		t.Errorf("Hash = %q, want 0xabc", f[0].Hash)
	}
	if f[0].FeeToken != "USDC" {
		t.Errorf("FeeToken = %q, want USDC", f[0].FeeToken)
	}
	if f[0].Tid != 9 {
		t.Errorf("Tid = %d, want 9", f[0].Tid)
	}
	if f[0].BuilderFee == nil || *f[0].BuilderFee != "0.1" {
		t.Errorf("BuilderFee = %v, want 0.1", f[0].BuilderFee)
	}
}

func TestParseUserFundingResult(t *testing.T) {
	raw := json.RawMessage(`[{"time":1700000000000,"hash":"0xabc","delta":{"type":"funding","coin":"BTC","usdc":"-0.25","fundingRate":"0.0001"}}]`)
	funding, err := ParseUserFundingResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(funding) != 1 {
		t.Fatalf("len = %d, want 1", len(funding))
	}
	if funding[0].Delta.Type != "funding" {
		t.Errorf("Delta.Type = %q, want funding", funding[0].Delta.Type)
	}
	if funding[0].Delta.Coin != "BTC" {
		t.Errorf("Delta.Coin = %q, want BTC", funding[0].Delta.Coin)
	}
	if funding[0].Delta.USDC != "-0.25" {
		t.Errorf("Delta.USDC = %q, want -0.25", funding[0].Delta.USDC)
	}
}

func TestParseFundingResult_APR(t *testing.T) {
	raw := json.RawMessage(`[{"coin":"BTC","fundingRate":"0.0001","premium":"0","time":1700000000000}]`)
	f, err := ParseFundingResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows := f.Rows()
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1", len(rows))
	}
	// 0.0001 * 8760 = 0.876
	if rows[0][3] != "0.876" {
		t.Errorf("APR = %q, want 0.876", rows[0][3])
	}
}

func TestParsePerpDexsResult(t *testing.T) {
	raw := json.RawMessage(`[{"name":"xyz","index":1,"numMarkets":5}]`)
	p, err := ParsePerpDexsResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows := p.Rows()
	if len(rows) != 1 || rows[0][0] != "xyz" {
		t.Errorf("row = %v", rows[0])
	}
}

func TestParsePredictedFundingsResult(t *testing.T) {
	raw := json.RawMessage(`[
		["AVAX", [["BinPerp", {"fundingRate":"0.0001","nextFundingTime":1733961600000}], ["HlPerp", {"fundingRate":"0.0000125","nextFundingTime":1733958000000}]]]
	]`)
	p, err := ParsePredictedFundingsResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p) != 1 {
		t.Fatalf("len = %d, want 1", len(p))
	}
	if p[0].Coin != "AVAX" {
		t.Errorf("Coin = %q, want AVAX", p[0].Coin)
	}
	if len(p[0].Venues) != 2 {
		t.Fatalf("venues len = %d, want 2", len(p[0].Venues))
	}
	if p[0].Venues[0].Venue != "BinPerp" {
		t.Errorf("Venue = %q, want BinPerp", p[0].Venues[0].Venue)
	}
	if p[0].Venues[0].Details.FundingRate != "0.0001" {
		t.Errorf("FundingRate = %q, want 0.0001", p[0].Venues[0].Details.FundingRate)
	}
	rows := p.Rows()
	if len(rows) != 2 {
		t.Fatalf("rows len = %d, want 2", len(rows))
	}
	if rows[0][0] != "AVAX" {
		t.Errorf("first row coin = %q, want AVAX", rows[0][0])
	}
}

func TestComputeAPR_InvalidRate(t *testing.T) {
	result := computeAPR("not-a-number")
	if result != "N/A" {
		t.Errorf("got %q, want N/A", result)
	}
}
