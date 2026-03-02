// Package client provides the HTTP client for Hyperliquid Info and Exchange APIs.
//
// All requests are POST with JSON bodies. Responses are decoded with
// json.Decoder.UseNumber() to preserve financial precision (never float64).
// Transient errors (5xx and 429 rate limits) are retried with quadratic backoff;
// other 4xx errors fail immediately.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/timbrinded/hlgo/pkg/output"
)

const (
	defaultTimeout    = 10 * time.Second
	defaultMaxRetries = 2

	// baseBackoff is the base unit for quadratic backoff: attempt^2 * baseBackoff.
	baseBackoff = 100 * time.Millisecond
)

// Client is the HTTP client for Hyperliquid's Info and Exchange APIs.
// It is safe for concurrent use, but per SOUL.md the typical lifecycle is
// create -> call -> discard within a single CLI command.
type Client struct {
	baseURL       string
	httpClient    http.Client
	maxRetries    int
	weightTracker *WeightTracker
	warnWriter    io.Writer
}

// NewClient creates a Client targeting baseURL (e.g. "https://api.hyperliquid.xyz").
// Functional options configure timeout, retries, and other behaviour.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: http.Client{
			Timeout: defaultTimeout,
		},
		maxRetries: defaultMaxRetries,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SignatureWire is the structured signature format expected by the /exchange endpoint.
// Each ECDSA component (r, s, v) is sent as a separate JSON field.
type SignatureWire struct {
	R string `json:"r"`
	S string `json:"s"`
	V int    `json:"v"`
}

// exchangeRequest is the envelope for POST /exchange.
type exchangeRequest struct {
	Action       any           `json:"action"`
	Nonce        int64         `json:"nonce"`
	Signature    SignatureWire `json:"signature"`
	VaultAddress string        `json:"vaultAddress,omitempty"`
	ExpiresAfter *int64        `json:"expiresAfter,omitempty"`
}

// PostInfo sends a request to the /info endpoint and returns the raw JSON response.
// The request body is marshalled as-is (e.g. {"type": "allMids"}).
func (c *Client) PostInfo(ctx context.Context, request any) (json.RawMessage, error) {
	return c.doPost(ctx, "/info", request)
}

