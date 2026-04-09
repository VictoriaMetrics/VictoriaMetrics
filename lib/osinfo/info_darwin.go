package osinfo

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var darwinRelease string

func ExposeAsMetric() {
	out, err := exec.Command("sysctl", "-n", "kern.osrelease").Output()
	if err != nil {
		logger.Warnf("os info wont be exposed as vm_os_info metric; exec 'sysctl -n \"kern.osrelease\"' failed: %s", err)
		return
	}

	darwinRelease = strings.TrimSpace(string(out))

	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vm_os_info{os="darwin", release=%q}`, darwinRelease), func() float64 { return 1 })
}
