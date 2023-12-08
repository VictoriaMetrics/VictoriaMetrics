package metrics

import (
	"fmt"
	"io"
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

func writeGoMetrics(w io.Writer) {
	writeRuntimeMetrics(w)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(w, "go_memstats_alloc_bytes %d\n", ms.Alloc)
	fmt.Fprintf(w, "go_memstats_alloc_bytes_total %d\n", ms.TotalAlloc)
	fmt.Fprintf(w, "go_memstats_buck_hash_sys_bytes %d\n", ms.BuckHashSys)
	fmt.Fprintf(w, "go_memstats_frees_total %d\n", ms.Frees)
	fmt.Fprintf(w, "go_memstats_gc_cpu_fraction %g\n", ms.GCCPUFraction)
	fmt.Fprintf(w, "go_memstats_gc_sys_bytes %d\n", ms.GCSys)

	fmt.Fprintf(w, "go_memstats_heap_alloc_bytes %d\n", ms.HeapAlloc)
	fmt.Fprintf(w, "go_memstats_heap_idle_bytes %d\n", ms.HeapIdle)
	fmt.Fprintf(w, "go_memstats_heap_inuse_bytes %d\n", ms.HeapInuse)
	fmt.Fprintf(w, "go_memstats_heap_objects %d\n", ms.HeapObjects)
	fmt.Fprintf(w, "go_memstats_heap_released_bytes %d\n", ms.HeapReleased)
	fmt.Fprintf(w, "go_memstats_heap_sys_bytes %d\n", ms.HeapSys)
	fmt.Fprintf(w, "go_memstats_last_gc_time_seconds %g\n", float64(ms.LastGC)/1e9)
	fmt.Fprintf(w, "go_memstats_lookups_total %d\n", ms.Lookups)
	fmt.Fprintf(w, "go_memstats_mallocs_total %d\n", ms.Mallocs)
	fmt.Fprintf(w, "go_memstats_mcache_inuse_bytes %d\n", ms.MCacheInuse)
	fmt.Fprintf(w, "go_memstats_mcache_sys_bytes %d\n", ms.MCacheSys)
	fmt.Fprintf(w, "go_memstats_mspan_inuse_bytes %d\n", ms.MSpanInuse)
	fmt.Fprintf(w, "go_memstats_mspan_sys_bytes %d\n", ms.MSpanSys)
	fmt.Fprintf(w, "go_memstats_next_gc_bytes %d\n", ms.NextGC)
	fmt.Fprintf(w, "go_memstats_other_sys_bytes %d\n", ms.OtherSys)
	fmt.Fprintf(w, "go_memstats_stack_inuse_bytes %d\n", ms.StackInuse)
	fmt.Fprintf(w, "go_memstats_stack_sys_bytes %d\n", ms.StackSys)
	fmt.Fprintf(w, "go_memstats_sys_bytes %d\n", ms.Sys)

	fmt.Fprintf(w, "go_cgo_calls_count %d\n", runtime.NumCgoCall())
	fmt.Fprintf(w, "go_cpu_count %d\n", runtime.NumCPU())

	gcPauses := histogram.NewFast()
	for _, pauseNs := range ms.PauseNs[:] {
		gcPauses.Update(float64(pauseNs) / 1e9)
	}
	phis := []float64{0, 0.25, 0.5, 0.75, 1}
	quantiles := make([]float64, 0, len(phis))
	for i, q := range gcPauses.Quantiles(quantiles[:0], phis) {
		fmt.Fprintf(w, `go_gc_duration_seconds{quantile="%g"} %g`+"\n", phis[i], q)
	}
	fmt.Fprintf(w, `go_gc_duration_seconds_sum %g`+"\n", float64(ms.PauseTotalNs)/1e9)
	fmt.Fprintf(w, `go_gc_duration_seconds_count %d`+"\n", ms.NumGC)
	fmt.Fprintf(w, `go_gc_forced_count %d`+"\n", ms.NumForcedGC)

	fmt.Fprintf(w, `go_gomaxprocs %d`+"\n", runtime.GOMAXPROCS(0))
	fmt.Fprintf(w, `go_goroutines %d`+"\n", runtime.NumGoroutine())
	numThread, _ := runtime.ThreadCreateProfile(nil)
	fmt.Fprintf(w, `go_threads %d`+"\n", numThread)

	// Export build details.
	fmt.Fprintf(w, "go_info{version=%q} 1\n", runtime.Version())
	fmt.Fprintf(w, "go_info_ext{compiler=%q, GOARCH=%q, GOOS=%q, GOROOT=%q} 1\n",
		runtime.Compiler, runtime.GOARCH, runtime.GOOS, runtime.GOROOT())
}

func writeRuntimeMetrics(w io.Writer) {
	samples := make([]runtimemetrics.Sample, len(runtimeMetrics))
	for i, rm := range runtimeMetrics {
		samples[i].Name = rm[0]
	}
	runtimemetrics.Read(samples)
	for i, rm := range runtimeMetrics {
		writeRuntimeMetric(w, rm[1], &samples[i])
	}
}

func writeRuntimeMetric(w io.Writer, name string, sample *runtimemetrics.Sample) {
	switch sample.Value.Kind() {
	case runtimemetrics.KindBad:
		panic(fmt.Errorf("BUG: unexpected runtimemetrics.KindBad for sample.Name=%q", sample.Name))
	case runtimemetrics.KindUint64:
		fmt.Fprintf(w, "%s %d\n", name, sample.Value.Uint64())
	case runtimemetrics.KindFloat64:
		fmt.Fprintf(w, "%s %g\n", name, sample.Value.Float64())
	case runtimemetrics.KindFloat64Histogram:
		writeRuntimeHistogramMetric(w, name, sample.Value.Float64Histogram())
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
}

// Limit the number of buckets for Go runtime histograms in order to prevent from high cardinality issues at scraper side.
const maxRuntimeHistogramBuckets = 30
