package appmetrics

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/metrics"
)

type osInfo struct {
	os      string
	release string
}

var initedOS osInfo
var initOSOnce sync.Once

func writeOSMetrics(w io.Writer) {
	initOSOnce.Do(initOS)

	if initedOS.os != "" {
		metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_os_info{os=%q, release=%q}`, initedOS.os, initedOS.release), 1)
	}
}
