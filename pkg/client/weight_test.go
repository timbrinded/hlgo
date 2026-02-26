package client

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func newTestTracker(now time.Time) *WeightTracker {
	wt := NewWeightTracker()
	wt.nowFunc = func() time.Time { return now }
	return wt
}

func TestWeightTracker_Record_And_CurrentUsage(t *testing.T) {
	now := time.Now()
	wt := newTestTracker(now)

	wt.Record(20)
	wt.Record(20)

	if got := wt.CurrentUsage(); got != 40 {
		t.Errorf("CurrentUsage() = %d, want 40", got)
	}
}

func TestWeightTracker_SlidingWindow(t *testing.T) {
	now := time.Now()
	wt := newTestTracker(now)

	// Record some weight in the past.
	wt.Record(100)

	// Advance time past the window.
	wt.nowFunc = func() time.Time { return now.Add(61 * time.Second) }

	if got := wt.CurrentUsage(); got != 0 {
		t.Errorf("CurrentUsage() after window = %d, want 0", got)
	}
}

func TestWeightTracker_ShouldWarn_BelowThreshold(t *testing.T) {
	now := time.Now()
	wt := newTestTracker(now)

	// 80% of 1200 = 960. Record 959.
	for range 47 {
		wt.Record(20)
	}
	wt.Record(19)

	if wt.ShouldWarn() {
		t.Error("ShouldWarn() = true, want false (usage=959, threshold=960)")
	}
}

func TestWeightTracker_ShouldWarn_AboveThreshold(t *testing.T) {
	now := time.Now()
	wt := newTestTracker(now)

	// Record exactly 961 (above 960 threshold).
	for range 48 {
		wt.Record(20)
	}
	wt.Record(1)

	if !wt.ShouldWarn() {
		t.Error("ShouldWarn() = false, want true (usage=961)")
	}
}

func TestWeightTracker_WarningJSON(t *testing.T) {
	now := time.Now()
	wt := newTestTracker(now)

	// Below threshold — no warning.
	wt.Record(20)
	if warning := wt.WarningJSON(); warning != nil {
		t.Errorf("expected nil warning below threshold, got %s", string(warning))
	}

	// Exceed threshold.
	for range 48 {
		wt.Record(20)
	}

	warning := wt.WarningJSON()
	if warning == nil {
		t.Fatal("expected warning above threshold")
	}

	var msg map[string]any
	if err := json.Unmarshal(warning, &msg); err != nil {
		t.Fatalf("failed to unmarshal warning: %v", err)
	}
	if msg["warning"] != "rate_limit_approaching" {
		t.Errorf("warning = %v, want rate_limit_approaching", msg["warning"])
	}
}

func TestWeightTracker_ConcurrentSafety(t *testing.T) {
	now := time.Now()
	wt := newTestTracker(now)

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			wt.Record(1)
			_ = wt.CurrentUsage()
			_ = wt.ShouldWarn()
		})
	}
	wg.Wait()

	if got := wt.CurrentUsage(); got != 100 {
		t.Errorf("CurrentUsage() = %d, want 100", got)
	}
}

func TestWeightForInfoType(t *testing.T) {
	tests := []struct {
		infoType string
		want     int
	}{
		{"l2Book", 2},
		{"allMids", 2},
		{"clearinghouseState", 2},
		{"orderStatus", 2},
		{"spotClearinghouseState", 2},
		{"userRole", 60},
		{"meta", 20},
		{"userFills", 20},
		{"unknown", 20},
	}
	for _, tt := range tests {
		if got := WeightForInfoType(tt.infoType); got != tt.want {
			t.Errorf("WeightForInfoType(%q) = %d, want %d", tt.infoType, got, tt.want)
		}
	}
}

func TestWeightForExchangeBatch(t *testing.T) {
	tests := []struct {
		batchLen int
		want     int
	}{
		{1, 1},
		{39, 1},
		{40, 2},
		{80, 3},
	}
	for _, tt := range tests {
		if got := WeightForExchangeBatch(tt.batchLen); got != tt.want {
			t.Errorf("WeightForExchangeBatch(%d) = %d, want %d", tt.batchLen, got, tt.want)
		}
	}
}
