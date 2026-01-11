package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// directories with this filename are scheduled to be removed by MustRemoveDir().
const deleteDirFilename = ".delete-this-dir"

// MustRemoveDir removes the dirPath with all its contents.
//
// The dirPath contents may be partially deleted if unclean shutdown happens during the removal.
// The caller must verify whether the given directory is partially removed via IsPartiallyRemovedDir() call
// on the startup before using it. If the directory is partially removed, it must be removed again
// via MustRemoveDir() call.
func MustRemoveDir(dirPath string) {
	if !IsPathExist(dirPath) {
		// Nothing do delete.
		return
	}

	// The code below is written in the way that partially deleted directories could be deleted
	// on the next start after unclean shutdown, by verifying them with IsPartiallyRemovedDir() call.
	//
	// The code below doesn't depend on atomic renaming of directories, since it isn't supported
	// by NFS and object storage.

	// Create a deleteDirFilename file, which indicates that the dirPath must be removed.
	deleteFilePath := filepath.Join(dirPath, deleteDirFilename)
	f, err := os.Create(deleteFilePath)
	if err != nil {
		logger.Panicf("FATAL: cannot create %q while deleting %q: %s", deleteFilePath, dirPath, err)
	}
	if err := f.Close(); err != nil {
		logger.Panicf("FATAL: cannot close %q: %s", deleteFilePath, err)
	}

	// Make sure the deleteDirFilename file is visible in the dirPath.
	MustSyncPath(dirPath)

	// Remove the contents of the dirPath except of the deleteDirFilename file.
	//
	// Make this in parallel in order to reduce the time needed for the removal of big number of items
	// on high-latency storage systems such as NFS.
	// Directories for VitoriaLogs parts may contain big number of items when wide events are stored there.
	// Also the number of parts in a partition may be quite big.

	if tryRemoveDir(dirPath) {
		return
	}

	// schedule NFS background dir removal.
	// NFS may perform "silly rename" before deletion, if client detects more than 1 file reference.
	// Silly raname is async operation and client may take an additional time before
	// unlink operation will succeed and could be actually deleted.
	select {
	case removeDirConcurrencyCh <- struct{}{}:
	default:
		logger.Panicf("FATAL: cannot schedule %s for removal, since the removal queue is full (%d entries)", dirPath, cap(removeDirConcurrencyCh))
	}
	dirRemoverWG.Go(func() {
		for {
			if tryRemoveDir(dirPath) {
				return
			}
			time.Sleep(time.Second)
		}
	})
}

// IsPartiallyRemovedDir returns true if dirPath is partially removed because of unclean shutdown during the MustRemoveDir() call.
//
// The caller must call MustRemoveDir(dirPath) on partially removed dirPath.
func IsPartiallyRemovedDir(dirPath string) bool {
	des := MustReadDir(dirPath)
	if len(des) == 0 {
		// Delete empty dirs too, since they may appear when the unclean shutdown happens after the deleteDirFilename is deleted,
		// but before the directory is deleted itself.
		return true
	}

	deleteFilePath := filepath.Join(dirPath, deleteDirFilename)
	for _, de := range des {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if name == deleteFilePath {
			// The directory contains the deleteDirFilename. This means it is partially deleted.
			return true
		}
	}
	return false
}

// MustRemovePath removes the given path. It must be either a file or an empty directory.
//
// Use MustRemoveDir for removing non-empty directories.
func MustRemovePath(path string) {
	if err := os.Remove(path); err != nil {
		logger.Panicf("FATAL: cannot remove %q: %s", path, err)
	}
}

// MustRemoveDirContents removes all the contents of the given dir if it exists.
//
// It doesn't remove the dir itself, so the dir may be mounted to a separate partition.
func MustRemoveDirContents(dir string) {
	if !IsPathExist(dir) {
		// The path doesn't exist, so nothing to remove.
		return
	}

	des := MustReadDir(dir)
	for _, de := range des {
		name := de.Name()
		fullPath := filepath.Join(dir, name)
		if err := os.RemoveAll(fullPath); err != nil {
			logger.Panicf("FATAL: cannot remove %s: %s", fullPath, err)
		}
	}
	MustSyncPath(dir)
}

