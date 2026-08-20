package bandwidth

import "testing"

func TestNewTrackerDefaultsPeakBandwidth(t *testing.T) {
	status := NewTracker(0).Snapshot()
	if status.PeakBandwidthMbps != 200 {
		t.Fatalf("peak bandwidth = %d, want 200", status.PeakBandwidthMbps)
	}
}

func TestTrackerRecordsBytesAndDownloads(t *testing.T) {
	tracker := NewTracker(200)
	tracker.StartDownload()
	tracker.RecordBytes(25_000_000)
	status := tracker.Snapshot()
	if status.TotalBytesServed != 25_000_000 {
		t.Fatalf("total bytes = %d, want 25000000", status.TotalBytesServed)
	}
	if status.ActiveDownloads != 1 {
		t.Fatalf("active downloads = %d, want 1", status.ActiveDownloads)
	}
	if status.CurrentBandwidthMbps <= 0 {
		t.Fatalf("current bandwidth should be positive: %+v", status)
	}
	tracker.FinishDownload()
	if got := tracker.Snapshot().ActiveDownloads; got != 0 {
		t.Fatalf("active downloads after finish = %d, want 0", got)
	}
}
