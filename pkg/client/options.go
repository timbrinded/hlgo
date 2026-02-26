package client

import "time"

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
