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

var os osInfo
var initOSOnce sync.Once

func writeOSMetrics(w io.Writer) {
	initOSOnce.Do(initOS)

	if os.name != "" {
		metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_os_info{os=%q, release=%q}`, os.name, os.release), 1)
	}
}
