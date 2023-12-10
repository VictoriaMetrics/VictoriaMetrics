// Package metrics implements Prometheus-compatible metrics for applications.
//
// This package is lightweight alternative to https://github.com/prometheus/client_golang
// with simpler API and smaller dependencies.
//
// Usage:
//
//  1. Register the required metrics via New* functions.
//  2. Expose them to `/metrics` page via WritePrometheus.
//  3. Update the registered metrics during application lifetime.
//
// The package has been extracted from https://victoriametrics.com/
package metrics

import (
	"io"
	"sort"
	"sync"
	"unsafe"
)

type namedMetric struct {
	name   string
	metric metric
	isAux  bool
}

type metric interface {
	marshalTo(prefix string, w io.Writer)
}

var defaultSet = NewSet()

func init() {
	RegisterSet(defaultSet)
}

var (
	registeredSets     = make(map[*Set]struct{})
	registeredSetsLock sync.Mutex
)

// RegisterSet registers the given set s for metrics export via global WritePrometheus() call.
//
// See also UnregisterSet.
func RegisterSet(s *Set) {
	registeredSetsLock.Lock()
	registeredSets[s] = struct{}{}
	registeredSetsLock.Unlock()
}

// UnregisterSet stops exporting metrics for the given s via global WritePrometheus() call.
//
// Call s.UnregisterAllMetrics() after unregistering s if it is no longer used.
func UnregisterSet(s *Set) {
	registeredSetsLock.Lock()
	delete(registeredSets, s)
	registeredSetsLock.Unlock()
}

// WritePrometheus writes all the metrics from default set and all the registered sets in Prometheus format to w.
//
// Additional sets can be registered via RegisterSet() call.
//
// If exposeProcessMetrics is true, then various `go_*` and `process_*` metrics
// are exposed for the current process.
//
// The WritePrometheus func is usually called inside "/metrics" handler:
//
//	http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
//	    metrics.WritePrometheus(w, true)
//	})
func WritePrometheus(w io.Writer, exposeProcessMetrics bool) {
	registeredSetsLock.Lock()
	sets := make([]*Set, 0, len(registeredSets))
	for s := range registeredSets {
		sets = append(sets, s)
	}
	registeredSetsLock.Unlock()

	sort.Slice(sets, func(i, j int) bool {
		return uintptr(unsafe.Pointer(sets[i])) < uintptr(unsafe.Pointer(sets[j]))
	})
	for _, s := range sets {
		s.WritePrometheus(w)
	}
	if exposeProcessMetrics {
		WriteProcessMetrics(w)
	}
}

