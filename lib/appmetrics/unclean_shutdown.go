package appmetrics

import (
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// UncleanShutdownMarkerFilename is the marker file used to detect a previous unclean shutdown.
const UncleanShutdownMarkerFilename = ".vm_app_running"

var (
	uncleanShutdownEnabled atomic.Bool
	uncleanShutdown        uint64
)

// MustCreateUncleanShutdownMarker creates an UncleanShutdownMarkerFilename marker file in the given dirPath.
// Must be called once on program startup and paired with a single MustRemoveUncleanShutdownMarker call on exit.
//
// If the marker file already exists on startup, it indicates a previous unclean shutdown and uncleanShutdown is set to 1.
func MustCreateUncleanShutdownMarker(dirPath string) {
	if !uncleanShutdownEnabled.CompareAndSwap(false, true) {
		logger.Fatalf("BUG: unclean shutdown marker was already initialized. It could only be called once")
	}
	marker := filepath.Join(dirPath, UncleanShutdownMarkerFilename)
	f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		fs.MustClose(f)
		return
	}
	if os.IsExist(err) {
		uncleanShutdown = 1
		logger.Warnf("Previous shutdown was unclean since file %q exists. Please check logs and investigate the reason of unclean shutdown", marker)
		return
	}
	logger.Panicf("FATAL: cannot create unclean shutdown marker %q: %s", marker, err)
}

// MustRemoveUncleanShutdownMarker removes the UncleanShutdownMarkerFilename marker file created by MustCreateUncleanShutdownMarker.
// Must be called once, as late as possible before program exit.
func MustRemoveUncleanShutdownMarker(dirPath string) {
	if !uncleanShutdownEnabled.Load() {
		logger.Fatalf("BUG: unclean shutdown marker was not initialized with MustCreateUncleanShutdownMarker call")
	}
	marker := filepath.Join(dirPath, UncleanShutdownMarkerFilename)
	if err := os.Remove(marker); err != nil {
		logger.Fatalf("FATAL: cannot remove unclean shutdown marker %q: %s", marker, err)
	}
	fs.MustSyncPath(dirPath)
}
