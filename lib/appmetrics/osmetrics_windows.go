package appmetrics

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"golang.org/x/sys/windows"
)

var release string
var initOnce sync.Once

func writeOSMetrics(w io.Writer) {
	initOnce.Do(initOSMetrics)

	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_os_info{os="windows", release=%q}`, release), 1)
}

func initOSMetrics() {
	ver := windows.RtlGetVersion()
	if ver == nil {
		logger.Warnf("vm_os_info metric will miss release info since windows.RtlGetVersion returned nil version")
		return
	}

	release = fmt.Sprintf("%d.%d.%d", ver.MajorVersion, ver.MinorVersion, ver.BuildNumber)
}
