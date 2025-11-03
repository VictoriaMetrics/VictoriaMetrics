package fsutil

import (
	"flag"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
)

var maxConcurrency = flag.Int("fs.maxConcurrency", getDefaultConcurrency(), "The maximum number of concurrent goroutines to work with files; smaller values may help reducing Go scheduling latency "+
	"on systems with small number of CPU cores; higher values may help reducing data ingestion latency on systems with high-latency storage such as NFS or Ceph")

func getDefaultConcurrency() int {
	n := 16 * cgroup.AvailableCPUs()
	if n > 265 {
		n = 265
	}
	return n
}

// GetConcurrencyCh returns a channel for limiting the concurrency of operations with files.
func GetConcurrencyCh() chan struct{} {
	concurrencyChOnce.Do(initConcurrencyCh)
	return concurrencyCh
}

func initConcurrencyCh() {
	concurrencyCh = make(chan struct{}, *maxConcurrency)
}

var concurrencyChOnce sync.Once
var concurrencyCh chan struct{}
