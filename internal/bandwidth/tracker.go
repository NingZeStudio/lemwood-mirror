package bandwidth

import (
	"sync"
	"sync/atomic"
	"time"
)

const measurementWindow = 10 * time.Second

type sample struct {
	at    time.Time
	bytes int64
}

// Tracker records bytes written by download responses and exposes a short
// rolling bandwidth estimate. It intentionally stays in memory because it is
// an operational status signal, not historical accounting.
type Tracker struct {
	peakMbps int64

	mu      sync.Mutex
	samples []sample
	active  int64
	total   int64
	peakBps int64
}

type Status struct {
	PeakBandwidthMbps       int64   `json:"peak_bandwidth_mbps"`
	CurrentBandwidthMbps    float64 `json:"current_bandwidth_mbps"`
	CurrentBandwidthBPS     int64   `json:"current_bandwidth_bps"`
	PeakObservedMbps        float64 `json:"peak_observed_mbps"`
	UtilizationPercent      float64 `json:"utilization_percent"`
	ActiveDownloads         int64   `json:"active_downloads"`
	TotalBytesServed        int64   `json:"total_bytes_served"`
	MeasurementWindowSecond int64   `json:"measurement_window_seconds"`
	UpdatedAt               string  `json:"updated_at"`
}

func NewTracker(peakMbps int64) *Tracker {
	if peakMbps <= 0 {
		peakMbps = 200
	}
	return &Tracker{peakMbps: peakMbps}
}

func (t *Tracker) StartDownload() {
	if t != nil {
		atomic.AddInt64(&t.active, 1)
	}
}

func (t *Tracker) FinishDownload() {
	if t == nil {
		return
	}
	for {
		current := atomic.LoadInt64(&t.active)
		if current <= 0 {
			return
		}
		if atomic.CompareAndSwapInt64(&t.active, current, current-1) {
			return
		}
	}
}

func (t *Tracker) RecordBytes(n int64) {
	if t == nil || n <= 0 {
		return
	}
	now := time.Now()
	atomic.AddInt64(&t.total, n)
	t.mu.Lock()
	if len(t.samples) == 0 || now.Sub(t.samples[len(t.samples)-1].at) >= time.Second {
		t.samples = append(t.samples, sample{at: now, bytes: n})
	} else {
		t.samples[len(t.samples)-1].bytes += n
	}
	t.pruneLocked(now)
	t.mu.Unlock()
}

func (t *Tracker) Snapshot() Status {
	if t == nil {
		return Status{}
	}
	now := time.Now()
	t.mu.Lock()
	t.pruneLocked(now)
	var bytes int64
	for _, s := range t.samples {
		bytes += s.bytes
	}
	var elapsed time.Duration
	if len(t.samples) > 0 {
		elapsed = now.Sub(t.samples[0].at)
	}
	t.mu.Unlock()
	if elapsed <= 0 {
		elapsed = time.Second
	}
	bps := int64(float64(bytes) / elapsed.Seconds())
	for {
		peak := atomic.LoadInt64(&t.peakBps)
		if bps <= peak || atomic.CompareAndSwapInt64(&t.peakBps, peak, bps) {
			break
		}
	}
	currentMbps := float64(bps*8) / 1_000_000
	peakObservedMbps := float64(atomic.LoadInt64(&t.peakBps)*8) / 1_000_000
	utilization := currentMbps / float64(t.peakMbps) * 100
	if utilization > 100 {
		utilization = 100
	}
	return Status{
		PeakBandwidthMbps:       t.peakMbps,
		CurrentBandwidthMbps:    currentMbps,
		CurrentBandwidthBPS:     bps,
		PeakObservedMbps:        peakObservedMbps,
		UtilizationPercent:      utilization,
		ActiveDownloads:         atomic.LoadInt64(&t.active),
		TotalBytesServed:        atomic.LoadInt64(&t.total),
		MeasurementWindowSecond: int64(measurementWindow / time.Second),
		UpdatedAt:               now.UTC().Format(time.RFC3339),
	}
}

func (t *Tracker) pruneLocked(now time.Time) {
	cutoff := now.Add(-measurementWindow)
	first := 0
	for first < len(t.samples) && t.samples[first].at.Before(cutoff) {
		first++
	}
	if first > 0 {
		t.samples = append([]sample(nil), t.samples[first:]...)
	}
}
