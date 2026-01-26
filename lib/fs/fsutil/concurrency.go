package fsutil

import (
	"flag"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
)

var maxConcurrency = flag.Int("fs.maxConcurrency", getDefaultConcurrency(), "The maximum number of concurrent goroutines to work with files; smaller values may help reducing Go scheduling latency "+
	"on systems with small number of CPU cores; higher values may help reducing data ingestion latency on systems with high-latency storage such as NFS or Ceph")

func getDefaultConcurrency() int {
	n := min(16*cgroup.AvailableCPUs(), 256)
	return n
}

// getConcurrencyCh returns a channel for limiting the concurrency of operations with files.
func getConcurrencyCh() chan struct{} {
	concurrencyChOnce.Do(initConcurrencyCh)
	return concurrencyCh
}

func initConcurrencyCh() {
	concurrencyCh = make(chan struct{}, *maxConcurrency)
}

var concurrencyChOnce sync.Once
var concurrencyCh chan struct{}

type parallelTask interface {
	Run()
}

// ParallelExecutor is used for parallel files operations
//
// ParallelExecutor is needed for speeding up files operations on high-latency storage systems such as NFS or Ceph.
type ParallelExecutor struct {
	tasks []parallelTask
}

// Add registers a task for parallel file operations
//
// Tasks are executed in parallel on Run() call.
func (pe *ParallelExecutor) Add(task parallelTask) {
	pe.tasks = append(pe.tasks, task)
}

func (pe *ParallelExecutor) Run() {
	var wg sync.WaitGroup
	concurrencyCh := getConcurrencyCh()
	for _, task := range pe.tasks {
		concurrencyCh <- struct{}{}
		wg.Add(1)

		go func(task parallelTask) {
			defer func() {
				wg.Done()
				<-concurrencyCh
			}()
			task.Run()
		}(task)
	}
	wg.Wait()
}
