package remotewrite

import (
	"flag"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/metrics"
)

var (
	mdxEnable = flagutil.NewArrayBool("remoteWrite.mdx.enable",
		"Send only internal VM metrics to the corresponding -remoteWrite.url. "+
			"When set, only metrics whose instance label matches an instance that has previously emitted vm_app_version are forwarded. "+
			"See also -remoteWrite.mdx.instanceTTL")

	mdxInstanceTTL = flag.Duration("remoteWrite.mdx.instanceTTL", 10*time.Minute,
		"Retention period for instances in the MDX filter. "+
			"An instance is removed from the filter if no vm_app_version metric is received within this duration. "+
			"Only used when -remoteWrite.mdx.enable is set")
)

type mdxTracker struct {
	mu        sync.RWMutex
	instances map[string]time.Time
}

var globalMDXTracker = &mdxTracker{
	instances: make(map[string]time.Time),
}

var (
	mdxTrackedInstances *metrics.Gauge
	mdxEnabled          bool
)

func initMDXTracker() {
	mdxTrackedInstances = metrics.NewGauge("vmagent_mdx_tracked_instances", func() float64 {
		globalMDXTracker.mu.RLock()
		n := len(globalMDXTracker.instances)
		globalMDXTracker.mu.RUnlock()
		return float64(n)
	})
}

// updateMDXInstances records the instance label of every vm_app_version series in tss.
// It is called in the global push path before per-URL fan-out, so instance discovery
// is not affected by per-URL relabeling rules.
func updateMDXInstances(tss []prompb.TimeSeries) {
	now := time.Now()
	globalMDXTracker.mu.Lock()
	defer globalMDXTracker.mu.Unlock()
	for i := range tss {
		ts := &tss[i]
		var metricName, instance string
		for _, label := range ts.Labels {
			switch label.Name {
			case "__name__":
				metricName = label.Value
			case "instance":
				instance = label.Value
			}
		}
		if metricName == "vm_app_version" && instance != "" {
			globalMDXTracker.instances[instance] = now
		}
	}
}

// applyMDXFilter drops any series whose instance label is not a known VictoriaMetrics instance.
// Series without an instance label are also dropped.
// tss must be a private copy; it is modified in-place.
func applyMDXFilter(tss []prompb.TimeSeries) []prompb.TimeSeries {
	globalMDXTracker.mu.RLock()
	defer globalMDXTracker.mu.RUnlock()
	dst := tss[:0]
	for i := range tss {
		instance := ""
		for _, label := range tss[i].Labels {
			if label.Name == "instance" {
				instance = label.Value
				break
			}
		}
		if instance != "" {
			if _, ok := globalMDXTracker.instances[instance]; ok {
				dst = append(dst, tss[i])
			}
		}
	}
	return dst
}

// cleanupMDXInstances removes instances that haven't sent vm_app_version within -remoteWrite.mdx.instanceTTL.
func cleanupMDXInstances() {
	cutoff := time.Now().Add(-*mdxInstanceTTL)
	globalMDXTracker.mu.Lock()
	defer globalMDXTracker.mu.Unlock()
	for instance, lastSeen := range globalMDXTracker.instances {
		if lastSeen.Before(cutoff) {
			logger.Infof("MDX: removing stale instance %q", instance)
			delete(globalMDXTracker.instances, instance)
		}
	}
}

// isMDXEnabledForAnyURL returns true if -remoteWrite.mdx.enable is set for at least one -remoteWrite.url.
func isMDXEnabledForAnyURL() bool {
	for i := range *remoteWriteURLs {
		if mdxEnable.GetOptionalArg(i) {
			return true
		}
	}
	return false
}
