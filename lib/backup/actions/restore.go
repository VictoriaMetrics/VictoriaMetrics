package actions

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/formatutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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

	// SkipPreallocation may be set in order to skip preallocation of files during restore.
	//
	// This will likely be slower in most cases, but allows restores to resume mid file
	SkipPreallocation bool

	// RestoreSince, if non-zero, only restores partitions whose time range ends at or after
	// (now - RestoreSince). This allows restoring only recent data to reduce disk usage.
	//
	// For example, RestoreSince=5*24*time.Hour restores only the last 5 days of data.
	RestoreSince time.Duration

	// RestorePartitions is an optional list of partition names in YYYY_MM format to restore.
	// When non-empty, only the listed partitions are restored; all other partitions are skipped.
	// Non-partition files (metadata, etc.) are always restored regardless of this setting.
	RestorePartitions []string
}

// Run runs r with the provided settings.
func (r *Restore) Run(ctx context.Context) error {
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

	if !r.SkipPreallocation {
		dst.UseTmpFiles = true
	}

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

	if err := dst.CleanupTmpFiles(); err != nil {
		return fmt.Errorf("cannot cleanup temporary files at %s: %w", dst, err)
	}

	logger.Infof("obtaining list of parts at %s", src)
	srcParts, err := src.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list src parts: %w", err)
	}
	for _, srcPart := range srcParts {
		if !srcPart.IsLocalPathInsideDir(r.Dst.Dir) {
			return fmt.Errorf("part file %s would be written outside storage directory %s", srcPart.Path, r.Dst.Dir)
		}
	}
	if r.RestoreSince > 0 || len(r.RestorePartitions) > 0 {
		srcParts, err = filterPartitions(srcParts, r.RestoreSince, r.RestorePartitions, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("cannot filter partitions: %w", err)
		}
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
	if offset != pOld.FileSize {
		return fmt.Errorf("invalid size for %q; got %d; want %d", path, offset, pOld.FileSize)
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
	downloadSizeHuman := formatutil.HumanizeBytes(float64(downloadSize))
	if len(partsToCopy) > 0 {
		perPath := make(map[string][]common.Part)
		for _, p := range partsToCopy {
			parts := perPath[p.Path]
			parts = append(parts, p)
			perPath[p.Path] = parts
		}
		logger.Infof("downloading %d parts from %s to %s", len(partsToCopy), src, dst)
		var bytesDownloaded atomic.Uint64
		err = runParallelPerPath(ctx, concurrency, perPath, func(parts []common.Part) error {
			// Sort partsToCopy in order to properly grow file size during downloading
			common.SortParts(parts)
			if !r.SkipPreallocation {
				if err := dst.PreallocateFile(parts[0]); err != nil {
					return fmt.Errorf("cannot preallocate %s: %w", parts[0].Path, err)
				}
			}
			for _, p := range parts {
				logger.Infof("downloading %s from %s to %s", &p, src, dst)
				if err := pipelinedDownload(src, dst, p, &bytesDownloaded); err != nil {
					return err
				}
			}
			if err := dst.FinalizeFile(parts[0]); err != nil {
				return fmt.Errorf("cannot finalize %s: %w", parts[0].Path, err)
			}
			return nil
		}, func(elapsed time.Duration) {
			if elapsed.Seconds() <= 0 {
				// The only way for this to happen is when the operation is immediately canceled.
				// There is no need to log progress in this case, and this prevents division by zero below.
				return
			}
			n := bytesDownloaded.Load()
			downloadedHuman := formatutil.HumanizeBytes(float64(n))
			prc := 100 * float64(n) / float64(downloadSize)
			speed := float64(n) / elapsed.Seconds()
			estimatedTotal := time.Duration(float64(downloadSize)/speed) * time.Second
			eta := max(estimatedTotal-elapsed, 0)
			logger.Infof("downloaded %s out of %s bytes (%.2f%%) from %s to %s in %s; estimated time to completion: %s", downloadedHuman, downloadSizeHuman, prc, src, dst, elapsed, eta)
		})
		if err != nil {
			return err
		}
	}

	logger.Infof("restored %d bytes from backup in %.3f seconds; deleted %d bytes; downloaded %d bytes",
		backupSize, time.Since(startTime).Seconds(), deleteSize, downloadSize)

	removeRestoreLock(r.Dst.Dir)
	return nil
}

const writeBufSize = 2 * 1024 * 1024

var writeBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, writeBufSize)
		return &buf
	},
}

const readBufSize = 8 * 1024 * 1024

var readBufPool = sync.Pool{
	New: func() any {
		return bufio.NewWriterSize(nil, readBufSize)
	},
}

