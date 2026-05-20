package traffic

import (
	"testing"

	"lemwood_mirror/internal/config"
	"lemwood_mirror/internal/db"
)

const (
	testGB = int64(1024 * 1024 * 1024)
	testMB = int64(1024 * 1024)
)

func setupTrackerTest(t *testing.T, limitGB int) *Tracker {
	t.Helper()

	base := t.TempDir()
	if db.DB != nil {
		_ = db.DB.Close()
		db.DB = nil
	}
	if err := db.InitDB(base, &config.Config{}); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	InitTracker(limitGB, "banned_ips.txt", "test-contact", base)

	t.Cleanup(func() {
		CloseTracker()
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
	})

	return GetTracker()
}

func TestEstimateTransferBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fileSize    int64
		rangeHeader string
		expected    int64
	}{
		{name: "full file without range", fileSize: 1000, rangeHeader: "", expected: 1000},
		{name: "single range", fileSize: 1000, rangeHeader: "bytes=0-99", expected: 100},
		{name: "open ended range", fileSize: 1000, rangeHeader: "bytes=500-", expected: 500},
		{name: "suffix range", fileSize: 1000, rangeHeader: "bytes=-200", expected: 200},
		{name: "invalid range falls back to full file", fileSize: 1000, rangeHeader: "bytes=abc-def", expected: 1000},
		{name: "multi range falls back to full file", fileSize: 1000, rangeHeader: "bytes=0-99,200-299", expected: 1000},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := EstimateTransferBytes(tc.fileSize, tc.rangeHeader); got != tc.expected {
				t.Fatalf("EstimateTransferBytes(%d, %q) = %d, want %d", tc.fileSize, tc.rangeHeader, got, tc.expected)
			}
		})
	}
}

func TestReserveTrafficRejectsWhenProjectedExceedsLimit(t *testing.T) {
	tracker := setupTrackerTest(t, 1)
	ip := "127.0.0.1"

	if err := db.RecordTraffic(ip, testGB); err != nil {
		t.Fatalf("RecordTraffic() error = %v", err)
	}

	allowed, currentBytes, projectedBytes, reason := tracker.ReserveTraffic(ip, 128)
	if allowed {
		t.Fatal("ReserveTraffic() allowed request unexpectedly")
	}
	if currentBytes != testGB {
		t.Fatalf("currentBytes = %d, want %d", currentBytes, testGB)
	}
	if projectedBytes != testGB+128 {
		t.Fatalf("projectedBytes = %d, want %d", projectedBytes, testGB+128)
	}
	if reason == "" {
		t.Fatal("ReserveTraffic() reason should not be empty")
	}
}

func TestReserveTrafficCountsPendingBytesUntilFinalize(t *testing.T) {
	tracker := setupTrackerTest(t, 1)
	ip := "127.0.0.1"

	firstEstimate := int64(700 * testMB)
	secondEstimate := int64(400 * testMB)

	allowed, _, _, reason := tracker.ReserveTraffic(ip, firstEstimate)
	if !allowed {
		t.Fatalf("first ReserveTraffic() rejected unexpectedly: %s", reason)
	}

	allowed, _, _, reason = tracker.ReserveTraffic(ip, secondEstimate)
	if allowed {
		t.Fatal("second ReserveTraffic() should be rejected because pending bytes already occupy the quota")
	}
	if reason == "" {
		t.Fatal("second ReserveTraffic() reason should not be empty")
	}

	banned, _, _, err := tracker.FinalizeTraffic(ip, firstEstimate, 128)
	if err != nil {
		t.Fatalf("FinalizeTraffic() error = %v", err)
	}
	if banned {
		t.Fatal("FinalizeTraffic() banned unexpectedly")
	}

	allowed, _, _, reason = tracker.ReserveTraffic(ip, secondEstimate)
	if !allowed {
		t.Fatalf("ReserveTraffic() should succeed after pending bytes are released: %s", reason)
	}
}
