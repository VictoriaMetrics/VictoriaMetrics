package appmetrics

import (
	"os/exec"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func initOS() {
	os = osInfo{name: "darwin"}

	out, err := exec.Command("sysctl", "-n", "kern.osrelease").Output()
	if err != nil {
		logger.Warnf("vm_os_info metric will miss release info since exec 'sysctl -n \"kern.osrelease\"' call failed: %s", err)
		return
	}

	os.release = strings.TrimSpace(string(out))
}
