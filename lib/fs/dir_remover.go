package fs

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

func mustRemoveAll(path string, done func()) bool {
	err := os.RemoveAll(path)
	if err == nil {
		// Make sure the parent directory doesn't contain references
		// to the current directory.
		mustSyncParentDirIfExists(path)
		done()
		return true
	}
	if !isTemporaryNFSError(err) {
		logger.Panicf("FATAL: cannot remove %q: %s", path, err)
	}
	// NFS prevents from removing directories with open files.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 .
	// Schedule for later directory removal.
	nfsDirRemoveFailedAttempts.Inc()
	w := &removeDirWork{
		path: path,
		done: done,
	}
	select {
	case removeDirCh <- w:
	default:
		logger.Panicf("FATAL: cannot schedule %s for removal, since the removal queue is full (%d entries)", path, cap(removeDirCh))
	}
	return false
}

var nfsDirRemoveFailedAttempts = metrics.NewCounter(`vm_nfs_dir_remove_failed_attempts_total`)

type removeDirWork struct {
	path string
	done func()
}

var removeDirCh = make(chan *removeDirWork, 1024)

func dirRemover() {
	const minSleepTime = 100 * time.Millisecond
	const maxSleepTime = time.Second
	sleepTime := minSleepTime
	for {
		var w *removeDirWork
		select {
		case w = <-removeDirCh:
		default:
			if atomic.LoadUint64(&stopDirRemover) != 0 {
				return
			}
			time.Sleep(minSleepTime)
			continue
		}
		if mustRemoveAll(w.path, w.done) {
			sleepTime = minSleepTime
			continue
		}

		// Couldn't remove the directory at the path because of NFS lock.
		// Sleep for a while and try again.
		// Do not limit the amount of time required for deleting the directory,
		// since this may break on laggy NFS.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/162 .
		time.Sleep(sleepTime)
		if sleepTime < maxSleepTime {
			sleepTime *= 2
		} else {
			logger.Errorf("failed to remove directory %q due to NFS lock; retrying later", w.path)
		}
	}
}

func isTemporaryNFSError(err error) bool {
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 for details.
	errStr := err.Error()
	return strings.Contains(errStr, "directory not empty") || strings.Contains(errStr, "device or resource busy")
}

var dirRemoverWG sync.WaitGroup
var stopDirRemover uint64

func init() {
	dirRemoverWG.Add(1)
	go func() {
		defer dirRemoverWG.Done()
		dirRemover()
	}()
}

// MustStopDirRemover must be called in the end of graceful shutdown
// in order to wait for removing the remaining directories from removeDirCh.
//
// It is expected that nobody calls MustRemoveAll when MustStopDirRemover
// is called.
func MustStopDirRemover() {
	atomic.StoreUint64(&stopDirRemover, 1)
	doneCh := make(chan struct{})
	go func() {
		dirRemoverWG.Wait()
		close(doneCh)
	}()
	const maxWaitTime = 5 * time.Second
	select {
	case <-doneCh:
		return
	case <-time.After(maxWaitTime):
		logger.Panicf("FATAL: cannot stop dirRemover in %s", maxWaitTime)
	}
}
