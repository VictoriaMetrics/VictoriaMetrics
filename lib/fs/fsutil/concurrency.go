package fsutil

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var maxConcurrency = flagutil.NewIntWithDynamicDefault("fs.maxConcurrency", getDefaultConcurrency(), "fsutil.getDefaultConcurrency()",
	"The maximum number of concurrent goroutines to work with files; smaller values may help reducing Go scheduling latency "+
		"on systems with small number of CPU cores; higher values may help reducing data ingestion latency on systems with high-latency storage such as NFS or Ceph")

func getDefaultConcurrency() int {
	n := min(16*cgroup.AvailableCPUs(), 256)
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
