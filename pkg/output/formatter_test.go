package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// stringDecimal simulates shopspring/decimal's json.Marshaler behavior:
// it serializes as a JSON string, not a number.
type stringDecimal string

func (d stringDecimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(d))
}

type priceData struct {
	Symbol string        `json:"symbol"`
	Price  stringDecimal `json:"price"`
}

// testTabular implements Tabular for table/CSV tests.
type testTabular struct {
	headers []string
	rows    [][]string
}

func (t testTabular) Headers() []string { return t.headers }
func (t testTabular) Rows() [][]string  { return t.rows }

func TestJSONFormatter(t *testing.T) {
	t.Run("struct to indented JSON", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter("json", false)
		data := struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}{"BTC", 42}

		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("Format: %v", err)
		}

		want := "{\n  \"name\": \"BTC\",\n  \"count\": 42\n}\n"
		if buf.String() != want {
			t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
		}
	})

	t.Run("custom Marshaler preserves string output", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter("json", false)
		data := priceData{Symbol: "BTC", Price: "95123.5"}

		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("Format: %v", err)
		}

		// Price must be a JSON string, not a number
		if !strings.Contains(buf.String(), `"price": "95123.5"`) {
			t.Errorf("expected string price, got:\n%s", buf.String())
		}
	})
}

func TestTableFormatter(t *testing.T) {
	t.Run("tabular data renders table", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter("table", false)
		data := testTabular{
			headers: []string{"Symbol", "Price"},
			rows:    [][]string{{"BTC", "95123.5"}, {"ETH", "3456.7"}},
		}

		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("Format: %v", err)
		}

		out := buf.String()
		upper := strings.ToUpper(out)
		// Table output should contain headers (uppercased by tablewriter) and data
		for _, want := range []string{"SYMBOL", "PRICE", "BTC", "95123.5", "ETH", "3456.7"} {
			if !strings.Contains(upper, strings.ToUpper(want)) {
				t.Errorf("table output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("non-tabular falls back to JSON", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter("table", false)
		data := map[string]string{"key": "value"}

		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("Format: %v", err)
		}

		if !strings.Contains(buf.String(), `"key": "value"`) {
			t.Errorf("expected JSON fallback, got:\n%s", buf.String())
		}
	})
}

func TestCSVFormatter(t *testing.T) {
	t.Run("tabular data renders CSV", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter("csv", false)
		data := testTabular{
			headers: []string{"Symbol", "Price"},
			rows:    [][]string{{"BTC", "95123.5"}, {"ETH", "3456.7"}},
		}

		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("Format: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines (header + 2 rows), got %d:\n%s", len(lines), buf.String())
		}
		if lines[0] != "Symbol,Price" {
			t.Errorf("header = %q, want %q", lines[0], "Symbol,Price")
		}
		if lines[1] != "BTC,95123.5" {
			t.Errorf("row 1 = %q, want %q", lines[1], "BTC,95123.5")
		}
	})

	t.Run("non-tabular falls back to JSON", func(t *testing.T) {
		var buf bytes.Buffer
		f := NewFormatter("csv", false)
		data := map[string]string{"key": "value"}

		if err := f.Format(&buf, data); err != nil {
			t.Fatalf("Format: %v", err)
		}

		if !strings.Contains(buf.String(), `"key": "value"`) {
			t.Errorf("expected JSON fallback, got:\n%s", buf.String())
		}
	})
}

func TestQuietFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("json", true)

	if err := f.Format(&buf, map[string]string{"key": "value"}); err != nil {
		t.Fatalf("Format: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("quiet formatter should produce no output, got: %q", buf.String())
	}
}

func TestNewFormatterFactory(t *testing.T) {
	tests := []struct {
		format string
		quiet  bool
		want   string
	}{
		{"json", false, "output.jsonFormatter"},
		{"table", false, "output.tableFormatter"},
		{"csv", false, "output.csvFormatter"},
		{"unknown", false, "output.jsonFormatter"},
		{"json", true, "output.quietFormatter"},
		{"table", true, "output.quietFormatter"},
	}
	for _, tt := range tests {
		name := tt.format
		if tt.quiet {
			name += "+quiet"
		}
		t.Run(name, func(t *testing.T) {
			f := NewFormatter(tt.format, tt.quiet)
			got := fmt.Sprintf("%T", f)
			if got != tt.want {
				t.Errorf("NewFormatter(%q, %v) type = %s, want %s", tt.format, tt.quiet, got, tt.want)
			}
		})
	}
}
