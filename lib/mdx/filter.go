package mdx

import (
	"flag"
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
	mdxInstanceEntryTTL = flagutil.NewExtendedDuration("mdx.instanceEntryTTL", "1h", "After not receiving metrics for the VictoriaMetrics instance for the configured time, remove this instance from the MDX instance list."+
		"It should be several times the scrape interval for VictoriaMetrics instances. The cleanup mechanism helps release memory after a VictoriaMetrics instance is permanently taken offline, preventing the MDX instance list from growing indefinitely."+
		"It must be explicitly set when -remoteWrite.mdx.enable is set and requires explicit unit suffixes (s, m, h, d, w, y). Please see https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring-data-exchange")
	keepMetricsWithLabelName = flag.String("mdx.keepMetricsWithLabel.name", "", "Keep metrics containing specific label and label value to the `-remoteWrite.url` that configured with `-remoteWrite.mdx.enable=true`. "+
		"See also -mdx.keepMetricsWithLabel.value.")
	keepMetricsWithLabelValue = flag.String("mdx.keepMetricsWithLabel.value", "", "Keep metrics containing specific label and label value to the `-remoteWrite.url` that configured with `-remoteWrite.mdx.enable=true`. "+
		"See also -mdx.keepMetricsWithLabel.name")
)

// Filter manages the list of VictoriaMetrics instances discovered from previous data flow, and uses it to filter out metrics that are not from VictoriaMetrics instances.
type Filter struct {
	mu            sync.RWMutex
	wg            sync.WaitGroup
	stopCh        chan struct{}
	vmInstance    map[string]*atomic.Int64
	filterByLabel bool
}

var GlobalFilter *Filter

func InitGlobalFilter() {
	GlobalFilter = &Filter{
		vmInstance: make(map[string]*atomic.Int64),
		stopCh:     make(chan struct{}),
	}
	if len(*keepMetricsWithLabelName) > 0 && len(*keepMetricsWithLabelValue) > 0 {
		GlobalFilter.filterByLabel = true
	} else if len(*keepMetricsWithLabelName) > 0 || len(*keepMetricsWithLabelValue) > 0 {
		logger.Fatalf("Both -mdx.keepMetricsWithLabel.name and -mdx.keepMetricsWithLabel.value must be set if one of them is set.")
	}

	_ = metrics.NewGauge("vmagent_mdx_tracked_vm_instances", func() float64 {
		GlobalFilter.mu.RLock()
		n := len(GlobalFilter.vmInstance)
		GlobalFilter.mu.RUnlock()
		return float64(n)
	})
	if mdxInstanceEntryTTL.Milliseconds() != 0 {
		GlobalFilter.wg.Go(GlobalFilter.cleanStale)
	}
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
		isStored := false
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
			if filter.filterByLabel {
				if label.Name == *keepMetricsWithLabelName && label.Value == *keepMetricsWithLabelValue {
					resTss = append(resTss, ts)
					isStored = true
				}
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
			if !isStored {
				resTss = append(resTss, ts)
				isStored = true
			}
			continue
		}
		if !isVmInstance {
			continue
		}

		// slow path
		if !isStored {
			resTss = append(resTss, ts)
			isStored = true
		}
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
