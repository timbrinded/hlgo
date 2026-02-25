package output

import (
	"encoding/csv"
	"encoding/json"
	"io"

	"github.com/olekukonko/tablewriter"
)

// Formatter renders command output to a writer.
type Formatter interface {
	Format(w io.Writer, data any) error
}

// Tabular is implemented by types that can render as rows and columns.
// Types that don't implement Tabular fall back to JSON in table/CSV modes.
type Tabular interface {
	Headers() []string
	Rows() [][]string
}

// NewFormatter returns a Formatter for the given format string.
// Unknown formats default to JSON. When quiet is true, a no-op formatter is returned.
func NewFormatter(format string, quiet bool) Formatter {
	if quiet {
		return quietFormatter{}
	}
	switch format {
	case "table":
		return tableFormatter{}
	case "csv":
		return csvFormatter{}
	default:
		return jsonFormatter{}
	}
}

// jsonFormatter renders data as indented JSON.
// Types implementing json.Marshaler (e.g. shopspring/decimal) control their own serialization.
type jsonFormatter struct{}

func (jsonFormatter) Format(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// tableFormatter renders Tabular data as an ASCII table.
// Non-Tabular data falls back to JSON.
type tableFormatter struct{}

func (tableFormatter) Format(w io.Writer, data any) error {
	tab, ok := data.(Tabular)
	if !ok {
		return jsonFormatter{}.Format(w, data)
	}

	headers := tab.Headers()
	hdr := make([]any, len(headers))
	for i, h := range headers {
		hdr[i] = h
	}

	t := tablewriter.NewTable(w)
	t.Header(hdr...)
	if err := t.Bulk(tab.Rows()); err != nil {
		return err
	}
	return t.Render()
}

// csvFormatter renders Tabular data as CSV.
// Non-Tabular data falls back to JSON.
type csvFormatter struct{}

func (csvFormatter) Format(w io.Writer, data any) error {
	tab, ok := data.(Tabular)
	if !ok {
		return jsonFormatter{}.Format(w, data)
	}

	cw := csv.NewWriter(w)
	if err := cw.Write(tab.Headers()); err != nil {
		return err
	}
	if err := cw.WriteAll(tab.Rows()); err != nil {
		return err
	}
	return nil
}

// quietFormatter suppresses all output. Used with --quiet flag.
type quietFormatter struct{}

func (quietFormatter) Format(_ io.Writer, _ any) error {
	return nil
}
