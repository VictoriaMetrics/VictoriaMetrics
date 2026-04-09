package appmetrics

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var release string
var initOnce sync.Once

func writeOSMetrics(w io.Writer) {
	initOnce.Do(initOSMetrics)

	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_os_info{os="darwin", release=%q}`, release), 1)
}

func initOSMetrics() {
	out, err := exec.Command("sysctl", "-n", "kern.osrelease").Output()
	if err != nil {
		logger.Warnf("vm_os_info metric will miss release info since exec 'sysctl -n \"kern.osrelease\"' call failed: %s", err)
		return
	}

	release = strings.TrimSpace(string(out))
}