// WriteProcessMetrics writes additional process metrics in Prometheus format to w.
//
// The following `go_*` and `process_*` metrics are exposed for the currently
// running process. Below is a short description for the exposed `process_*` metrics:
//
//   - process_cpu_seconds_system_total - CPU time spent in syscalls
//
//   - process_cpu_seconds_user_total - CPU time spent in userspace
//
//   - process_cpu_seconds_total - CPU time spent by the process
//
//   - process_major_pagefaults_total - page faults resulted in disk IO
//
//   - process_minor_pagefaults_total - page faults resolved without disk IO
//
//   - process_resident_memory_bytes - recently accessed memory (aka RSS or resident memory)
//
//   - process_resident_memory_peak_bytes - the maximum RSS memory usage
//
//   - process_resident_memory_anon_bytes - RSS for memory-mapped files
//
//   - process_resident_memory_file_bytes - RSS for memory allocated by the process
//
//   - process_resident_memory_shared_bytes - RSS for memory shared between multiple processes
//
//   - process_virtual_memory_bytes - virtual memory usage
//
//   - process_virtual_memory_peak_bytes - the maximum virtual memory usage
//
//   - process_num_threads - the number of threads
//
//   - process_start_time_seconds - process start time as unix timestamp
//
//   - process_io_read_bytes_total - the number of bytes read via syscalls
//
//   - process_io_written_bytes_total - the number of bytes written via syscalls
//
//   - process_io_read_syscalls_total - the number of read syscalls
//
//   - process_io_write_syscalls_total - the number of write syscalls
//
//   - process_io_storage_read_bytes_total - the number of bytes actually read from disk
//
//   - process_io_storage_written_bytes_total - the number of bytes actually written to disk
//
//   - go_sched_latencies_seconds - time spent by goroutines in ready state before they start execution
//
//   - go_mutex_wait_seconds_total - summary time spent by all the goroutines while waiting for locked mutex
//
//   - go_gc_mark_assist_cpu_seconds_total - summary CPU time spent by goroutines in GC mark assist state
//
//   - go_gc_cpu_seconds_total - summary time spent in GC
//
//   - go_gc_pauses_seconds - duration of GC pauses
//
//   - go_scavenge_cpu_seconds_total - CPU time spent on returning the memory to OS
//
//   - go_memlimit_bytes - the GOMEMLIMIT env var value
//
//   - go_memstats_alloc_bytes - memory usage for Go objects in the heap
//
//   - go_memstats_alloc_bytes_total - the cumulative counter for total size of allocated Go objects
//
//   - go_memstats_buck_hash_sys_bytes - bytes of memory in profiling bucket hash tables
//
//   - go_memstats_frees_total - the cumulative counter for number of freed Go objects
//
//   - go_memstats_gc_cpu_fraction - the fraction of CPU spent in Go garbage collector
//
//   - go_memstats_gc_sys_bytes - the size of Go garbage collector metadata
//
//   - go_memstats_heap_alloc_bytes - the same as go_memstats_alloc_bytes
//
//   - go_memstats_heap_idle_bytes - idle memory ready for new Go object allocations
//
//   - go_memstats_heap_inuse_bytes - bytes in in-use spans
//
//   - go_memstats_heap_objects - the number of Go objects in the heap
//
//   - go_memstats_heap_released_bytes - bytes of physical memory returned to the OS
//
//   - go_memstats_heap_sys_bytes - memory requested for Go objects from the OS
//
//   - go_memstats_last_gc_time_seconds - unix timestamp the last garbage collection finished
//
//   - go_memstats_lookups_total - the number of pointer lookups performed by the runtime
//
//   - go_memstats_mallocs_total - the number of allocations for Go objects
//
//   - go_memstats_mcache_inuse_bytes - bytes of allocated mcache structures
//
//   - go_memstats_mcache_sys_bytes - bytes of memory obtained from the OS for mcache structures
//
//   - go_memstats_mspan_inuse_bytes - bytes of allocated mspan structures
//
//   - go_memstats_mspan_sys_bytes - bytes of memory obtained from the OS for mspan structures
//
//   - go_memstats_next_gc_bytes - the target heap size when the next garbage collection should start
//
//   - go_memstats_other_sys_bytes - bytes of memory in miscellaneous off-heap runtime allocations
//
//   - go_memstats_stack_inuse_bytes - memory used for goroutine stacks
//
//   - go_memstats_stack_sys_bytes - memory requested fromthe OS for goroutine stacks
//
//   - go_memstats_sys_bytes - memory requested by Go runtime from the OS
//
//   - go_cgo_calls_count - the total number of CGO calls
//
//   - go_cpu_count - the number of CPU cores on the host where the app runs
//
// The WriteProcessMetrics func is usually called in combination with writing Set metrics
// inside "/metrics" handler:
//
//	http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
//	    mySet.WritePrometheus(w)
//	    metrics.WriteProcessMetrics(w)
//	})
//
// See also WrteFDMetrics.
func WriteProcessMetrics(w io.Writer) {
	writeGoMetrics(w)
	writeProcessMetrics(w)
	writePushMetrics(w)
}

// WriteFDMetrics writes `process_max_fds` and `process_open_fds` metrics to w.
func WriteFDMetrics(w io.Writer) {
	writeFDMetrics(w)
}

// UnregisterMetric removes metric with the given name from default set.
//
// See also UnregisterAllMetrics.
func UnregisterMetric(name string) bool {
	return defaultSet.UnregisterMetric(name)
}

// UnregisterAllMetrics unregisters all the metrics from default set.
func UnregisterAllMetrics() {
	defaultSet.UnregisterAllMetrics()
}

// ListMetricNames returns sorted list of all the metric names from default set.
func ListMetricNames() []string {
	return defaultSet.ListMetricNames()
}

// GetDefaultSet returns the default metrics set.
func GetDefaultSet() *Set {
	return defaultSet
}
