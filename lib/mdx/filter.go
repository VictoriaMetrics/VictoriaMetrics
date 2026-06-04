package mdx

import (
	"flag"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

var (
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

func NewFilter() *Filter {
	filter := &Filter{
		vmInstance: make(map[string]*atomic.Int64),
		stopCh:     make(chan struct{}),
	}
	if len(*keepMetricsWithLabelName) > 0 && len(*keepMetricsWithLabelValue) > 0 {
		filter.filterByLabel = true
	} else if len(*keepMetricsWithLabelName) > 0 || len(*keepMetricsWithLabelValue) > 0 {
		logger.Fatalf("Both -mdx.keepMetricsWithLabel.name and -mdx.keepMetricsWithLabel.value must be set if one of them is set.")
	}
	filter.wg.Go(filter.cleanStale)
	return filter
}

func (filter *Filter) VmInstancesCount() int {
	filter.mu.RLock()
	defer filter.mu.RUnlock()
	return len(filter.vmInstance)

}

func (filter *Filter) cleanStale() {
	entryTTL := time.Hour * 1
	ttlSec := int64(entryTTL.Seconds())
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
	currTs := time.Now().Unix()
	var identicalKey []byte

nextTss:
	for _, ts := range tss {
		var hasVersionLabel, triedJobInstance bool
		var job, instance string
		for _, label := range ts.Labels {
			if filter.filterByLabel && label.Name == *keepMetricsWithLabelName && label.Value == *keepMetricsWithLabelValue {
				resTss = append(resTss, ts)
				continue nextTss
			}

			if label.Name == "__name__" && label.Value == "vm_app_version" {
				hasVersionLabel = true
			}
			if instance == "" && label.Name == "instance" {
				if label.Value == "" {
					continue
				}

				instance = label.Value
			}
			if job == "" && label.Name == "job" {
				if label.Value == "" {
					continue
				}

				job = label.Value
			}
			if !triedJobInstance && job != "" && instance != "" {
				identicalKey = identicalKey[:0]
				identicalKey = strconv.AppendQuote(identicalKey, job)
				identicalKey = append(identicalKey, ':')
				identicalKey = strconv.AppendQuote(identicalKey, instance)
				filter.mu.RLock()
				ptr, found := filter.vmInstance[bytesutil.ToUnsafeString(identicalKey)]
				filter.mu.RUnlock()
				if found {
					ptr.Store(currTs)
					resTss = append(resTss, ts)
					continue nextTss
				}
				triedJobInstance = true
			}

			if hasVersionLabel && job != "" && instance != "" {
				identicalKey = identicalKey[:0]
				identicalKey = strconv.AppendQuote(identicalKey, job)
				identicalKey = append(identicalKey, ':')
				identicalKey = strconv.AppendQuote(identicalKey, instance)

				v := &atomic.Int64{}
				v.Store(currTs)

				filter.mu.Lock()
				filter.vmInstance[string(identicalKey)] = v
				filter.mu.Unlock()

				continue nextTss
			}
		}
	}
	logger.Infof("jayice:%d", len(resTss))
	return resTss
}
