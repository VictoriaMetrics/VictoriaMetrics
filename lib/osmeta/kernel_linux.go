package osmeta

import (
	"fmt"
	"log"
	"syscall"

	"github.com/VictoriaMetrics/metrics"
)

var osReleaseInfo string

func ExposeOsInfoAsMetric() {
	var uname syscall.Utsname
	err := syscall.Uname(&uname)
	if err != nil {
		log.Printf("ERROR: metrics: fail to call syscall.Uname: %s", err)
		return
	}
	release := make([]byte, 0, len(uname.Release))
	for _, v := range uname.Release {
		if v == 0 {
			break
		}
		release = append(release, byte(v))
	}
	osReleaseInfo = string(release)
	_ = metrics.GetOrCreateGauge(fmt.Sprintf(`vm_os_info{kernel="linux", release=%q}`, osReleaseInfo), func() float64 { return 1 })
}
