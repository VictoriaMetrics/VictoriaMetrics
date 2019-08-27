package netstorage

import (
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

func mustFadviseRandomRead(f *os.File) {
	fd := int(f.Fd())
	if err := unix.Fadvise(int(fd), 0, 0, unix.FADV_RANDOM|unix.FADV_WILLNEED); err != nil {
		logger.Panicf("FATAL: error returned from unix.Fadvise(RANDOM|WILLNEED): %s", err)
	}
}
