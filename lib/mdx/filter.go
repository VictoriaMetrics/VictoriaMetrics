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

// Filter manages the list of VictoriaMetrics instances discovered from previous data flow, and uses it to filter out metrics that are not from VictoriaMetrics instances.
type Filter struct {
	mu                    sync.RWMutex
	wg                    sync.WaitGroup
	stopCh                chan struct{}
	vmInstance            map[string]*atomic.Int64
	mdxTrackedVmInstances *metrics.Gauge
}

var GlobalFilter *Filter

func InitGlobalFilter() {
	if mdxInstanceEntryTTL.Milliseconds() == 0 {
		logger.Warnf("MDX instance entry cleanup mechanism will be disabled without explicilty setting -mdx.instanceEntryTTL.")
		return
	}
	GlobalFilter = &Filter{
		vmInstance: make(map[string]*atomic.Int64),
		stopCh:     make(chan struct{}),
	}
	GlobalFilter.mdxTrackedVmInstances = metrics.NewGauge("vmagent_mdx_tracked_vm_instances", func() float64 {
		GlobalFilter.mu.RLock()
		n := len(GlobalFilter.vmInstance)
		GlobalFilter.mu.RUnlock()
		return float64(n)
	})

	GlobalFilter.wg.Go(GlobalFilter.cleanStale)
}

func (filter *Filter) cleanStale() {
	ttlSec := int64(mdxInstanceEntryTTL.Duration().Seconds())
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			filter.mu.Lock()
			currTs := time.Now().Unix()

			dst := make(map[string]*atomic.Int64, len(filter.vmInstance))
			for k, v := range filter.vmInstance {
				if currTs-v.Load() < ttlSec {
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

func (filter *Filter) MustStop() {
	if filter == nil {
		return
	}
	close(filter.stopCh)
	filter.wg.Wait()
}

func (filter *Filter) Filter(tss []prompb.TimeSeries, resTss []prompb.TimeSeries) []prompb.TimeSeries {
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
		if len(job) == 0 || len(instance) == 0 {
			continue
		}
		identicalKey := fmt.Sprintf("%q:%q", job, instance)
		currTs := time.Now().Unix()

		//fast path
		filter.mu.RLock()
		ptr, ok := filter.vmInstance[identicalKey]
		filter.mu.RUnlock()
		if ok {
			ptr.Store(currTs)
			resTss = append(resTss, ts)
			continue
		}
		if !isVmInstance {
			continue
		}

		// slow path
		resTss = append(resTss, ts)
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

	return resTss
}
