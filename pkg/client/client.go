// Package client provides the HTTP client for Hyperliquid Info and Exchange APIs.
//
// All requests are POST with JSON bodies. Responses are decoded with
// json.Decoder.UseNumber() to preserve financial precision (never float64).
// Transient 5xx errors are retried with quadratic backoff; 4xx errors fail immediately.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	baseURL    string
	httpClient http.Client
	maxRetries int
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

// exchangeRequest is the envelope for POST /exchange.
type exchangeRequest struct {
	Action       any    `json:"action"`
	Nonce        int64  `json:"nonce"`
	Signature    string `json:"signature"`
	VaultAddress string `json:"vaultAddress,omitempty"`
}

// PostInfo sends a request to the /info endpoint and returns the raw JSON response.
// The request body is marshalled as-is (e.g. {"type": "allMids"}).
func (c *Client) PostInfo(ctx context.Context, request any) (json.RawMessage, error) {
	return c.doPost(ctx, "/info", request)
}

// PostExchange sends a signed action to the /exchange endpoint.
// The action, nonce, and signature are wrapped in the standard envelope.
// vaultAddress is included only when non-empty.
func (c *Client) PostExchange(ctx context.Context, action any, nonce int64, signature string, vaultAddress string) (json.RawMessage, error) {
	body := exchangeRequest{
		Action:       action,
		Nonce:        nonce,
		Signature:    signature,
		VaultAddress: vaultAddress,
	}
	return c.doPost(ctx, "/exchange", body)
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
			return result, nil
		}

		// Only retry on 5xx; everything else fails immediately.
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
		return nil, output.NewCLIError(output.ErrRateLimit, "rate limited by API").
			WithDetails("path", path).
			WithDetails("status_code", resp.StatusCode)
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
