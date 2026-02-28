package client

import (
	"io"
	"time"
)

// Option configures the Client. Use With* functions to create options.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout. Default is 10 seconds.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithRetries sets the maximum number of retries for transient (5xx) errors.
// Default is 2. Only 5xx responses are retried; 4xx are never retried.
func WithRetries(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// WithWeightTracker attaches a WeightTracker for client-side rate limit awareness.
func WithWeightTracker(wt *WeightTracker) Option {
	return func(c *Client) {
		c.weightTracker = wt
	}
}

// WithWarnWriter sets the writer for rate limit warnings (typically os.Stderr).
// If nil, warnings are suppressed (used with --quiet).
func WithWarnWriter(w io.Writer) Option {
	return func(c *Client) {
		c.warnWriter = w
	}
}
