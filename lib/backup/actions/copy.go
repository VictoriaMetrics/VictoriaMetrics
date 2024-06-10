package actions

import (
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// RemoteBackupCopy copies backup from Src to Dst.
type RemoteBackupCopy struct {
	// Concurrency is the number of concurrent workers during the backup.
	Concurrency int

	// Src is the copy source
	Src common.RemoteFS

	// Dst is the copy destination.
	//
	// If dst contains the previous backup data, then dst is updated incrementally,
	// i.e. only the changed data is copied.
	//
	// If dst points to empty dir, then full copy is made.
	Dst common.RemoteFS
}

// Run runs copy with the provided settings.
func (b *RemoteBackupCopy) Run() error {
	concurrency := b.Concurrency
	src := b.Src
	dst := b.Dst

	if err := dst.DeleteFile(backupnames.BackupCompleteFilename); err != nil {
		return fmt.Errorf("cannot delete `backup complete` file at %s: %w", dst, err)
	}
	if err := runCopy(src, dst, concurrency); err != nil {
		return err
	}
	if err := copyMetadata(src, dst); err != nil {
		return fmt.Errorf("cannot store backup metadata: %w", err)
	}
	if err := dst.CreateFile(backupnames.BackupCompleteFilename, nil); err != nil {
		return fmt.Errorf("cannot create `backup complete` file at %s: %w", dst, err)
	}

	return nil
}

func copyMetadata(src common.RemoteFS, dst common.RemoteFS) error {
	metadata, err := src.ReadFile(backupnames.BackupMetadataFilename)
	if err != nil {
		return fmt.Errorf("cannot read metadata from %s: %w", src, err)
	}
	if err := dst.CreateFile(backupnames.BackupMetadataFilename, metadata); err != nil {
		return fmt.Errorf("cannot create metadata at %s: %w", dst, err)
	}
	return nil
}

func runCopy(src common.OriginFS, dst common.RemoteFS, concurrency int) error {
	startTime := time.Now()

	logger.Infof("starting remote backup copy from %s to %s", src, dst)

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

	backupSize := getPartsSize(srcParts)
	partsToDelete := common.PartsDifference(dstParts, srcParts)
	deleteSize := getPartsSize(partsToDelete)
	if err := deleteDstParts(dst, partsToDelete, concurrency); err != nil {
		return fmt.Errorf("cannot delete unneeded parts at dst: %w", err)
	}

	partsToCopy := common.PartsDifference(srcParts, dstParts)
	copySize := getPartsSize(partsToCopy)
	if err := copySrcParts(src, dst, partsToCopy, concurrency); err != nil {
		return fmt.Errorf("cannot server-side copy parts from src to dst: %w", err)
	}

	logger.Infof("remote backup copy from %s to %s is complete; backed up %d bytes in %.3f seconds; server-side deleted %d bytes; server-side copied %d bytes",
		src, dst, backupSize, time.Since(startTime).Seconds(), deleteSize, copySize)

	return nil
}
