package mdx

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

type VmInstanceFilter struct {
	mu         sync.RWMutex
	wg         sync.WaitGroup
	stopCh     chan struct{}
	vmInstance map[string]*atomic.Int64
}

var GlobalVmInstanceFilter *VmInstanceFilter

func InitGlobalVmInstanceFilter() {
	GlobalVmInstanceFilter = &VmInstanceFilter{
		vmInstance: make(map[string]*atomic.Int64),
		stopCh:     make(chan struct{}),
	}
	GlobalVmInstanceFilter.wg.Go(GlobalVmInstanceFilter.cleanStale)
}

func (filter *VmInstanceFilter) cleanStale() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			filter.mu.Lock()
			currTs := time.Now().Unix()

			dst := make(map[string]*atomic.Int64, len(filter.vmInstance))
			for k, v := range filter.vmInstance {
				// todo configurable
				if currTs-v.Load() < 60 {
					dst[k] = v
				}
			}
			if len(dst) != len(filter.vmInstance) {
				filter.vmInstance = dst
			}
			filter.mu.Unlock()
		case <-filter.stopCh:
			return
		}
	}
}

func (filter *VmInstanceFilter) MustStop() {
	if filter == nil {
		return
	}
	close(filter.stopCh)
	filter.wg.Wait()
}

func (filter *VmInstanceFilter) ApplyMdxFilter(tss []prompb.TimeSeries) []prompb.TimeSeries {
	tssDst := tss[:0]
	for _, ts := range tss {
		isVmInstance := false
		var instance string
		var job string
		for _, label := range ts.Labels {
			if label.Name == "__name__" && label.Value == "vm_app_version" {
				isVmInstance = true
			}
			if label.Name == "instance" {
				instance = label.Value
			}
			if label.Name == "job" {
				job = label.Value
			}
		}
		identicalKey := fmt.Sprintf("%s:%s", job, instance)
		currTs := time.Now().Unix()

		//fast path
		filter.mu.RLock()
		ptr, ok := filter.vmInstance[identicalKey]
		filter.mu.RUnlock()
		if ok {
			ptr.Store(currTs)
			tssDst = append(tssDst, ts)
			continue
		}
		if !isVmInstance {
			continue
		}

		// slow path
		tssDst = append(tssDst, ts)
		filter.mu.Lock()
		if ptr, ok = filter.vmInstance[identicalKey]; ok {
			ptr.Store(currTs)
		} else {
			v := atomic.Int64{}
			v.Store(currTs)
			filter.vmInstance[identicalKey] = &v
		}
		filter.mu.Unlock()
	}

	return tssDst
}
