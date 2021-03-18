// Package metrics implements Prometheus-compatible metrics for applications.
//
// This package is lightweight alternative to https://github.com/prometheus/client_golang
// with simpler API and smaller dependencies.
//
// Usage:
//
//     1. Register the required metrics via New* functions.
//     2. Expose them to `/metrics` page via WritePrometheus.
//     3. Update the registered metrics during application lifetime.
//
// The package has been extracted from https://victoriametrics.com/
package metrics

import (
	"io"
)

type namedMetric struct {
	name   string
	metric metric
}

type metric interface {
	marshalTo(prefix string, w io.Writer)
}

var defaultSet = NewSet()

// WritePrometheus writes all the registered metrics in Prometheus format to w.
//
// If exposeProcessMetrics is true, then various `go_*` and `process_*` metrics
// are exposed for the current process.
//
// The WritePrometheus func is usually called inside "/metrics" handler:
//
//     http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
//         metrics.WritePrometheus(w, true)
//     })
//
func WritePrometheus(w io.Writer, exposeProcessMetrics bool) {
	defaultSet.WritePrometheus(w)
	if exposeProcessMetrics {
		WriteProcessMetrics(w)
	}
}

// WriteProcessMetrics writes additional process metrics in Prometheus format to w.
//
// The following `go_*` and `process_*` metrics are exposed for the currently
// running process. Below is a short description for the exposed `process_*` metrics:
//
//     - process_cpu_seconds_system_total - CPU time spent in syscalls
//     - process_cpu_seconds_user_total - CPU time spent in userspace
//     - process_cpu_seconds_total - CPU time spent by the process
//     - process_major_pagefaults_total - page faults resulted in disk IO
//     - process_minor_pagefaults_total - page faults resolved without disk IO
//     - process_resident_memory_bytes - recently accessed memory (aka RSS or resident memory)
//     - process_resident_memory_peak_bytes - the maximum RSS memory usage
//     - process_resident_memory_anon_bytes - RSS for memory-mapped files
//     - process_resident_memory_file_bytes - RSS for memory allocated by the process
//     - process_resident_memory_shared_bytes - RSS for memory shared between multiple processes
//     - process_virtual_memory_bytes - virtual memory usage
//     - process_virtual_memory_peak_bytes - the maximum virtual memory usage
//     - process_num_threads - the number of threads
//     - process_start_time_seconds - process start time as unix timestamp
//
//     - process_io_read_bytes_total - the number of bytes read via syscalls
//     - process_io_written_bytes_total - the number of bytes written via syscalls
//     - process_io_read_syscalls_total - the number of read syscalls
//     - process_io_write_syscalls_total - the number of write syscalls
//     - process_io_storage_read_bytes_total - the number of bytes actually read from disk
//     - process_io_storage_written_bytes_total - the number of bytes actually written to disk
//
// The WriteProcessMetrics func is usually called in combination with writing Set metrics
// inside "/metrics" handler:
//
//     http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
//         mySet.WritePrometheus(w)
//         metrics.WriteProcessMetrics(w)
//     })
//
// See also WrteFDMetrics.
func WriteProcessMetrics(w io.Writer) {
	writeGoMetrics(w)
	writeProcessMetrics(w)
}

// WriteFDMetrics writes `process_max_fds` and `process_open_fds` metrics to w.
func WriteFDMetrics(w io.Writer) {
	writeFDMetrics(w)
}

// UnregisterMetric removes metric with the given name from default set.
func UnregisterMetric(name string) bool {
	return defaultSet.UnregisterMetric(name)
}
