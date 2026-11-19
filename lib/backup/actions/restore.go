package actions

import (
	"bufio"
	"context"
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

	// ConcurrencyPerFile is the maximum number of concurrent workers to run for a single restored file.
	ConcurrencyPerFile int

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
	var err error
	runWithProgress(progress, func() {
		err = r.runParallelPerPathInternal(ctx, concurrency, perPath, bytesDownloaded)
	})
	return err
}

func (r *Restore) runParallelPerPathInternal(ctx context.Context, concurrency int, perPath map[string][]common.Part, bytesDownloaded *atomic.Uint64) error {
	if concurrency <= 0 {
		concurrency = 1
	}
	if len(perPath) == 0 {
		return nil
	}

	ctxLocal, cancelLocal := context.WithCancel(ctx)
	defer cancelLocal()

	// Build work queue with intelligently split parts
	workQueue := r.buildAdaptiveWorkQueue(perPath, concurrency)
	if len(workQueue) == 0 {
		return nil
	}

	// Use global work queue with adaptive worker scheduling
	return r.downloadWithGlobalQueue(ctxLocal, concurrency, workQueue, bytesDownloaded)
}

// workQueueItem represents a single part to download, including metadata for safe concurrent writes.
type workQueueItem struct {
	part common.Part
	// fileWriteLock ensures sequential writes to the same file (for preallocation safety)
	fileWriteLock *sync.Mutex
	// isFirstPart indicates if this is the first part of a file (needs preallocation)
	isFirstPart bool
	// isLastPart indicates if this is the last part of a file (needs finalization)
	isLastPart bool
}

// buildAdaptiveWorkQueue creates a work queue with intelligently split parts
func (r *Restore) buildAdaptiveWorkQueue(perPath map[string][]common.Part, concurrency int) []*workQueueItem {
	optimalPartSize := calculateOptimalPartSize(concurrency)
	fileWriteLocks := make(map[string]*sync.Mutex)

	var workQueue []*workQueueItem
	for _, parts := range perPath {
		common.SortParts(parts)
		for _, p := range parts {
			// Get or create lock for this file
			if _, exists := fileWriteLocks[p.Path]; !exists {
				fileWriteLocks[p.Path] = &sync.Mutex{}
			}
			fileLock := fileWriteLocks[p.Path]

			// Split large parts for concurrent download
			splitParts := splitPartAdaptive(p, optimalPartSize)
			for i, splitPart := range splitParts {
				item := &workQueueItem{
					part:          splitPart,
					fileWriteLock: fileLock,
					isFirstPart:   (i == 0 && p == parts[0]),
					isLastPart:    (i == len(splitParts)-1 && p == parts[len(parts)-1]),
				}
				workQueue = append(workQueue, item)
			}
		}
	}
	return workQueue
}

// calculateOptimalPartSize returns the ideal part size for concurrent downloads
func calculateOptimalPartSize(concurrency int) uint64 {
	const (
		minPartSize = 50 * 1024 * 1024    // 50MB minimum (S3 request overhead)
		maxPartSize = 500 * 1024 * 1024   // 500MB maximum
	)

	// Heuristic: smaller parts with more workers for better load balancing
	if concurrency <= 2 {
		return maxPartSize // Few workers, use big parts
	}
	if concurrency >= 10 {
		return minPartSize // Many workers, use smaller parts for better balance
	}
	// 3-9 workers: use middle ground
	return 100 * 1024 * 1024
}

// splitPartAdaptive intelligently splits a part if it's larger than optimal size
func splitPartAdaptive(p common.Part, optimalSize uint64) []common.Part {
	if p.Size <= optimalSize {
		return []common.Part{p}
	}

	var result []common.Part
	numSplits := (p.Size + optimalSize - 1) / optimalSize
	partSize := (p.Size + numSplits - 1) / numSplits

	for offset := uint64(0); offset < p.Size; offset += partSize {
		size := partSize
		if offset+size > p.Size {
			size = p.Size - offset
		}

		result = append(result, common.Part{
			Path:       p.Path,
			FileSize:   p.FileSize,
			Offset:     p.Offset + offset,
			Size:       size,
			ActualSize: size,
		})
	}
	return result
}

// downloadWithGlobalQueue manages a global work queue with adaptive worker scheduling
func (r *Restore) downloadWithGlobalQueue(ctx context.Context, concurrency int, workQueue []*workQueueItem, bytesDownloaded *atomic.Uint64) error {
	if len(workQueue) == 0 {
		return nil
	}

	// Track which files need preallocation/finalization
	filesPreallocated := make(map[string]bool)
	filesFinalized := make(map[string]bool)
	filePreallocLock := &sync.Mutex{}

	ctxLocal, cancelLocal := context.WithCancel(ctx)
	defer cancelLocal()

	var errMu sync.Mutex
	var firstErr error
	setErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
			cancelLocal()
		}
		errMu.Unlock()
	}

	// Semaphore to limit concurrent workers
	concurrencyCh := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for _, item := range workQueue {
		item := item
		if err := acquireRestoreSlot(ctxLocal, concurrencyCh); err != nil {
			setErr(err)
			break
		}

		wg.Go(func() {
			defer releaseRestoreSlot(concurrencyCh)

			// Handle preallocation
			if item.isFirstPart && !r.SkipPreallocation {
				filePreallocLock.Lock()
				if !filesPreallocated[item.part.Path] {
					if err := r.Dst.PreallocateFile(item.part); err != nil {
						filePreallocLock.Unlock()
						setErr(fmt.Errorf("cannot preallocate %s: %w", item.part.Path, err))
						return
					}
					filesPreallocated[item.part.Path] = true
				}
				filePreallocLock.Unlock()
			}

			// Acquire file-level lock to ensure sequential writes
			item.fileWriteLock.Lock()
			err := pipelinedDownload(r.Src, r.Dst, item.part, bytesDownloaded)
			item.fileWriteLock.Unlock()

			if err != nil {
				setErr(err)
				return
			}

			// Handle finalization
			if item.isLastPart {
				filePreallocLock.Lock()
				if !filesFinalized[item.part.Path] {
					if err := r.Dst.FinalizeFile(item.part); err != nil {
						filePreallocLock.Unlock()
						setErr(fmt.Errorf("cannot finalize %s: %w", item.part.Path, err))
						return
					}
					filesFinalized[item.part.Path] = true
				}
				filePreallocLock.Unlock()
			}
		})
	}
	wg.Wait()

	errMu.Lock()
	err := firstErr
	errMu.Unlock()
	return err
}

func acquireRestoreSlot(ctx context.Context, ch chan struct{}) error {
	select {
	case ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseRestoreSlot(ch chan struct{}) {
	<-ch
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
