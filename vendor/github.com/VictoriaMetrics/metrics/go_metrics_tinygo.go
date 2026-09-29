//go:build tinygo

package metrics

import (
	"io"
	"runtime"
)

// writeGoMetrics is a minimal stand-in for the native go_metrics.go under
// TinyGo, whose runtime provides neither the full runtime.MemStats struct
// (BuckHashSys, GCCPUFraction, NumGC, PauseNs, ...) nor the runtime/metrics
// and runtime.ThreadCreateProfile APIs the native collector reads. Only the
// goroutine count is exported; everything else in the package (counters,
// gauges, histograms, summaries, push) works unchanged.
func writeGoMetrics(w io.Writer) {
	WriteGaugeUint64(w, "go_goroutines", uint64(runtime.NumGoroutine()))
}
