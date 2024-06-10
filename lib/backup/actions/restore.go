package actions

import (
	"fmt"
	"io"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// Restore restores data according to the provided settings.
//
// Note that the restore works only for VictoriaMetrics backups made from snapshots.
// It works improperly on mutable files.
type Restore struct {
	// Concurrency is the number of concurrent workers to run during restore.
	Concurrency int

	// Src is the source containing backed up data.
	Src common.RemoteFS

	// Dst is destination to restore the data.
	//
	// If dst points to existing directory, then incremental restore is performed,
	// i.e. only new data is downloaded from src.
	Dst *fslocal.FS

	// SkipBackupCompleteCheck may be set in order to skip for `backup complete` file in Src.
	//
	// This may be needed for restoring from old backups with missing `backup complete` file.
	SkipBackupCompleteCheck bool
}

// Run runs r with the provided settings.
func (r *Restore) Run() error {
	startTime := time.Now()

	// Make sure VictoriaMetrics doesn't run during the restore process.
	fs.MustMkdirIfNotExist(r.Dst.Dir)
	flockF := fs.MustCreateFlockFile(r.Dst.Dir)
	defer fs.MustClose(flockF)

	if err := createRestoreLock(r.Dst.Dir); err != nil {
		return err
	}
	concurrency := r.Concurrency
	src := r.Src
	dst := r.Dst

	if !r.SkipBackupCompleteCheck {
		ok, err := src.HasFile(backupnames.BackupCompleteFilename)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("cannot find %s file in %s; this means either incomplete backup or old backup; "+
				"pass -skipBackupCompleteCheck command-line flag if you still need restoring from this backup", backupnames.BackupCompleteFilename, src)
		}
	}

	logger.Infof("starting restore from %s to %s", src, dst)

	logger.Infof("obtaining list of parts at %s", src)
	srcParts, err := src.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list src parts: %w", err)
	}
	logger.Infof("obtaining list of parts at %s", dst)
	dstParts, err := dst.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list dst parts: %w", err)
	}

	backupSize := getPartsSize(srcParts)

	// Validate srcParts. They must cover the whole files.
	common.SortParts(srcParts)
	offset := uint64(0)
	var pOld common.Part
	var path string
	for _, p := range srcParts {
		if p.Path != path {
			if offset != pOld.FileSize {
				return fmt.Errorf("invalid size for %q; got %d; want %d", path, offset, pOld.FileSize)
			}
			pOld = p
			path = p.Path
			offset = 0
		}
		if p.Offset < offset {
			return fmt.Errorf("there is an overlap in %d bytes between %s and %s", offset-p.Offset, &pOld, &p)
		}
		if p.Offset > offset {
			if offset == 0 {
				return fmt.Errorf("there is a gap in %d bytes from file start to %s", p.Offset, &p)
			}
			return fmt.Errorf("there is a gap in %d bytes between %s and %s", p.Offset-offset, &pOld, &p)
		}
		if p.Size != p.ActualSize {
			return fmt.Errorf("invalid size for %s; got %d; want %d", &p, p.ActualSize, p.Size)
		}
		offset += p.Size
	}

	partsToDelete := common.PartsDifference(dstParts, srcParts)
	deleteSize := uint64(0)
	if len(partsToDelete) > 0 {
		// Remove only files with the missing part at offset 0.
		// Assume other files are partially downloaded during the previous Restore.Run call,
		// so only the last part in them may be incomplete.
		// The last part for partially downloaded files will be re-downloaded later.
		// This addresses https://github.com/VictoriaMetrics/VictoriaMetrics/issues/487 .
		pathsToDelete := make(map[string]bool)
		for _, p := range partsToDelete {
			if p.Offset == 0 {
				pathsToDelete[p.Path] = true
			}
		}
		logger.Infof("deleting %d files from %s", len(pathsToDelete), dst)
		for path := range pathsToDelete {
			logger.Infof("deleting %s from %s", path, dst)
			size, err := dst.DeletePath(path)
			if err != nil {
				return fmt.Errorf("cannot delete %s from %s: %w", path, dst, err)
			}
			deleteSize += size
		}
		if err := dst.RemoveEmptyDirs(); err != nil {
			return fmt.Errorf("cannot remove empty directories at %s: %w", dst, err)
		}
	}

	// Re-read dstParts, since additional parts may be removed on the previous step.
	dstParts, err = dst.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list dst parts after the deletion: %w", err)
	}

	partsToCopy := common.PartsDifference(srcParts, dstParts)
	downloadSize := getPartsSize(partsToCopy)
	if len(partsToCopy) > 0 {
		perPath := make(map[string][]common.Part)
		for _, p := range partsToCopy {
			parts := perPath[p.Path]
			parts = append(parts, p)
			perPath[p.Path] = parts
		}
		logger.Infof("downloading %d parts from %s to %s", len(partsToCopy), src, dst)
		var bytesDownloaded atomic.Uint64
		err = runParallelPerPath(concurrency, perPath, func(parts []common.Part) error {
			// Sort partsToCopy in order to properly grow file size during downloading
			// and to properly resume downloading of incomplete files on the next Restore.Run call.
			common.SortParts(parts)
			for _, p := range parts {
				logger.Infof("downloading %s from %s to %s", &p, src, dst)
				wc, err := dst.NewWriteCloser(p)
				if err != nil {
					return fmt.Errorf("cannot create writer for %q to %s: %w", &p, dst, err)
				}
				sw := &statWriter{
					w:            wc,
					bytesWritten: &bytesDownloaded,
				}
				if err := src.DownloadPart(p, sw); err != nil {
					return fmt.Errorf("cannot download %s to %s: %w", &p, dst, err)
				}
				if err := wc.Close(); err != nil {
					return fmt.Errorf("cannot close reader from %s from %s: %w", &p, src, err)
				}
			}
			return nil
		}, func(elapsed time.Duration) {
			n := bytesDownloaded.Load()
			prc := 100 * float64(n) / float64(downloadSize)
			logger.Infof("downloaded %d out of %d bytes (%.2f%%) from %s to %s in %s", n, downloadSize, prc, src, dst, elapsed)
		})
		if err != nil {
			return err
		}
	}

	logger.Infof("restored %d bytes from backup in %.3f seconds; deleted %d bytes; downloaded %d bytes",
		backupSize, time.Since(startTime).Seconds(), deleteSize, downloadSize)

	return removeRestoreLock(r.Dst.Dir)
}

type statWriter struct {
	w            io.Writer
	bytesWritten *atomic.Uint64
}

func (sw *statWriter) Write(p []byte) (int, error) {
	n, err := sw.w.Write(p)
	sw.bytesWritten.Add(uint64(n))
	bytesDownloadedTotal.Add(n)
	return n, err
}

var bytesDownloadedTotal = metrics.NewCounter(`vm_backups_downloaded_bytes_total`)

func createRestoreLock(dstDir string) error {
	lockF := path.Join(dstDir, backupnames.RestoreInProgressFilename)
	f, err := os.Create(lockF)
	if err != nil {
		return fmt.Errorf("cannot create restore lock file %q: %w", lockF, err)
	}
	return f.Close()
}

func removeRestoreLock(dstDir string) error {
	lockF := path.Join(dstDir, backupnames.RestoreInProgressFilename)
	if err := os.Remove(lockF); err != nil {
		return fmt.Errorf("cannote remove restore lock file %q: %w", lockF, err)
	}
	return nil
}
