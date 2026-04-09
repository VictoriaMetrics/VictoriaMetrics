package osinfo

import (
	"fmt"
	"syscall"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var linuxRelease string

func ExposeAsMetric() {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		logger.Warnf("os info wont be exposed as vm_os_info metric; failed to call syscall.Uname: %s", err)
		return
	}

	release := make([]byte, 0, len(uname.Release))
	for _, v := range uname.Release {
		if v == 0 {
			break
		}
		release = append(release, byte(v))
	}
	linuxRelease = string(release)

	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vm_os_info{os="linux", release=%q}`, linuxRelease), func() float64 { return 1 })
}