// PostExchange sends a signed action to the /exchange endpoint.
// The action, nonce, and signature are wrapped in the standard envelope.
// vaultAddress is included only when non-empty.
func (c *Client) PostExchange(ctx context.Context, action any, nonce int64, signature SignatureWire, vaultAddress string, expiresAfter *int64) (json.RawMessage, error) {
	body := exchangeRequest{
		Action:       action,
		Nonce:        nonce,
		Signature:    signature,
		VaultAddress: vaultAddress,
		ExpiresAfter: expiresAfter,
	}
	raw, err := c.doPost(ctx, "/exchange", body)
	if err != nil {
		return nil, err
	}
	if err := validateExchangeResponse(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func validateExchangeResponse(raw json.RawMessage) error {
	var envelope struct {
		Status   string          `json:"status"`
		Response json.RawMessage `json:"response"`
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&envelope); err != nil {
		return output.NewCLIError(output.ErrAPI, "failed to decode exchange response envelope").
			WithDetails("path", "/exchange").
			WithDetails("cause", err.Error())
	}

	if !strings.EqualFold(envelope.Status, "err") {
		return validateExchangeStatuses(envelope.Status, envelope.Response)
	}

	cliErr := output.NewCLIError(output.ErrAPI, "exchange returned error status").
		WithDetails("path", "/exchange").
		WithDetails("exchange_status", envelope.Status)

	if len(envelope.Response) > 0 {
		var responseMessage string
		if err := json.Unmarshal(envelope.Response, &responseMessage); err == nil {
			cliErr.Message = "exchange error: " + responseMessage
			cliErr = cliErr.WithDetails("exchange_response", responseMessage)
		} else {
			cliErr = cliErr.WithDetails("exchange_response", string(envelope.Response))
		}
	}

	return cliErr
}

func validateExchangeStatuses(status string, response json.RawMessage) error {
	var payload struct {
		Data struct {
			Statuses []json.RawMessage `json:"statuses"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response, &payload); err != nil {
		return nil
	}
	if len(payload.Data.Statuses) == 0 {
		return nil
	}

	var errs []string
	for _, entryRaw := range payload.Data.Statuses {
		var asString string
		if err := json.Unmarshal(entryRaw, &asString); err == nil {
			asString = strings.TrimSpace(asString)
			if strings.EqualFold(asString, "success") || asString == "" {
				continue
			}
			errs = append(errs, asString)
			continue
		}

		var asObject map[string]json.RawMessage
		if err := json.Unmarshal(entryRaw, &asObject); err != nil {
			continue
		}

		rawErr, ok := asObject["error"]
		if !ok {
			continue
		}

		var msg string
		if err := json.Unmarshal(rawErr, &msg); err == nil && strings.TrimSpace(msg) != "" {
			errs = append(errs, msg)
			continue
		}
		errs = append(errs, string(rawErr))
	}

	if len(errs) == 0 {
		return nil
	}

	return output.NewCLIError(output.ErrAPI, "exchange action returned error statuses").
		WithDetails("path", "/exchange").
		WithDetails("exchange_status", status).
		WithDetails("exchange_errors", errs)
}

// doPost performs a POST request with retry logic for transient errors.
// It marshals body to JSON, sends it, and decodes the response using UseNumber.
func (c *Client) doPost(ctx context.Context, path string, body any) (json.RawMessage, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "failed to marshal request body").
			WithDetails("path", path).
			WithDetails("cause", err.Error())
	}

	url := c.baseURL + path
	var lastErr error

	for attempt := range c.maxRetries + 1 {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * baseBackoff
			select {
			case <-ctx.Done():
				return nil, output.NewCLIError(output.ErrNetwork, "request cancelled during retry backoff").
					WithDetails("path", path).
					WithDetails("attempt", attempt).
					WithDetails("cause", ctx.Err().Error())
			case <-time.After(backoff):
			}
		}

		result, err := c.executeRequest(ctx, url, path, payload)
		if err == nil {
			c.recordWeight(path, payload)
			return result, nil
		}

		// Only retry on transient errors (5xx, 429); other 4xx fail immediately.
		if !isRetryable(err) {
			return nil, err
		}
		lastErr = err
	}

	return nil, lastErr
}

// executeRequest performs a single HTTP POST and interprets the response.
func (c *Client) executeRequest(ctx context.Context, url, path string, payload []byte) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, output.NewCLIError(output.ErrNetwork, "failed to create HTTP request").
			WithDetails("path", path).
			WithDetails("cause", err.Error())
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, output.NewCLIError(output.ErrNetwork, "HTTP request failed").
			WithDetails("path", path).
			WithDetails("cause", err.Error())
	}
	//nolint:errcheck // best-effort close on read-only response body; error is non-actionable
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, output.NewCLIError(output.ErrNetwork, "failed to read response body").
			WithDetails("path", path).
			WithDetails("status_code", resp.StatusCode).
			WithDetails("cause", err.Error())
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &retryableError{
			err: output.NewCLIError(output.ErrRateLimit, "rate limited by API").
				WithDetails("path", path).
				WithDetails("status_code", resp.StatusCode),
		}
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, output.NewCLIError(output.ErrAPI, fmt.Sprintf("API error: %s", string(respBody))).
			WithDetails("path", path).
			WithDetails("status_code", resp.StatusCode)
	}

	if resp.StatusCode >= 500 {
		return nil, &retryableError{
			err: output.NewCLIError(output.ErrAPI, fmt.Sprintf("server error: %s", string(respBody))).
				WithDetails("path", path).
				WithDetails("status_code", resp.StatusCode),
		}
	}

	// Decode with UseNumber to preserve financial precision.
	var result json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(respBody))
	dec.UseNumber()
	if err := dec.Decode(&result); err != nil {
		return nil, output.NewCLIError(output.ErrAPI, "failed to decode response JSON").
			WithDetails("path", path).
			WithDetails("status_code", resp.StatusCode).
			WithDetails("cause", err.Error()).
			WithDetails("body", string(respBody))
	}

	return result, nil
}

// retryableError wraps an error to signal that the request may be retried.
type retryableError struct {
	err error
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// isRetryable reports whether err signals a transient failure worth retrying.
func isRetryable(err error) bool {
	var re *retryableError
	return errors.As(err, &re)
}

// recordWeight records API weight and emits a warning if approaching the limit.
func (c *Client) recordWeight(path string, payload []byte) {
	if c.weightTracker == nil {
		return
	}

	var weight int
	switch path {
	case "/info":
		var req struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(payload, &req) == nil {
			weight = WeightForInfoType(req.Type)
		}
	case "/exchange":
		weight = WeightForExchangeBatch(1)
	}

	if weight > 0 {
		c.weightTracker.Record(weight)
	}

	if c.warnWriter != nil && c.weightTracker.ShouldWarn() {
		if warning := c.weightTracker.WarningJSON(); warning != nil {
			//nolint:errcheck // best-effort warning; stderr write failure is non-fatal
			c.warnWriter.Write(append(warning, '\n'))
		}
	}
}
