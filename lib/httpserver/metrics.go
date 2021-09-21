package httpserver

import (
	"flag"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/metrics"
)

var versionRe = regexp.MustCompile(`v\d+\.\d+\.\d+(?:-enterprise)?(?:-cluster)?`)

// WritePrometheusMetrics writes all the registered metrics to w in Prometheus exposition format.
func WritePrometheusMetrics(w io.Writer) {
	currentTime := time.Now()
	metricsCacheLock.Lock()
	if currentTime.Sub(metricsCacheLastUpdateTime) > time.Second {
		var bb bytesutil.ByteBuffer
		writePrometheusMetrics(&bb)
		metricsCache.Store(&bb)
		metricsCacheLastUpdateTime = currentTime
	}
	metricsCacheLock.Unlock()

	bb := metricsCache.Load().(*bytesutil.ByteBuffer)
	_, _ = w.Write(bb.B)
}

var (
	metricsCacheLock           sync.Mutex
	metricsCacheLastUpdateTime time.Time
	metricsCache               atomic.Value
)

func writePrometheusMetrics(w io.Writer) {
	metrics.WritePrometheus(w, true)
	metrics.WriteFDMetrics(w)

	fmt.Fprintf(w, "vm_app_version{version=%q, short_version=%q} 1\n", buildinfo.Version,
		versionRe.FindString(buildinfo.Version))
	fmt.Fprintf(w, "vm_allowed_memory_bytes %d\n", memory.Allowed())
	fmt.Fprintf(w, "vm_available_memory_bytes %d\n", memory.Allowed()+memory.Remaining())
	fmt.Fprintf(w, "vm_available_cpu_cores %d\n", cgroup.AvailableCPUs())
	fmt.Fprintf(w, "vm_gogc %d\n", cgroup.GetGOGC())

	// Export start time and uptime in seconds
	fmt.Fprintf(w, "vm_app_start_timestamp %d\n", startTime.Unix())
	fmt.Fprintf(w, "vm_app_uptime_seconds %d\n", int(time.Since(startTime).Seconds()))

	// Export flags as metrics.
	isSetMap := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		isSetMap[f.Name] = true
	})
	flag.VisitAll(func(f *flag.Flag) {
		lname := strings.ToLower(f.Name)
		value := f.Value.String()
		if flagutil.IsSecretFlag(lname) {
			// Do not expose passwords and keys to prometheus.
			value = "secret"
		}
		isSet := "false"
		if isSetMap[f.Name] {
			isSet = "true"
		}
		fmt.Fprintf(w, "flag{name=%q, value=%q, is_set=%q} 1\n", f.Name, value, isSet)
	})
}

var startTime = time.Now()
