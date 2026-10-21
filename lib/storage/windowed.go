package storage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	snapshotMergeWaitTimeout  = 2 * time.Minute
	snapshotMergeWaitInterval = 50 * time.Millisecond
)

var errPartsInMerge = errors.New("parts are already in merge")

// ApplyTimeRangeToSnapshot drops parts whose [minTimestamp, maxTimestamp] does
// not overlap [startMsec, endMsec], then rewrites remaining parts so dest data
// contains samples in [startMsec, endMsec). The forceMerge-only window
// [0, math.MaxInt64] skips that drop, so parts entirely before the Unix epoch
// stay and are only compacted. ctx cancel (SIGTERM) stops wait and force-merge loops.
//
// The caller must open s on a snapshot copy, not on the live -storageDataPath,
// with OpenOptions.DisableBackgroundMerges set so ordinary part mergers do not
// race this rewrite. indexdb and metadata are kept wholesale so restored data
// remains resolvable.
func (s *Storage) ApplyTimeRangeToSnapshot(ctx context.Context, startMsec, endMsec int64, forceMerge bool) error {
	if startMsec >= endMsec {
		return fmt.Errorf("-start (%d) must be less than -end (%d) for a half-open window [start, end)", startMsec, endMsec)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tr := TimeRange{MinTimestamp: startMsec, MaxTimestamp: endMsec}
	if err := s.waitUntilNoPartsInMerge(ctx); err != nil {
		return err
	}
	// [0, MaxInt64] is the forceMerge-only window: keep every part, including
	// those entirely before the Unix epoch, then compact. A real window still
	// drops parts that do not overlap [start, end).
	needTrim := startMsec != 0 || endMsec != math.MaxInt64
	if needTrim {
		kept, dropped, err := s.dropPartsOutsideTimeRange(ctx, tr)
		if err != nil {
			return err
		}
		logger.Infof("windowed backup: dropped %d parts outside [%d, %d); kept %d overlapping parts", dropped, startMsec, endMsec, kept)
		if kept == 0 {
			return fmt.Errorf("no parts overlap the requested window [%d, %d); refusing empty backup", startMsec, endMsec)
		}
	}
	if !needTrim && !forceMerge {
		return nil
	}
	if needTrim {
		s.mergeTrimMinTimestamp.Store(startMsec)
		s.mergeTrimMaxTimestamp.Store(endMsec)
		s.mergeTrimEnabled.Store(true)
		defer func() {
			s.mergeTrimEnabled.Store(false)
			s.mergeTrimMinTimestamp.Store(0)
			s.mergeTrimMaxTimestamp.Store(0)
		}()
		logger.Infof("windowed backup: rewriting overlapping parts to samples in [%d, %d)", startMsec, endMsec)
	}
	if err := s.forceMergePartitionsStrict(ctx, ""); err != nil {
		return err
	}
	if needTrim && s.snapshotDataRows() == 0 {
		return fmt.Errorf("no samples remain in [%d, %d) after trim; refusing empty backup", startMsec, endMsec)
	}
	return nil
}

func (s *Storage) snapshotDataRows() uint64 {
	ptws := s.tb.GetAllPartitions(nil)
	defer s.tb.PutPartitions(ptws)
	var n uint64
	for _, ptw := range ptws {
		pws := ptw.pt.GetParts(nil, true)
		for _, pw := range pws {
			n += pw.p.ph.RowsCount
		}
		ptw.pt.PutParts(pws)
	}
	return n
}

func (s *Storage) dropPartsOutsideTimeRange(ctx context.Context, tr TimeRange) (kept, dropped int, err error) {
	ptws := s.tb.GetAllPartitions(nil)
	defer s.tb.PutPartitions(ptws)
	for _, ptw := range ptws {
		k, d, dropErr := ptw.pt.dropPartsOutsideTimeRange(ctx, tr)
		kept += k
		dropped += d
		if dropErr != nil {
			return kept, dropped, dropErr
		}
	}
	return kept, dropped, nil
}

func (pt *partition) dropPartsOutsideTimeRange(ctx context.Context, tr TimeRange) (kept, dropped int, err error) {
	deadline := time.Now().Add(snapshotMergeWaitTimeout)
	logged := false
	for {
		kept, dropped, err = pt.tryDropPartsOutsideTimeRange(tr)
		if err == nil {
			return kept, dropped, nil
		}
		if !errors.Is(err, errPartsInMerge) {
			return 0, 0, err
		}
		if !logged {
			logger.Infof("windowed backup: waiting for in-flight merge in partition %q before dropping parts outside the window", pt.name)
			logged = true
		}
		if time.Now().After(deadline) {
			return 0, 0, fmt.Errorf("timeout waiting for in-flight merge in partition %q before dropping parts outside the backup window", pt.name)
		}
		if err := sleepOrCancel(ctx, snapshotMergeWaitInterval); err != nil {
			return 0, 0, err
		}
	}
}

func (s *Storage) waitUntilNoPartsInMerge(ctx context.Context) error {
	ptws := s.tb.GetAllPartitions(nil)
	defer s.tb.PutPartitions(ptws)
	for _, ptw := range ptws {
		if err := ptw.pt.waitUntilNoPartsInMerge(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (pt *partition) waitUntilNoPartsInMerge(ctx context.Context) error {
	deadline := time.Now().Add(snapshotMergeWaitTimeout)
	logged := false
	for {
		pt.partsLock.Lock()
		busy := hasActiveMerges(pt.inmemoryParts) || hasActiveMerges(pt.smallParts) || hasActiveMerges(pt.bigParts)
		pt.partsLock.Unlock()
		if !busy {
			return nil
		}
		if !logged {
			logger.Infof("windowed backup: waiting for in-flight merge to finish in partition %q", pt.name)
			logged = true
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for in-flight merge in partition %q", pt.name)
		}
		if err := sleepOrCancel(ctx, snapshotMergeWaitInterval); err != nil {
			return err
		}
	}
}

func sleepOrCancel(ctx context.Context, d time.Duration) error {
	if ctx == nil {
		time.Sleep(d)
		return nil
	}
	done := ctx.Done()
	if done == nil {
		time.Sleep(d)
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-done:
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func sleepOrStop(stopCh <-chan struct{}, d time.Duration) error {
	if stopCh == nil {
		time.Sleep(d)
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stopCh:
		return errForciblyStopped
	case <-t.C:
		return nil
	}
}

func (pt *partition) hasAnyParts() bool {
	pt.partsLock.Lock()
	n := len(pt.inmemoryParts) + len(pt.smallParts) + len(pt.bigParts)
	pt.partsLock.Unlock()
	return n > 0
}

func (pt *partition) tryDropPartsOutsideTimeRange(tr TimeRange) (kept, dropped int, err error) {
	pt.partsLock.Lock()
	var toDrop []*partWrapper
	collect := func(pws []*partWrapper) error {
		for _, pw := range pws {
			if pw.isInMerge {
				return errPartsInMerge
			}
			// Window is half-open [tr.Min, tr.Max). A part that only touches
			// MinTimestamp == end does not overlap.
			partOverlapsWindow := pw.p.ph.MinTimestamp < tr.MaxTimestamp && pw.p.ph.MaxTimestamp >= tr.MinTimestamp
			if partOverlapsWindow {
				kept++
				continue
			}
			pw.isInMerge = true
			toDrop = append(toDrop, pw)
		}
		return nil
	}
	if err = collect(pt.inmemoryParts); err != nil {
		pt.releasePartsToMergeLocked(toDrop)
		pt.partsLock.Unlock()
		return 0, 0, err
	}
	if err = collect(pt.smallParts); err != nil {
		pt.releasePartsToMergeLocked(toDrop)
		pt.partsLock.Unlock()
		return 0, 0, err
	}
	if err = collect(pt.bigParts); err != nil {
		pt.releasePartsToMergeLocked(toDrop)
		pt.partsLock.Unlock()
		return 0, 0, err
	}
	pt.partsLock.Unlock()

	if len(toDrop) == 0 {
		return kept, 0, nil
	}
	pt.swapSrcWithDstParts(toDrop, nil, partSmall)
	return kept, len(toDrop), nil
}

func (pt *partition) releasePartsToMergeLocked(pws []*partWrapper) {
	for _, pw := range pws {
		pw.isInMerge = false
	}
}

func (s *Storage) forceMergePartitionsStrict(ctx context.Context, partitionNamePrefix string) error {
	ptws := s.tb.GetAllPartitions(nil)
	defer s.tb.PutPartitions(ptws)

	s.tb.forceMergeWG.Add(1)
	defer s.tb.forceMergeWG.Done()

	stopCh := ctx.Done()
	if stopCh == nil {
		stopCh = s.tb.stopCh
	}

	for _, ptw := range ptws {
		if !strings.HasPrefix(ptw.pt.name, partitionNamePrefix) {
			continue
		}
		logger.Infof("starting forced merge for partition %q", ptw.pt.name)
		if err := ptw.pt.waitUntilNoPartsInMerge(ctx); err != nil {
			return err
		}
		if err := ptw.pt.forceMergeAllParts(stopCh, true); err != nil {
			return fmt.Errorf("cannot complete forced merge for partition %q: %w", ptw.pt.name, err)
		}
	}
	return nil
}

// RemoveStorageRuntimeFiles removes flock, cache, and nested snapshots
// directories created by MustOpenStorage so a snapshot copy can be uploaded
// without those runtime artifacts.
//
// path must be a snapshot copy ({storageDataPath}/snapshots/{name}), never the
// live -storageDataPath. Only the copy's own snapshots subdirs are removed:
// path/snapshots, path/data/small/snapshots, and path/data/big/snapshots.
func RemoveStorageRuntimeFiles(path string) {
	cachePath := filepath.Join(path, cacheDirname)
	if fs.IsPathExist(cachePath) {
		fs.MustRemoveDirContents(cachePath)
		if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("cannot remove cache directory %q after windowed backup: %s", cachePath, err)
		}
	}
	flockPath := filepath.Join(path, fs.FlockFilename)
	if err := os.Remove(flockPath); err != nil && !os.IsNotExist(err) {
		logger.Warnf("cannot remove flock file %q after windowed backup: %s", flockPath, err)
	}
	nestedSnapshots := []string{
		filepath.Join(path, snapshotsDirname),
		filepath.Join(path, dataDirname, smallDirname, snapshotsDirname),
		filepath.Join(path, dataDirname, bigDirname, snapshotsDirname),
	}
	for _, snapshotsPath := range nestedSnapshots {
		if !fs.IsPathExist(snapshotsPath) {
			continue
		}
		fs.MustRemoveDirContents(snapshotsPath)
		if err := os.Remove(snapshotsPath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("cannot remove snapshots directory %q after windowed backup: %s", snapshotsPath, err)
		}
	}
}
