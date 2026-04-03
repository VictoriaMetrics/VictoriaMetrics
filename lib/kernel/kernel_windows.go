package kernel

import (
	"fmt"

	"github.com/VictoriaMetrics/metrics"
	"golang.org/x/sys/windows"
)

var osReleaseInfo string

func init() {
	ver := windows.RtlGetVersion()
	if ver == nil {
		return
	}
	osReleaseInfo = fmt.Sprintf("%d.%d.%d", ver.MajorVersion, ver.MinorVersion, ver.BuildNumber)
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vm_os_metadata{kernel="windows", release=%q}`, osReleaseInfo), func() float64 { return 1 })
}
