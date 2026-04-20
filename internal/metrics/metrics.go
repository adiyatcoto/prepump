// Package metrics provides Prometheus-compatible metrics
package metrics

import (
	"sync/atomic"
	"time"
)

// Metrics holds runtime statistics
type Metrics struct {
	// Counters
	RequestsTotal    atomic.Int64
	ErrorsTotal      atomic.Int64
	ScansCompleted   atomic.Int64
	PythUpdates      atomic.Int64
	
	// Gauges
	ActiveCoins      atomic.Int64
	PumpScoreMax     atomic.Int64 // scaled by 100
	WSConns          atomic.Int64
	
	// Latencies (ring buffer for histogram approximation)
	scanLatencies    *ringBuffer
	pythLatencies    *ringBuffer
}

// NewMetrics creates a metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		scanLatencies: newRingBuffer(100),
		pythLatencies: newRingBuffer(100),
	}
}

// RecordScan captures scan duration
func (m *Metrics) RecordScan(d time.Duration) {
	m.ScansCompleted.Add(1)
	m.scanLatencies.Push(int64(d.Milliseconds()))
}

// RecordPythUpdate captures Pyth update latency
func (m *Metrics) RecordPythUpdate(d time.Duration) {
	m.PythUpdates.Add(1)
	m.pythLatencies.Push(int64(d.Milliseconds()))
}

// RecordError increments error counter
func (m *Metrics) RecordError() {
	m.ErrorsTotal.Add(1)
}

// Snapshot returns current metrics as map
func (m *Metrics) Snapshot() map[string]int64 {
	return map[string]int64{
		"requests_total":   m.RequestsTotal.Load(),
		"errors_total":     m.ErrorsTotal.Load(),
		"scans_completed":  m.ScansCompleted.Load(),
		"pyth_updates":     m.PythUpdates.Load(),
		"active_coins":     m.ActiveCoins.Load(),
		"pump_score_max":   m.PumpScoreMax.Load(),
		"ws_conns":         m.WSConns.Load(),
	}
}

// ringBuffer for latency tracking
type ringBuffer struct {
	size int
	buf  []int64
	idx  atomic.Int64
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		size: size,
		buf:  make([]int64, size),
	}
}

func (r *ringBuffer) Push(v int64) {
	i := r.idx.Add(1) % int64(r.size)
	r.buf[i] = v
}

func (r *ringBuffer) Avg() int64 {
	if r.idx.Load() == 0 {
		return 0
	}
	var sum int64
	for _, v := range r.buf {
		sum += v
	}
	if r.idx.Load() < int64(r.size) {
		return sum / r.idx.Load()
	}
	return sum / int64(r.size)
}

// HealthStatus represents service health
type HealthStatus struct {
	Healthy bool
	Checks  map[string]string
}

// HealthChecker provides health checks
type HealthChecker struct {
	m        *Metrics
	startTime time.Time
}

// NewHealthChecker creates a health checker
func NewHealthChecker(m *Metrics) *HealthChecker {
	return &HealthChecker{
		m:         m,
		startTime: time.Now(),
	}
}

// Check returns current health status
func (h *HealthChecker) Check() HealthStatus {
	checks := make(map[string]string)
	healthy := true
	
	// Check uptime
	uptime := time.Since(h.startTime)
	checks["uptime"] = uptime.String()
	
	// Check error rate
	reqs := h.m.RequestsTotal.Load()
	errs := h.m.ErrorsTotal.Load()
	if reqs > 0 && float64(errs)/float64(reqs) > 0.1 {
		checks["error_rate"] = "WARNING: >10% errors"
		healthy = false
	} else {
		checks["error_rate"] = "OK"
	}
	
	// Check scan freshness
	if h.m.ScansCompleted.Load() == 0 {
		checks["scan_freshness"] = "WARNING: no scans completed"
		healthy = false
	} else {
		checks["scan_freshness"] = "OK"
	}
	
	return HealthStatus{
		Healthy: healthy,
		Checks:  checks,
	}
}