func pipelinedDownload(src common.RemoteFS, dst *fslocal.FS, p common.Part, bytesDownloaded *atomic.Uint64) error {
	wc, err := dst.NewDirectWriteCloser(p)
	if err != nil {
		return fmt.Errorf("cannot create writer for %q to %s: %w", &p, dst, err)
	}

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		buf := readBufPool.Get().(*bufio.Writer)
		buf.Reset(pw)
		err := src.DownloadPart(p, buf)
		if err == nil {
			err = buf.Flush()
		}
		readBufPool.Put(buf)
		pw.CloseWithError(err)
		errCh <- err
	}()

	sw := &statWriter{
		w:            wc,
		bytesWritten: bytesDownloaded,
	}
	bufp := writeBufPool.Get().(*[]byte)
	_, writeErr := io.CopyBuffer(sw, pr, *bufp)
	writeBufPool.Put(bufp)
	pr.Close()

	downloadErr := <-errCh

	closeErr := wc.Close()

	if writeErr != nil {
		return fmt.Errorf("cannot write %s to %s: %w", &p, dst, writeErr)
	}
	if downloadErr != nil {
		return fmt.Errorf("cannot download %s from %s: %w", &p, src, downloadErr)
	}
	if closeErr != nil {
		return fmt.Errorf("cannot close writer for %s at %s: %w", &p, dst, closeErr)
	}
	return nil
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

// partitionDirPrefixes are the directory prefixes that contain per-partition subdirectories in a backup.
var partitionDirPrefixes = []string{"data/small/", "data/big/", "data/indexdb/"}

// extractPartitionName returns the YYYY_MM partition name from a part path, or an empty string
// if the path does not belong to a partition directory (e.g. metadata files).
//
// Part paths in VictoriaMetrics backups follow the pattern:
//
//	data/small/YYYY_MM/...
//	data/big/YYYY_MM/...
//	data/indexdb/YYYY_MM/...
func extractPartitionName(partPath string) string {
	for _, prefix := range partitionDirPrefixes {
		if strings.HasPrefix(partPath, prefix) {
			rest := partPath[len(prefix):]
			// The partition name is the next path component (YYYY_MM).
			if idx := strings.IndexByte(rest, '/'); idx > 0 {
				return rest[:idx]
			}
			return rest
		}
	}
	return ""
}

// filterPartitions filters srcParts according to the restoreSince duration and the
// explicit restorePartitions list. Non-partition files are always retained.
//
// - If restoreSince > 0, only partitions whose month ends at or after (now - restoreSince) are kept.
// - If restorePartitions is non-empty, only the explicitly listed partitions are kept.
// - Both filters are applied when both are set (intersection).
//
// now is the reference time used for computing the restoreSince cutoff.
func filterPartitions(srcParts []common.Part, restoreSince time.Duration, restorePartitions []string, now time.Time) ([]common.Part, error) {
	if err := validatePartitionNames(restorePartitions); err != nil {
		return nil, err
	}

	allowedPartitions := make(map[string]struct{}, len(restorePartitions))
	for _, name := range restorePartitions {
		allowedPartitions[name] = struct{}{}
	}

	var sinceTime time.Time
	if restoreSince > 0 {
		sinceTime = now.Add(-restoreSince)
	}

	var keptParts []common.Part
	for _, p := range srcParts {
		ptName := extractPartitionName(p.Path)
		if ptName == "" {
			// Non-partition file (metadata, etc.) — always include.
			keptParts = append(keptParts, p)
			continue
		}

		if len(allowedPartitions) > 0 {
			if _, ok := allowedPartitions[ptName]; !ok {
				continue
			}
		}

		if sinceTime.IsZero() {
			keptParts = append(keptParts, p)
			continue
		}

		ptTime, err := time.Parse("2006_01", ptName)
		if err != nil {
			return nil, fmt.Errorf("cannot parse partition name %q from path %q: %w", ptName, p.Path, err)
		}
		// The partition covers the whole calendar month. Its end timestamp is
		// the start of the next month (exclusive). Include the partition only if
		// its end time is after sinceTime, i.e. the partition contains data newer
		// than sinceTime.
		y, m, _ := ptTime.Date()
		partitionEnd := time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
		if partitionEnd.After(sinceTime) {
			keptParts = append(keptParts, p)
		}
	}

	skipped := len(srcParts) - len(keptParts)
	if skipped > 0 {
		logger.Infof("skipped %d parts from %d partitions that are outside the requested restore range", skipped, countPartitions(srcParts)-countPartitions(keptParts))
	}
	return keptParts, nil
}

// validatePartitionNames returns an error if any name in names is not a valid YYYY_MM partition name.
func validatePartitionNames(names []string) error {
	for _, name := range names {
		if _, err := time.Parse("2006_01", name); err != nil {
			return fmt.Errorf("invalid partition name %q in -restorePartitions; expected YYYY_MM format, e.g. 2024_01", name)
		}
	}
	return nil
}

// countPartitions returns the number of distinct partition names in parts.
func countPartitions(parts []common.Part) int {
	m := make(map[string]struct{})
	for _, p := range parts {
		if name := extractPartitionName(p.Path); name != "" {
			m[name] = struct{}{}
		}
	}
	return len(m)
}

func createRestoreLock(dstDir string) error {
	lockF := path.Join(dstDir, backupnames.RestoreInProgressFilename)
	f, err := os.Create(lockF)
	if err != nil {
		return fmt.Errorf("cannot create restore lock file %q: %w", lockF, err)
	}
	return f.Close()
}

func removeRestoreLock(dstDir string) {
	lockF := path.Join(dstDir, backupnames.RestoreInProgressFilename)
	fs.MustRemovePath(lockF)
}
