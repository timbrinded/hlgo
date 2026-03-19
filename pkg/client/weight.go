package client

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	defaultWeightLimit = 1200
	defaultWindow      = time.Minute
	warnThreshold      = 0.80
)

// weightEntry records a single API call's weight and timestamp.
type weightEntry struct {
	weight int
	at     time.Time
}

// WeightTracker tracks cumulative API request weight within a sliding window
// for client-side rate limit awareness.
type WeightTracker struct {
	mu      sync.Mutex
	entries []weightEntry
	limit   int
	window  time.Duration
	nowFunc func() time.Time
}

// NewWeightTracker creates a WeightTracker with default limit (1200) and window (1 minute).
func NewWeightTracker() *WeightTracker {
	return &WeightTracker{limit: defaultWeightLimit, window: defaultWindow, nowFunc: time.Now}
}

// Record adds a weight entry and prunes expired entries.
func (wt *WeightTracker) Record(weight int) {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	now := wt.nowFunc()
	wt.prune(now)
	wt.entries = append(wt.entries, weightEntry{weight: weight, at: now})
}

// CurrentUsage returns the total weight within the current window.
func (wt *WeightTracker) CurrentUsage() int {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	wt.prune(wt.nowFunc())
	total := 0
	for _, e := range wt.entries {
		total += e.weight
	}
	return total
}

// ShouldWarn returns true if current usage exceeds the warn threshold (80% of limit).
func (wt *WeightTracker) ShouldWarn() bool {
	return float64(wt.CurrentUsage()) > warnThreshold*float64(wt.limit)
}

// WarningJSON returns a structured warning message as JSON, or nil if no warning.
func (wt *WeightTracker) WarningJSON() json.RawMessage {
	wt.mu.Lock()
	defer wt.mu.Unlock()

	now := wt.nowFunc()
	wt.prune(now)

	total := 0
	for _, e := range wt.entries {
		total += e.weight
	}

	if float64(total) <= warnThreshold*float64(wt.limit) {
		return nil
	}

	oldest := now
	if len(wt.entries) > 0 {
		oldest = wt.entries[0].at
	}
	remaining := wt.window - now.Sub(oldest)
	remaining = max(remaining, 0)

	msg := map[string]any{"warning": "rate_limit_approaching", "current": total, "limit": wt.limit, "window_remaining": fmt.Sprintf("%.0fs", remaining.Seconds())}
	data, _ := json.Marshal(msg) //nolint:errcheck // msg is a fixed-shape map, cannot fail
	return data
}

// prune removes entries outside the sliding window.
func (wt *WeightTracker) prune(now time.Time) {
	cutoff := now.Add(-wt.window)
	i := 0
	for i < len(wt.entries) && wt.entries[i].at.Before(cutoff) {
		i++
	}
	if i > 0 {
		wt.entries = wt.entries[i:]
	}
}

// WeightForInfoType returns the API weight for a given info request type.
func WeightForInfoType(infoType string) int {
	switch infoType {
	case "l2Book", "allMids", "clearinghouseState", "orderStatus", "spotClearinghouseState":
		return 2
	case "userRole":
		return 60
	default:
		return 20
	}
}

// WeightForExchangeBatch returns the API weight for a batch exchange request.
func WeightForExchangeBatch(batchLen int) int {
	return 1 + batchLen/40
}
