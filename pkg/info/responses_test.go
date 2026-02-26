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
	raw := json.RawMessage(`[{"t":1700000000000,"o":"95000","h":"96000","l":"94000","c":"95500","v":"100.5"}]`)
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
		"marginSummary": {}
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
	raw := json.RawMessage(`[{"coin":"BTC","side":"B","px":"95000","sz":"0.1","time":1700000000000,"fee":"0.5","oid":1,"startPosition":"0","closedPnl":"0"}]`)
	f, err := ParseFillsResult(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows := f.Rows()
	if len(rows) != 1 || rows[0][1] != "BTC" {
		t.Errorf("row = %v", rows[0])
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

func TestComputeAPR_InvalidRate(t *testing.T) {
	result := computeAPR("not-a-number")
	if result != "N/A" {
		t.Errorf("got %q, want N/A", result)
	}
}
