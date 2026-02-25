package output

import (
	"encoding/json"
	"errors"
	"io"
)

// ErrorCode is a machine-readable error classification.
// Agents branch on these codes without parsing human-readable messages.
type ErrorCode string

const (
	ErrValidation ErrorCode = "VALIDATION_ERROR"
	ErrSigning    ErrorCode = "SIGNING_ERROR"
	ErrAPI        ErrorCode = "API_ERROR"
	ErrRateLimit  ErrorCode = "RATE_LIMIT"
	ErrNetwork    ErrorCode = "NETWORK_ERROR"
	ErrConfig     ErrorCode = "CONFIG_ERROR"
)

// exitCodes maps each ErrorCode to a distinct process exit code.
// Agents can branch on exit codes without parsing stderr JSON.
var exitCodes = map[ErrorCode]int{
	ErrValidation: 1,
	ErrConfig:     2,
	ErrNetwork:    3,
	ErrAPI:        4,
	ErrSigning:    5,
	ErrRateLimit:  6,
}

// CLIError is the structured error type returned by all hlgo commands.
// It serializes to JSON on stderr per SOUL.md: {"error": "...", "code": "...", "details": {...}}.
type CLIError struct {
	Message string         `json:"error"`
	Code    ErrorCode      `json:"code"`
	Details map[string]any `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *CLIError) Error() string {
	return e.Message
}

// ExitCode returns the process exit code for this error's code.
// Unknown codes default to 1.
func (e *CLIError) ExitCode() int {
	if code, ok := exitCodes[e.Code]; ok {
		return code
	}
	return 1
}

// WithDetails adds a key-value pair to the error's details map.
// It returns the receiver for fluent chaining.
func (e *CLIError) WithDetails(key string, value any) *CLIError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// NewCLIError creates a new CLIError with the given code and message.
func NewCLIError(code ErrorCode, message string) *CLIError {
	return &CLIError{
		Code:    code,
		Message: message,
	}
}

// WriteError serializes err as structured JSON to w and returns the exit code.
// If err is (or wraps) a *CLIError, its code and details are preserved.
// Otherwise, the error is wrapped as ErrAPI.
func WriteError(w io.Writer, err error) int {
	if err == nil {
		return 0
	}

	var cliErr *CLIError
	if !errors.As(err, &cliErr) {
		cliErr = NewCLIError(ErrAPI, err.Error())
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	//nolint:errcheck // best-effort; stderr write failure is unrecoverable
	enc.Encode(cliErr)

	return cliErr.ExitCode()
}
