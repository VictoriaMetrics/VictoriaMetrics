package actions

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/s3remote"
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

	// RestoreSince, if non-zero, restores every monthly partition that overlaps the last RestoreSince.
	// Partitions are restored whole, so they may contain data older than RestoreSince.
	//
	// For example, RestoreSince=5*24*time.Hour on 2026-11-03 restores 2026_10 and 2026_11.
	RestoreSince time.Duration

	// RestorePartitions, if non-nil, is a regular expression matched against partition
	// names (YYYY_MM). Only partitions whose name matches are restored; all other partitions
	// are skipped. Non-partition files (metadata, etc.) are always restored regardless of
	// this setting.
	RestorePartitions *regexp.Regexp

	// AllowPartialRestoreWithoutLegacyIndex allows a partial restore of a backup containing the legacy non-partitioned indexdb.
	AllowPartialRestoreWithoutLegacyIndex bool
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

	dst.UseTmpFiles = !r.SkipPreallocation

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
	if r.RestoreSince > 0 || r.RestorePartitions != nil {
		if slices.ContainsFunc(srcParts, func(p common.Part) bool {
			return strings.HasPrefix(p.Path, "indexdb/")
		}) {
			if !r.AllowPartialRestoreWithoutLegacyIndex {
				return fmt.Errorf("the backup contains the legacy non-partitioned IndexDB, which isn't restored when -restoreSince or -restorePartitions is set; " +
					"data written before the upgrade to v1.133.0 won't be searchable after the restore; " +
					"run a full restore if you need such data, or pass -allowPartialRestoreWithoutLegacyIndex if all the selected partitions were written after the upgrade; " +
					"see https://docs.victoriametrics.com/victoriametrics/vmrestore/#partial-restore")
			}
			logger.Warnf("skipping the legacy non-partitioned IndexDB, since -allowPartialRestoreWithoutLegacyIndex is set; " +
				"data in the selected partitions written before the upgrade to v1.133.0 won't be searchable after the restore")
		}
		srcParts, err = filterPartitions(srcParts, r.RestoreSince, r.RestorePartitions, time.Now().UTC())
		if err != nil {
			return err
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
		err = r.runParallelPerPath(ctx, concurrency, perPath, &bytesDownloaded, func(elapsed time.Duration) {
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

func (r *Restore) runParallelPerPath(ctx context.Context, concurrency int, perPath map[string][]common.Part, bytesDownloaded *atomic.Uint64, progress func(elapsed time.Duration)) error {
	_, isS3 := r.Src.(*s3remote.FS)
	var err error
	runWithProgress(progress, func() {
		err = r.runParallelPerPathInternal(ctx, concurrency, isS3, perPath, bytesDownloaded)
	})
	return err
}

func (r *Restore) runParallelPerPathInternal(ctx context.Context, concurrency int, isS3 bool, perPath map[string][]common.Part, bytesDownloaded *atomic.Uint64) error {
	concurrency = max(concurrency, 1)
	perFile := concurrency
	if !isS3 || r.SkipPreallocation {
		// Keep non-S3 sources sequential per file. Resuming by file size also
		// requires completed writes to form a contiguous prefix.
		perFile = 1
	}

	type fileToRestore struct {
		parts       []common.Part
		nextPart    atomic.Int64
		remaining   atomic.Int64
		prepareOnce sync.Once
		prepareErr  error
	}
	var workers []*fileToRestore
	for _, parts := range perPath {
		if len(parts) == 0 {
			continue
		}
		common.SortParts(parts)
		f := &fileToRestore{parts: parts}
		f.remaining.Store(int64(len(parts)))
		for range min(perFile, len(parts)) {
			workers = append(workers, f)
		}
	}
	workCh := make(chan *fileToRestore, len(workers))
	for _, f := range workers {
		workCh <- f
	}
	close(workCh)

	ctxLocal, cancelLocal := context.WithCancel(ctx)
	defer cancelLocal()

	var errOnce sync.Once
	var firstErr error
	setErr := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancelLocal()
		})
	}

	var wg sync.WaitGroup
	for range min(concurrency, len(workers)) {
		wg.Go(func() {
			for f := range workCh {
				if ctxLocal.Err() != nil {
					return
				}
				f.prepareOnce.Do(func() {
					if !r.SkipPreallocation {
						f.prepareErr = r.Dst.PreallocateFile(f.parts[0])
					}
				})
				if f.prepareErr != nil {
					setErr(fmt.Errorf("cannot preallocate %s: %w", f.parts[0].Path, f.prepareErr))
					return
				}
				for {
					if ctxLocal.Err() != nil {
						return
					}
					i := f.nextPart.Add(1) - 1
					if i >= int64(len(f.parts)) {
						break
					}
					p := f.parts[i]
					if err := r.downloadPart(ctxLocal, p, bytesDownloaded, isS3); err != nil {
						setErr(err)
						return
					}
					if f.remaining.Add(-1) == 0 {
						if err := r.Dst.FinalizeFile(p); err != nil {
							setErr(fmt.Errorf("cannot finalize %s: %w", p.Path, err))
							return
						}
					}
				}
			}
		})
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

func (r *Restore) downloadPart(ctx context.Context, p common.Part, bytesDownloaded *atomic.Uint64, retryEOF bool) error {
	const maxAttempts = 3
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		logger.Infof("downloading %s from %s to %s", &p, r.Src, r.Dst)
		n, err := pipelinedDownload(r.Src, r.Dst, p, bytesDownloaded)
		if err == nil || !retryEOF || !errors.Is(err, io.ErrUnexpectedEOF) {
			return err
		}
		if attempt == maxAttempts {
			return fmt.Errorf("download of %s failed after %d attempts: %w", &p, attempt, err)
		}
		// Retried bytes overwrite the same part, so don't count them twice in restore progress.
		// The downloaded-bytes metric still includes the actual bytes transferred.
		bytesDownloaded.Add(-n)
		delay := time.Duration(attempt) * time.Second
		logger.Warnf("download of %s interrupted: %s; retrying the whole part in %s (attempt %d/%d)", &p, err, delay, attempt+1, maxAttempts)
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
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

func pipelinedDownload(src common.RemoteFS, dst *fslocal.FS, p common.Part, bytesDownloaded *atomic.Uint64) (uint64, error) {
	wc, err := dst.NewDirectWriteCloser(p)
	if err != nil {
		return 0, fmt.Errorf("cannot create writer for %q to %s: %w", &p, dst, err)
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
		return sw.n, fmt.Errorf("cannot write %s to %s: %w", &p, dst, writeErr)
	}
	if downloadErr != nil {
		return sw.n, fmt.Errorf("cannot download %s from %s: %w", &p, src, downloadErr)
	}
	if closeErr != nil {
		return sw.n, fmt.Errorf("cannot close writer for %s at %s: %w", &p, dst, closeErr)
	}
	return sw.n, nil
}

type statWriter struct {
	w            io.Writer
	bytesWritten *atomic.Uint64
	n            uint64
}

func (sw *statWriter) Write(p []byte) (int, error) {
	n, err := sw.w.Write(p)
	sw.n += uint64(n)
	sw.bytesWritten.Add(uint64(n))
	bytesDownloadedTotal.Add(n)
	return n, err
}

var bytesDownloadedTotal = metrics.NewCounter(`vm_backups_downloaded_bytes_total`)

// extractPartitionName returns YYYY_MM for a data/{small,big,indexdb}/YYYY_MM/... path, or "" otherwise.
func extractPartitionName(partPath string) string {
	var rest string
	switch {
	case strings.HasPrefix(partPath, "data/small/"):
		rest = partPath[len("data/small/"):]
	case strings.HasPrefix(partPath, "data/big/"):
		rest = partPath[len("data/big/"):]
	case strings.HasPrefix(partPath, "data/indexdb/"):
		rest = partPath[len("data/indexdb/"):]
	default:
		return ""
	}
	ptName, _, ok := strings.Cut(rest, "/")
	if !ok {
		return ""
	}
	if _, err := time.Parse("2006_01", ptName); err != nil {
		return ""
	}
	return ptName
}

// filterPartitions keeps non-partition parts and the partitions selected by restoreSince and partitionRe.
// The legacy non-partitioned indexdb is skipped. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7599
// It returns an error if the backup contains partitions, but none of them is selected.
func filterPartitions(srcParts []common.Part, restoreSince time.Duration, partitionRe *regexp.Regexp, now time.Time) ([]common.Part, error) {
	var sinceTime time.Time
	if restoreSince > 0 {
		sinceTime = now.Add(-restoreSince)
	}

	var keptParts []common.Part
	keptPartitions := make(map[string]struct{})
	skippedPartitions := make(map[string]struct{})
	for _, p := range srcParts {
		// Skip the legacy indexdb: it can't be matched against partitions,
		// and it is usually big, so restoring it would defeat the disk savings.
		if strings.HasPrefix(p.Path, "indexdb/") {
			continue
		}

		ptName := extractPartitionName(p.Path)
		if ptName == "" {
			// Non-partition file (snapshots, etc.) — always include.
			keptParts = append(keptParts, p)
			continue
		}

		if partitionRe != nil && !partitionRe.MatchString(ptName) {
			skippedPartitions[ptName] = struct{}{}
			continue
		}

		if sinceTime.IsZero() {
			keptParts = append(keptParts, p)
			keptPartitions[ptName] = struct{}{}
			continue
		}

		ptTime, err := time.Parse("2006_01", ptName)
		if err != nil {
			logger.Panicf("FATAL: cannot parse partition name %q from path %q: %s", ptName, p.Path, err)
		}

		// Keep the whole monthly partition if it overlaps the last restoreSince, i.e. [sinceTime, now]
		y, m, _ := ptTime.Date()
		partitionEnd := time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
		if !partitionEnd.After(sinceTime) {
			skippedPartitions[ptName] = struct{}{}
			continue
		}
		keptParts = append(keptParts, p)
		keptPartitions[ptName] = struct{}{}
	}

	if len(skippedPartitions) > 0 && len(keptPartitions) == 0 {
		return nil, fmt.Errorf("-restoreSince and -restorePartitions don't select any of %d partitions in the backup; check the flag values", len(skippedPartitions))
	}
	if len(skippedPartitions) > 0 {
		logger.Infof("skipped %d partitions outside the requested restore range", len(skippedPartitions))
	}
	return keptParts, nil
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
