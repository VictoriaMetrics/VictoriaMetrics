package appmetrics

import (
	"fmt"
	"sync"
	"syscall"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var release string
var initOnce sync.Once

func writeOSMetrics(w io.Writer) {
	initOnce.Do(initOSMetrics)

	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_os_info{os="linux", release=%q}`, release), 1)
}

func initOSMetrics() {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		logger.Warnf("vm_os_info metric will miss release info since syscall.Uname failed: %s", err)
		return
	}

	ur := make([]byte, 0, len(uname.Release))
	for _, v := range uname.Release {
		if v == 0 {
			break
		}
		ur = append(ur, byte(v))
	}
	release = string(ur)
}
