package appmetrics

import (
	stdos "os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// UncleanShutdownMarkerFilename is the marker file used by MustStartUncleanShutdownMarker.
const UncleanShutdownMarkerFilename = ".vm_app_running"

var (
	uncleanShutdownMarkerDirPath string

	uncleanShutdownMetricEnabled atomic.Bool
	startedAfterUncleanShutdown  atomic.Uint64
)

// MustStartUncleanShutdownMarker creates a durable running marker in dirPath.
//
// If the marker already exists, vm_app_started_after_unclean_shutdown is set to 1.
// Must be paired with a single MustStopUncleanShutdownMarker call.
func MustStartUncleanShutdownMarker(dirPath string) {
	if uncleanShutdownMarkerDirPath != "" {
		logger.Panicf("BUG: MustStartUncleanShutdownMarker has been already called")
	}

	fs.MustMkdirIfNotExist(dirPath)
	fs.MustSyncPathAndParentDir(dirPath)

	markerPath := filepath.Join(dirPath, UncleanShutdownMarkerFilename)
	if mustCreateUncleanShutdownMarker(markerPath) {
		fs.MustSyncPath(dirPath)
	} else {
		startedAfterUncleanShutdown.Store(1)
		logger.Warnf("detected an unclean shutdown because %q exists", markerPath)
	}

	uncleanShutdownMarkerDirPath = dirPath
	uncleanShutdownMetricEnabled.Store(true)
}

// MustStopUncleanShutdownMarker removes the running marker created by MustStartUncleanShutdownMarker.
func MustStopUncleanShutdownMarker() {
	if uncleanShutdownMarkerDirPath == "" {
		logger.Panicf("BUG: MustStartUncleanShutdownMarker must be called before MustStopUncleanShutdownMarker")
	}

	dirPath := uncleanShutdownMarkerDirPath
	markerPath := filepath.Join(dirPath, UncleanShutdownMarkerFilename)
	if err := removeUncleanShutdownMarker(markerPath, stdos.Remove, time.Sleep); err != nil {
		logger.Panicf("FATAL: cannot remove %q after %d attempts: %s", markerPath, uncleanShutdownMarkerRemoveAttempts, err)
	}
	fs.MustSyncPath(dirPath)
	uncleanShutdownMarkerDirPath = ""
}

// mustCreateUncleanShutdownMarker atomically creates markerPath and returns true.
// It returns false if markerPath already exists.
func mustCreateUncleanShutdownMarker(markerPath string) bool {
	f, err := stdos.OpenFile(markerPath, stdos.O_WRONLY|stdos.O_CREATE|stdos.O_EXCL, 0600)
	if err != nil {
		if stdos.IsExist(err) {
			return false
		}
		logger.Panicf("FATAL: cannot create unclean shutdown marker %q: %s", markerPath, err)
	}
	fs.MustClose(f)
	fs.MustSyncPath(markerPath)
	return true
}

const (
	uncleanShutdownMarkerRemoveAttempts = 10
	uncleanShutdownMarkerRemoveRetry    = time.Second
)

func removeUncleanShutdownMarker(markerPath string, remove func(string) error, sleep func(time.Duration)) error {
	var err error
	for attempt := 1; attempt <= uncleanShutdownMarkerRemoveAttempts; attempt++ {
		err = remove(markerPath)
		if err == nil || stdos.IsNotExist(err) {
			return nil
		}
		if attempt < uncleanShutdownMarkerRemoveAttempts {
			logger.Warnf("cannot remove %q: %s; retrying in %s", markerPath, err, uncleanShutdownMarkerRemoveRetry)
			sleep(uncleanShutdownMarkerRemoveRetry)
		}
	}
	return err
}
