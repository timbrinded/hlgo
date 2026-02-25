package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want int
	}{
		{ErrValidation, 1},
		{ErrConfig, 2},
		{ErrNetwork, 3},
		{ErrAPI, 4},
		{ErrSigning, 5},
		{ErrRateLimit, 6},
		{ErrorCode("UNKNOWN"), 1},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			e := NewCLIError(tt.code, "test")
			if got := e.ExitCode(); got != tt.want {
				t.Errorf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestErrorReturnsMessage(t *testing.T) {
	e := NewCLIError(ErrAPI, "something broke")
	if got := e.Error(); got != "something broke" {
		t.Errorf("Error() = %q, want %q", got, "something broke")
	}
}

func TestWithDetails(t *testing.T) {
	e := NewCLIError(ErrValidation, "bad input").
		WithDetails("field", "price").
		WithDetails("value", "abc")

	if e.Details["field"] != "price" {
		t.Errorf("field = %v, want %q", e.Details["field"], "price")
	}
	if e.Details["value"] != "abc" {
		t.Errorf("value = %v, want %q", e.Details["value"], "abc")
	}
}

func TestJSONSerialization(t *testing.T) {
	t.Run("without details", func(t *testing.T) {
		e := NewCLIError(ErrNetwork, "connection refused")
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		if got["error"] != "connection refused" {
			t.Errorf("error = %v, want %q", got["error"], "connection refused")
		}
		if got["code"] != "NETWORK_ERROR" {
			t.Errorf("code = %v, want %q", got["code"], "NETWORK_ERROR")
		}
		if _, ok := got["details"]; ok {
			t.Error("details should be omitted when empty")
		}
	})

	t.Run("with details", func(t *testing.T) {
		e := NewCLIError(ErrValidation, "invalid size").
			WithDetails("min", "0.001").
			WithDetails("got", "0")

		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		details, ok := got["details"].(map[string]any)
		if !ok {
			t.Fatal("details missing or wrong type")
		}
		if details["min"] != "0.001" {
			t.Errorf("details.min = %v, want %q", details["min"], "0.001")
		}
	})
}

func TestWriteError_CLIError(t *testing.T) {
	var buf bytes.Buffer
	e := NewCLIError(ErrSigning, "invalid key")
	code := WriteError(&buf, e)

	if code != 5 {
		t.Errorf("exit code = %d, want 5", code)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["code"] != "SIGNING_ERROR" {
		t.Errorf("code = %v, want SIGNING_ERROR", got["code"])
	}
}

func TestWriteError_PlainError(t *testing.T) {
	var buf bytes.Buffer
	code := WriteError(&buf, fmt.Errorf("plain failure"))

	if code != 4 {
		t.Errorf("exit code = %d, want 4 (ErrAPI default)", code)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["code"] != "API_ERROR" {
		t.Errorf("code = %v, want API_ERROR", got["code"])
	}
	if got["error"] != "plain failure" {
		t.Errorf("error = %v, want %q", got["error"], "plain failure")
	}
}

func TestWriteError_WrappedCLIError(t *testing.T) {
	var buf bytes.Buffer
	inner := NewCLIError(ErrConfig, "missing key")
	wrapped := fmt.Errorf("config load: %w", inner)
	code := WriteError(&buf, wrapped)

	if code != 2 {
		t.Errorf("exit code = %d, want 2 (ErrConfig)", code)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["code"] != "CONFIG_ERROR" {
		t.Errorf("code = %v, want CONFIG_ERROR", got["code"])
	}
}

func TestWriteError_Nil(t *testing.T) {
	var buf bytes.Buffer
	code := WriteError(&buf, nil)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil error, got: %q", buf.String())
	}
}

func TestDetailsRoundTrip(t *testing.T) {
	original := NewCLIError(ErrRateLimit, "slow down").
		WithDetails("retry_after_ms", float64(5000)).
		WithDetails("endpoint", "/api/order")

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored CLIError
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.Code != ErrRateLimit {
		t.Errorf("code = %v, want RATE_LIMIT", restored.Code)
	}
	if restored.Details["endpoint"] != "/api/order" {
		t.Errorf("endpoint = %v, want %q", restored.Details["endpoint"], "/api/order")
	}
	// JSON numbers unmarshal as float64
	if restored.Details["retry_after_ms"] != float64(5000) {
		t.Errorf("retry_after_ms = %v, want 5000", restored.Details["retry_after_ms"])
	}
}
