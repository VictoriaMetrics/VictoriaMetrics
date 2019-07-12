package metrics

import (
	"fmt"
	"io"
	"runtime"

	"github.com/valyala/histogram"
)

func writeGoMetrics(w io.Writer) {
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
