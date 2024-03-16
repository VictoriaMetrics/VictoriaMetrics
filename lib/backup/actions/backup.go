package actions

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsnil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot/snapshotutil"
	"github.com/VictoriaMetrics/metrics"
)

// Backup performs backup according to the provided settings.
//
// Note that the backup works only for VictoriaMetrics snapshots
// made via `/snapshot/create`. It works improperly on mutable files.
type Backup struct {
	// Concurrency is the number of concurrent workers during the backup.
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

// BackupMetadata contains metadata about the backup.
// Note that CreatedAt and CompletedAt are in RFC3339 format.
type BackupMetadata struct {
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at"`
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

	if err := dst.DeleteFile(backupnames.BackupCompleteFilename); err != nil {
		return fmt.Errorf("cannot delete `backup complete` file at %s: %w", dst, err)
	}
	if err := runBackup(src, dst, origin, concurrency); err != nil {
		return err
	}
	if err := storeMetadata(src, dst); err != nil {
		return fmt.Errorf("cannot store backup metadata: %w", err)
	}
	if err := dst.CreateFile(backupnames.BackupCompleteFilename, nil); err != nil {
		return fmt.Errorf("cannot create `backup complete` file at %s: %w", dst, err)
	}

	return nil
}

func storeMetadata(src *fslocal.FS, dst common.RemoteFS) error {
	snapshotName := filepath.Base(src.Dir)
	snapshotTime, err := snapshotutil.Time(snapshotName)
	if err != nil {
		return fmt.Errorf("cannot decode snapshot name %q: %w", snapshotName, err)
	}

	d := BackupMetadata{
		CreatedAt:   snapshotTime.Format(time.RFC3339),
		CompletedAt: time.Now().Format(time.RFC3339),
	}

	metadata, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("cannot marshal metadata: %w", err)
	}

	if err := dst.CreateFile(backupnames.BackupMetadataFilename, metadata); err != nil {
		return fmt.Errorf("cannot create `backup complete` file at %s: %w", dst, err)
	}

	return nil
}

func runBackup(src *fslocal.FS, dst common.RemoteFS, origin common.OriginFS, concurrency int) error {
	startTime := time.Now()

	logger.Infof("starting backup from %s to %s using origin %s", src, dst, origin)

	srcParts, err := src.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list src parts: %w", err)
	}
	logger.Infof("obtained %d parts from src %s", len(srcParts), src)

	dstParts, err := dst.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list dst parts: %w", err)
	}
	logger.Infof("obtained %d parts from dst %s", len(dstParts), dst)

	originParts, err := origin.ListParts()
	if err != nil {
		return fmt.Errorf("cannot list origin parts: %w", err)
	}
	logger.Infof("obtained %d parts from origin %s", len(originParts), origin)

	backupSize := getPartsSize(srcParts)
	partsToDelete := common.PartsDifference(dstParts, srcParts)
	deleteSize := getPartsSize(partsToDelete)
	if err := deleteDstParts(dst, partsToDelete, concurrency); err != nil {
		return fmt.Errorf("cannot delete unneeded parts at dst: %w", err)
	}

	partsToCopy := common.PartsDifference(srcParts, dstParts)
	originPartsToCopy := common.PartsIntersect(originParts, partsToCopy)
	copySize := getPartsSize(originPartsToCopy)
	if err := copySrcParts(origin, dst, originPartsToCopy, concurrency); err != nil {
		return fmt.Errorf("cannot server-side copy origin parts to dst: %w", err)
	}

	srcCopyParts := common.PartsDifference(partsToCopy, originParts)
	uploadSize := getPartsSize(srcCopyParts)
	if len(srcCopyParts) > 0 {
		logger.Infof("uploading %d parts from %s to %s", len(srcCopyParts), src, dst)
		var bytesUploaded atomic.Uint64
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
			n := bytesUploaded.Load()
			prc := 100 * float64(n) / float64(uploadSize)
			logger.Infof("uploaded %d out of %d bytes (%.2f%%) from %s to %s in %s", n, uploadSize, prc, src, dst, elapsed)
		})
		if err != nil {
			return err
		}
	}

	logger.Infof("backup from %s to %s with origin %s is complete; backed up %d bytes in %.3f seconds; server-side deleted %d bytes; "+
		"server-side copied %d bytes; uploaded %d bytes",
		src, dst, origin, backupSize, time.Since(startTime).Seconds(), deleteSize, copySize, uploadSize)

	return nil
}

type statReader struct {
	r         io.Reader
	bytesRead *atomic.Uint64
}

func (sr *statReader) Read(p []byte) (int, error) {
	n, err := sr.r.Read(p)
	sr.bytesRead.Add(uint64(n))
	bytesUploadedTotal.Add(n)
	return n, err
}

var bytesUploadedTotal = metrics.NewCounter(`vm_backups_uploaded_bytes_total`)

func deleteDstParts(dst common.RemoteFS, partsToDelete []common.Part, concurrency int) error {
	if len(partsToDelete) == 0 {
		return nil
	}
	logger.Infof("deleting %d parts from %s", len(partsToDelete), dst)
	var deletedParts atomic.Uint64
	err := runParallel(concurrency, partsToDelete, func(p common.Part) error {
		logger.Infof("deleting %s from %s", &p, dst)
		if err := dst.DeletePart(p); err != nil {
			return fmt.Errorf("cannot delete %s from %s: %w", &p, dst, err)
		}
		deletedParts.Add(1)
		return nil
	}, func(elapsed time.Duration) {
		n := deletedParts.Load()
		logger.Infof("deleted %d out of %d parts from %s in %s", n, len(partsToDelete), dst, elapsed)
	})
	if err != nil {
		return err
	}
	if err := dst.RemoveEmptyDirs(); err != nil {
		return fmt.Errorf("cannot remove empty directories at %s: %w", dst, err)
	}
	return nil
}

func copySrcParts(src common.OriginFS, dst common.RemoteFS, partsToCopy []common.Part, concurrency int) error {
	if len(partsToCopy) == 0 {
		return nil
	}
	logger.Infof("server-side copying %d parts from %s to %s", len(partsToCopy), src, dst)
	var copiedParts atomic.Uint64
	err := runParallel(concurrency, partsToCopy, func(p common.Part) error {
		logger.Infof("server-side copying %s from %s to %s", &p, src, dst)
		if err := dst.CopyPart(src, p); err != nil {
			return fmt.Errorf("cannot copy %s from %s to %s: %w", &p, src, dst, err)
		}
		copiedParts.Add(1)
		return nil
	}, func(elapsed time.Duration) {
		n := copiedParts.Load()
		logger.Infof("server-side copied %d out of %d parts from %s to %s in %s", n, len(partsToCopy), src, dst, elapsed)
	})
	return err
}
