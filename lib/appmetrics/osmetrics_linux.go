package appmetrics

import (
	"syscall"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func initOS() {
	initedOS = osInfo{os: "linux"}

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
	initedOS.release = string(ur)
}