// tryRemoveDir performs removal of directory
// it checks error for the first NFS temporary error
func tryRemoveDir(dirPath string) bool {
	des := MustReadDir(dirPath)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	concurrencyCh := make(chan struct{}, min(32, len(des)-1))
	for _, de := range des {
		name := de.Name()
		if name == deleteDirFilename {
			continue
		}
		dirEntryPath := filepath.Join(dirPath, name)

		concurrencyCh <- struct{}{}
		wg.Add(1)
		go func(dirEntryPath string) {
			defer func() {
				wg.Done()
				<-concurrencyCh
			}()
			if err := os.RemoveAll(dirEntryPath); err != nil {
				mu.Lock()
				if !isTemporaryNFSError(err) {
					logger.Panicf("FATAL: cannot remove %q: %s", dirEntryPath, err)
				}
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(dirEntryPath)
	}
	wg.Wait()
	if firstErr != nil {
		nfsDirRemoveFailedAttempts.Inc()
		return false
	}
	// Make sure the deleted names are properly synced to the dirPath,
	// so they are no longer visible after unclean shutdown.
	MustSyncPath(dirPath)

	deleteFilePath := filepath.Join(dirPath, deleteDirFilename)
	// Remove the deleteDirFilename file, since there are no other entries left in the directory.
	MustRemovePath(deleteFilePath)

	// Sync the directory after the removing deletDirFilename file in order to make sure
	// all the metadata files are removed at some exotic filesystems such as OSSFS2.
	// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/649
	// and https://github.com/VictoriaMetrics/VictoriaMetrics/pull/9709
	MustSyncPath(dirPath)

	// Remove the dirPath itself
	MustRemovePath(dirPath)

	// Do not sync the parent directory for the dirPath - the caller can do this if needed.
	// It is OK if the dirPath will remain undeleted after unclean shutdown - it will be deleted
	// on the next startup.

	return true
}

var (
	dirRemoverWG               sync.WaitGroup
	nfsDirRemoveFailedAttempts = metrics.NewCounter(`vm_nfs_dir_remove_failed_attempts_total`)
	_                          = metrics.NewGauge(`vm_nfs_pending_dirs_to_remove`, func() float64 {
		return float64(len(removeDirConcurrencyCh))
	})
)

var removeDirConcurrencyCh = make(chan struct{}, 1024)

func isTemporaryNFSError(err error) bool {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		// Some NFS implementations return EEXIST instead of ENOTEMPTY
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6398
		if errors.Is(pathErr.Err, syscall.EEXIST) || errors.Is(pathErr.Err, syscall.ENOTEMPTY) {
			return true
		}
	}
	// Do not check for NFS file handle error, usually it means that other client has openned file without proper lock
	// in this scenario it's better to panic.
	// User must configure proper locking options for the NFS client to prevent such error.
	// It must never have "nolock" or "local_lock=all" options to be set.

	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/61 for details.
	errStr := err.Error()
	return strings.Contains(errStr, "directory not empty") || strings.Contains(errStr, "device or resource busy")
}

// MustStopDirRemover must be called in the end of graceful shutdown
// in order to wait for removing the remaining directories from removeDirConcurrencyCh.
//
// It is expected that nobody calls MustRemoveAll when MustStopDirRemover is called.
func MustStopDirRemover() {
	doneCh := make(chan struct{})
	go func() {
		dirRemoverWG.Wait()
		close(doneCh)
	}()
	const maxWaitTime = 10 * time.Second
	select {
	case <-doneCh:
		return
	case <-time.After(maxWaitTime):
		logger.Errorf("cannot stop dirRemover in %s; the remaining empty NFS directories should be automatically removed on the next startup", maxWaitTime)
	}
}
