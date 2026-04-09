package appmetrics

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/windows"
)

func initOS() {
	initedOS = osInfo{os: "windows"}

	ver := windows.RtlGetVersion()
	if ver == nil {
		logger.Warnf("vm_os_info metric will miss release info since windows.RtlGetVersion returned nil version")
		return
	}
	initedOS.release = fmt.Sprintf("%d.%d.%d", ver.MajorVersion, ver.MinorVersion, ver.BuildNumber)
}
