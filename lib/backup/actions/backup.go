package actions

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fscommon"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsnil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Backup performs backup according to the provided settings.
//
// Note that the backup works only for VictoriaMetrics snapshots
// made via `/snapshot/create`. It works improperly on mutable files.
type Backup struct {
	// Concurrency is the number of concurrent workers during the backup.
	// Concurrency=1 by default.
	Concurrency int

	// Src is backup source
	Src *fslocal.FS

	// Dst is backup destination.
	//
	// If dst contains the previous backup data, then incremental backup
	// is made, i.e. only the changed data is uploaded.
	//
	// If dst points to empty dir, then full backup is made.
	// Origin can be set to the previous backup in order to reduce backup duration
	// and reduce network bandwidth usage.
	Dst common.RemoteFS

	// Origin is optional origin for speeding up full backup if Dst points
	// to empty dir.
	Origin common.OriginFS
}

// Run runs b with the provided settings.
func (b *Backup) Run() error {
	concurrency := b.Concurrency
	src := b.Src
	dst := b.Dst
	origin := b.Origin

	if origin != nil && origin.String() == dst.String() {
		origin = nil
	}
	if origin == nil {
		origin = &fsnil.FS{}
	}

	if err := dst.DeleteFile(fscommon.BackupCompleteFilename); err != nil {
		return fmt.Errorf("cannot delete `backup complete` file at %s: %w", dst, err)
	}
	if err := runBackup(src, dst, origin, concurrency); err != nil {
		return err
	}
	if err := dst.CreateFile(fscommon.BackupCompleteFilename, []byte("ok")); err != nil {
		return fmt.Errorf("cannot create `backup complete` file at %s: %w", dst, err)
	}
	return nil
}

func runBackup(src *fslocal.FS, dst common.RemoteFS, origin common.OriginFS, concurrency int) error {
	startTime := time.Now()

	logger.Infof("starting backup from %s to %s using origin %s", src, dst, origin)

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
	logger.Infof("obtaining list of parts at %s", origin)
	originParts, err := origin.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list origin parts: %w", err)
	}

	backupSize := getPartsSize(srcParts)

	partsToDelete := common.PartsDifference(dstParts, srcParts)
	deleteSize := getPartsSize(partsToDelete)
	if len(partsToDelete) > 0 {
		logger.Infof("deleting %d parts from %s", len(partsToDelete), dst)
		deletedParts := uint64(0)
		err = runParallel(concurrency, partsToDelete, func(p common.Part) error {
			logger.Infof("deleting %s from %s", &p, dst)
			if err := dst.DeletePart(p); err != nil {
				return fmt.Errorf("cannot delete %s from %s: %w", &p, dst, err)
			}
			atomic.AddUint64(&deletedParts, 1)
			return nil
		}, func(elapsed time.Duration) {
			n := atomic.LoadUint64(&deletedParts)
			logger.Infof("deleted %d out of %d parts from %s in %s", n, len(partsToDelete), dst, elapsed)
		})
		if err != nil {
			return err
		}
		if err := dst.RemoveEmptyDirs(); err != nil {
			return fmt.Errorf("cannot remove empty directories at %s: %w", dst, err)
		}
	}

	partsToCopy := common.PartsDifference(srcParts, dstParts)
	originCopyParts := common.PartsIntersect(originParts, partsToCopy)
	copySize := getPartsSize(originCopyParts)
	if len(originCopyParts) > 0 {
		logger.Infof("server-side copying %d parts from %s to %s", len(originCopyParts), origin, dst)
		copiedParts := uint64(0)
		err = runParallel(concurrency, originCopyParts, func(p common.Part) error {
			logger.Infof("server-side copying %s from %s to %s", &p, origin, dst)
			if err := dst.CopyPart(origin, p); err != nil {
				return fmt.Errorf("cannot copy %s from %s to %s: %w", &p, origin, dst, err)
			}
			atomic.AddUint64(&copiedParts, 1)
			return nil
		}, func(elapsed time.Duration) {
			n := atomic.LoadUint64(&copiedParts)
			logger.Infof("server-side copied %d out of %d parts from %s to %s in %s", n, len(originCopyParts), origin, dst, elapsed)
		})
		if err != nil {
			return err
		}
	}

	srcCopyParts := common.PartsDifference(partsToCopy, originParts)
	uploadSize := getPartsSize(srcCopyParts)
	if len(srcCopyParts) > 0 {
		logger.Infof("uploading %d parts from %s to %s", len(srcCopyParts), src, dst)
		bytesUploaded := uint64(0)
		err = runParallel(concurrency, srcCopyParts, func(p common.Part) error {
			logger.Infof("uploading %s from %s to %s", &p, src, dst)
			rc, err := src.NewReadCloser(p)
			if err != nil {
				return fmt.Errorf("cannot create reader for %s from %s: %w", &p, src, err)
			}
			sr := &statReader{
				r:         rc,
				bytesRead: &bytesUploaded,
			}
			if err := dst.UploadPart(p, sr); err != nil {
				return fmt.Errorf("cannot upload %s to %s: %w", &p, dst, err)
			}
			if err = rc.Close(); err != nil {
				return fmt.Errorf("cannot close reader for %s from %s: %w", &p, src, err)
			}
			return nil
		}, func(elapsed time.Duration) {
			n := atomic.LoadUint64(&bytesUploaded)
			logger.Infof("uploaded %d out of %d bytes from %s to %s in %s", n, uploadSize, src, dst, elapsed)
		})
		if err != nil {
			return err
		}
	}

	logger.Infof("backed up %d bytes in %.3f seconds; deleted %d bytes; server-side copied %d bytes; uploaded %d bytes",
		backupSize, time.Since(startTime).Seconds(), deleteSize, copySize, uploadSize)

	return nil
}

type statReader struct {
	r         io.Reader
	bytesRead *uint64
}

func (sr *statReader) Read(p []byte) (int, error) {
	n, err := sr.r.Read(p)
	atomic.AddUint64(sr.bytesRead, uint64(n))
	return n, err
}
