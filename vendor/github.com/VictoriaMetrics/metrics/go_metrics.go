package metrics

import (
	"fmt"
	"io"
	"log"
	"math"
	"runtime"
	runtimemetrics "runtime/metrics"
	"strings"

	"github.com/valyala/histogram"
)

// See https://pkg.go.dev/runtime/metrics#hdr-Supported_metrics
var runtimeMetrics = [][2]string{
	{"/sched/latencies:seconds", "go_sched_latencies_seconds"},
	{"/sync/mutex/wait/total:seconds", "go_mutex_wait_seconds_total"},
	{"/cpu/classes/gc/mark/assist:cpu-seconds", "go_gc_mark_assist_cpu_seconds_total"},
	{"/cpu/classes/gc/total:cpu-seconds", "go_gc_cpu_seconds_total"},
	{"/gc/pauses:seconds", "go_gc_pauses_seconds"},
	{"/cpu/classes/scavenge/total:cpu-seconds", "go_scavenge_cpu_seconds_total"},
	{"/gc/gomemlimit:bytes", "go_memlimit_bytes"},
}

var supportedRuntimeMetrics = initSupportedRuntimeMetrics(runtimeMetrics)

func initSupportedRuntimeMetrics(rms [][2]string) [][2]string {
	exposedMetrics := make(map[string]struct{})
	for _, d := range runtimemetrics.All() {
		exposedMetrics[d.Name] = struct{}{}
	}
	var supportedMetrics [][2]string
	for _, rm := range rms {
		metricName := rm[0]
		if _, ok := exposedMetrics[metricName]; ok {
			supportedMetrics = append(supportedMetrics, rm)
		} else {
			log.Printf("github.com/VictoriaMetrics/metrics: do not expose %s metric, since the corresponding metric %s isn't supported in the current Go runtime", rm[1], metricName)
		}
	}
	return supportedMetrics
}

func writeGoMetrics(w io.Writer) {
	writeRuntimeMetrics(w)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	WriteGaugeUint64(w, "go_memstats_alloc_bytes", ms.Alloc)
	WriteCounterUint64(w, "go_memstats_alloc_bytes_total", ms.TotalAlloc)
	WriteGaugeUint64(w, "go_memstats_buck_hash_sys_bytes", ms.BuckHashSys)
	WriteCounterUint64(w, "go_memstats_frees_total", ms.Frees)
	WriteGaugeFloat64(w, "go_memstats_gc_cpu_fraction", ms.GCCPUFraction)
	WriteGaugeUint64(w, "go_memstats_gc_sys_bytes", ms.GCSys)

	WriteGaugeUint64(w, "go_memstats_heap_alloc_bytes", ms.HeapAlloc)
	WriteGaugeUint64(w, "go_memstats_heap_idle_bytes", ms.HeapIdle)
	WriteGaugeUint64(w, "go_memstats_heap_inuse_bytes", ms.HeapInuse)
	WriteGaugeUint64(w, "go_memstats_heap_objects", ms.HeapObjects)
	WriteGaugeUint64(w, "go_memstats_heap_released_bytes", ms.HeapReleased)
	WriteGaugeUint64(w, "go_memstats_heap_sys_bytes", ms.HeapSys)
	WriteGaugeFloat64(w, "go_memstats_last_gc_time_seconds", float64(ms.LastGC)/1e9)
	WriteCounterUint64(w, "go_memstats_lookups_total", ms.Lookups)
	WriteCounterUint64(w, "go_memstats_mallocs_total", ms.Mallocs)
	WriteGaugeUint64(w, "go_memstats_mcache_inuse_bytes", ms.MCacheInuse)
	WriteGaugeUint64(w, "go_memstats_mcache_sys_bytes", ms.MCacheSys)
	WriteGaugeUint64(w, "go_memstats_mspan_inuse_bytes", ms.MSpanInuse)
	WriteGaugeUint64(w, "go_memstats_mspan_sys_bytes", ms.MSpanSys)
	WriteGaugeUint64(w, "go_memstats_next_gc_bytes", ms.NextGC)
	WriteGaugeUint64(w, "go_memstats_other_sys_bytes", ms.OtherSys)
	WriteGaugeUint64(w, "go_memstats_stack_inuse_bytes", ms.StackInuse)
	WriteGaugeUint64(w, "go_memstats_stack_sys_bytes", ms.StackSys)
	WriteGaugeUint64(w, "go_memstats_sys_bytes", ms.Sys)

	WriteCounterUint64(w, "go_cgo_calls_count", uint64(runtime.NumCgoCall()))
	WriteGaugeUint64(w, "go_cpu_count", uint64(runtime.NumCPU()))

	gcPauses := histogram.NewFast()
	for _, pauseNs := range ms.PauseNs[:] {
		gcPauses.Update(float64(pauseNs) / 1e9)
	}
	phis := []float64{0, 0.25, 0.5, 0.75, 1}
	quantiles := make([]float64, 0, len(phis))
	WriteMetadataIfNeeded(w, "go_gc_duration_seconds", "summary")
	for i, q := range gcPauses.Quantiles(quantiles[:0], phis) {
		fmt.Fprintf(w, `go_gc_duration_seconds{quantile="%g"} %g`+"\n", phis[i], q)
	}
	fmt.Fprintf(w, "go_gc_duration_seconds_sum %g\n", float64(ms.PauseTotalNs)/1e9)
	fmt.Fprintf(w, "go_gc_duration_seconds_count %d\n", ms.NumGC)

	WriteCounterUint64(w, "go_gc_forced_count", uint64(ms.NumForcedGC))

	WriteGaugeUint64(w, "go_gomaxprocs", uint64(runtime.GOMAXPROCS(0)))
	WriteGaugeUint64(w, "go_goroutines", uint64(runtime.NumGoroutine()))
	numThread, _ := runtime.ThreadCreateProfile(nil)
	WriteGaugeUint64(w, "go_threads", uint64(numThread))

	// Export build details.
	WriteMetadataIfNeeded(w, "go_info", "gauge")
	fmt.Fprintf(w, "go_info{version=%q} 1\n", runtime.Version())

	WriteMetadataIfNeeded(w, "go_info_ext", "gauge")
	fmt.Fprintf(w, "go_info_ext{compiler=%q, GOARCH=%q, GOOS=%q, GOROOT=%q} 1\n",
		runtime.Compiler, runtime.GOARCH, runtime.GOOS, runtime.GOROOT())
}

