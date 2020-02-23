package httpserver

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/metrics"
)

// WritePrometheusMetrics writes all the registered metrics to w in Prometheus exposition format.
func WritePrometheusMetrics(w io.Writer) {
	metrics.WritePrometheus(w, true)

	fmt.Fprintf(w, "vm_app_version{version=%q} 1\n", buildinfo.Version)
	fmt.Fprintf(w, "vm_allowed_memory_bytes %d\n", memory.Allowed())

	// Export start time and uptime in seconds
	fmt.Fprintf(w, "vm_app_start_timestamp %d\n", startTime.Unix())
	fmt.Fprintf(w, "vm_app_uptime_seconds %d\n", int(time.Since(startTime).Seconds()))

	// Export flags as metrics.
	flag.VisitAll(func(f *flag.Flag) {
		lname := strings.ToLower(f.Name)
		value := f.Value.String()
		if isSecretFlag(lname) {
			// Do not expose passwords and keys to prometheus.
			value = "secret"
		}
		fmt.Fprintf(w, "flag{name=%q, value=%q} 1\n", f.Name, value)
	})
}

var startTime = time.Now()

// RegisterSecretFlag registers flagName as secret.
//
// This function must be called before starting httpserver.
// It cannot be called from concurrent goroutines.
//
// Secret flags aren't exported at `/metrics` page.
func RegisterSecretFlag(flagName string) {
	lname := strings.ToLower(flagName)
	secretFlags[lname] = true
}

var secretFlags = make(map[string]bool)

func isSecretFlag(s string) bool {
	if strings.Contains(s, "pass") || strings.Contains(s, "key") || strings.Contains(s, "secret") || strings.Contains(s, "token") {
		return true
	}
	return secretFlags[s]
}
