package appmetrics

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/metrics"
)

type osInfo struct {
	name    string
	release string
}

var hostOS osInfo
var initOSOnce sync.Once

func writeOSMetrics(w io.Writer) {
	initOSOnce.Do(initOS)

	if hostOS.name != "" {
		metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_os_info{os=%q, release=%q}`, hostOS.name, hostOS.release), 1)
	}
}
