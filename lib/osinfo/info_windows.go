package osinfo

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"golang.org/x/sys/windows"
)

var windowsRelease string

func ExposeAsMetric() {
	ver := windows.RtlGetVersion()
	if ver == nil {
		logger.Warnf("os info wont be exposed as vm_os_info metric; windows.RtlGetVersion returned nil version")
		return
	}
	windowsRelease = fmt.Sprintf("%d.%d.%d", ver.MajorVersion, ver.MinorVersion, ver.BuildNumber)
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vm_os_info{os="windows", release=%q}`, windowsRelease), func() float64 { return 1 })
}
