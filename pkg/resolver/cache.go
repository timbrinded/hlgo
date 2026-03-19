package resolver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// cacheEntry wraps the raw JSON payload with a timestamp for TTL checks.
type cacheEntry struct {
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// diskCache provides per-network disk storage for metadata JSON responses.
// Each file stores a cacheEntry with a timestamp for TTL-based expiration.
type diskCache struct {
	dir string // e.g. ~/.hlgo/cache/mainnet
}

// newDiskCache creates a diskCache rooted at dir.
func newDiskCache(dir string) *diskCache {
	return &diskCache{dir: dir}
}

// read loads a cache file and returns its data if the entry is fresh (within ttl).
// Returns (nil, false) if the file doesn't exist, is corrupt, or is expired.
func (c *diskCache) read(filename string, ttl time.Duration, now time.Time) ([]byte, bool) {
	path := filepath.Join(c.dir, filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry cacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, false
	}

	if now.Sub(entry.Timestamp) > ttl {
		return nil, false
	}

	return entry.Data, true
}

// write persists data to a cache file with the given timestamp.
// Errors are silently ignored — cache writes are best-effort.
func (c *diskCache) write(filename string, data []byte, now time.Time) {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return
	}

	entry := cacheEntry{Timestamp: now, Data: data}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}

	path := filepath.Join(c.dir, filename)
	//nolint:errcheck // best-effort cache write; failure is non-fatal
	os.WriteFile(path, raw, 0o600)
}
