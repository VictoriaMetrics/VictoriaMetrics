package mdx

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/metrics"
)

var (
	mdxInstanceEntryTTL = flagutil.NewExtendedDuration("mdx.instanceEntryTTL", "0", "After not receiving metrics for the VictoriaMetrics instance for the configured time, remove this instance from the MDX instance list."+
		"It should be several times the scrape interval for VictoriaMetrics instances. The cleanup mechanism helps release memory after a VictoriaMetrics instance is permanently taken offline, preventing the MDX instance list from growing indefinitely."+
		"It must be explicitly set when -remoteWrite.mdx.enable is set and requires explicit unit suffixes (s, m, h, d, w, y). Please see https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring-data-exchange")
)

type VmInstanceFilter struct {
	mu                    sync.RWMutex
	wg                    sync.WaitGroup
	stopCh                chan struct{}
	vmInstance            map[string]*atomic.Int64
	mdxTrackedVmInstances *metrics.Gauge
}

var GlobalVmInstanceFilter *VmInstanceFilter

func InitGlobalVmInstanceFilter() {
	if mdxInstanceEntryTTL.Milliseconds() == 0 {
		logger.Panicf("-mdx.instanceEntryTTL must be explicitly set when -remoteWrite.mdx.enable is set.")
	}
	GlobalVmInstanceFilter = &VmInstanceFilter{
		vmInstance: make(map[string]*atomic.Int64),
		stopCh:     make(chan struct{}),
	}
	GlobalVmInstanceFilter.mdxTrackedVmInstances = metrics.NewGauge("vmagent_mdx_tracked_vm_instances", func() float64 {
		GlobalVmInstanceFilter.mu.RLock()
		n := len(GlobalVmInstanceFilter.vmInstance)
		GlobalVmInstanceFilter.mu.RUnlock()
		return float64(n)
	})

	GlobalVmInstanceFilter.wg.Go(GlobalVmInstanceFilter.cleanStale)
}

func (filter *VmInstanceFilter) cleanStale() {
	if mdxInstanceEntryTTL.Milliseconds() == 0 {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			filter.mu.Lock()
			currTs := time.Now().Unix()

			dst := make(map[string]*atomic.Int64, len(filter.vmInstance))
			for k, v := range filter.vmInstance {
				if currTs-v.Load() < mdxInstanceEntryTTL.Duration().Milliseconds()/1000 {
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
