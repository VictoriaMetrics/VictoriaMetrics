package cgroup

import (
	"github.com/VictoriaMetrics/metrics"
	"runtime"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// AvailableCPUs returns the number of available CPU cores for the app.
//
// The number is rounded to the next integer value if fractional number of CPU cores are available.
func AvailableCPUs() int {
	return runtime.GOMAXPROCS(-1)
}

func init() {
	logger.Infof("VictoriaMetrics runtime GOMAXPROCS: %d", AvailableCPUs())
	metrics.NewGauge(`process_cpu_cores_available`, func() float64 {
		return float64(AvailableCPUs())
	})
}