func writeRuntimeMetrics(w io.Writer) {
	samples := make([]runtimemetrics.Sample, len(supportedRuntimeMetrics))
	for i, rm := range supportedRuntimeMetrics {
		samples[i].Name = rm[0]
	}
	runtimemetrics.Read(samples)
	for i, rm := range supportedRuntimeMetrics {
		writeRuntimeMetric(w, rm[1], &samples[i])
	}
}

func writeRuntimeMetric(w io.Writer, name string, sample *runtimemetrics.Sample) {
	kind := sample.Value.Kind()
	switch kind {
	case runtimemetrics.KindBad:
		panic(fmt.Errorf("BUG: unexpected runtimemetrics.KindBad for sample.Name=%q", sample.Name))
	case runtimemetrics.KindUint64:
		v := sample.Value.Uint64()
		if strings.HasSuffix(name, "_total") {
			WriteCounterUint64(w, name, v)
		} else {
			WriteGaugeUint64(w, name, v)
		}
	case runtimemetrics.KindFloat64:
		v := sample.Value.Float64()
		if isCounterName(name) {
			WriteCounterFloat64(w, name, v)
		} else {
			WriteGaugeFloat64(w, name, v)
		}
	case runtimemetrics.KindFloat64Histogram:
		h := sample.Value.Float64Histogram()
		writeRuntimeHistogramMetric(w, name, h)
	default:
		panic(fmt.Errorf("unexpected metric kind=%d", kind))
	}
}

func writeRuntimeHistogramMetric(w io.Writer, name string, h *runtimemetrics.Float64Histogram) {
	buckets := h.Buckets
	counts := h.Counts
	if len(buckets) != len(counts)+1 {
		panic(fmt.Errorf("the number of buckets must be bigger than the number of counts by 1 in histogram %s; got buckets=%d, counts=%d", name, len(buckets), len(counts)))
	}
	tailCount := uint64(0)
	if strings.HasSuffix(name, "_seconds") {
		// Limit the maximum bucket to 1 second, since Go runtime exposes buckets with 10K seconds,
		// which have little sense. At the same time such buckets may lead to high cardinality issues
		// at the scraper side.
		for len(buckets) > 0 && buckets[len(buckets)-1] > 1 {
			buckets = buckets[:len(buckets)-1]
			tailCount += counts[len(counts)-1]
			counts = counts[:len(counts)-1]
		}
	}

	iStep := float64(len(buckets)) / maxRuntimeHistogramBuckets

	totalCount := uint64(0)
	iNext := 0.0
	WriteMetadataIfNeeded(w, name, "histogram")
	for i, count := range counts {
		totalCount += count
		if float64(i) >= iNext {
			iNext += iStep
			le := buckets[i+1]
			if !math.IsInf(le, 1) {
				fmt.Fprintf(w, `%s_bucket{le="%g"} %d`+"\n", name, le, totalCount)
			}
		}
	}
	totalCount += tailCount
	fmt.Fprintf(w, `%s_bucket{le="+Inf"} %d`+"\n", name, totalCount)
	// _sum and _count are not exposed because the Go runtime histogram lacks accurate sum data.
	// Estimating the sum (as Prometheus does) could be misleading,  while exposing only `_count` without `_sum` is impractical.
	// We can reconsider if precise sum data becomes available.
	//
	// References:
	// - Go runtime histogram: https://github.com/golang/go/blob/3432c68467d50ffc622fed230a37cd401d82d4bf/src/runtime/metrics/histogram.go#L8
	// - Prometheus estimate: https://github.com/prometheus/client_golang/blob/5fe1d33cea76068edd4ece5f58e52f81d225b13c/prometheus/go_collector_latest.go#L498
	// - Related discussion: https://github.com/VictoriaMetrics/metrics/issues/94
}

// Limit the number of buckets for Go runtime histograms in order to prevent from high cardinality issues at scraper side.
const maxRuntimeHistogramBuckets = 30
