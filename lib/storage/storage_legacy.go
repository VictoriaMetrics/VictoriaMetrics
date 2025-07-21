package storage

import (
	"path/filepath"
	"slices"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
)

type legacyIndexDBs struct {
	idbPrev *indexDB
	idbCurr *indexDB
}

func (lidb *legacyIndexDBs) incRef() {
	if lidb == nil {
		// No legacy indexDBs, nothing to increment reference count.
		return
	}

	if lidb.idbPrev != nil {
		lidb.idbPrev.incRef()
	}
	if lidb.idbCurr != nil {
		lidb.idbCurr.incRef()
	}
}

func (lidb *legacyIndexDBs) decRef() {
	if lidb == nil {
		// No legacy indexDBs, nothing to decrement reference count.
		return
	}

	if lidb.idbPrev != nil {
		lidb.idbPrev.decRef()
	}
	if lidb.idbCurr != nil {
		lidb.idbCurr.decRef()
	}
}

func (lidb *legacyIndexDBs) appendTo(dst []*indexDB) []*indexDB {
	if lidb == nil {
		// No legacy indexDBs, nothing to append.
		return dst
	}

	if lidb.idbPrev != nil {
		dst = append(dst, lidb.idbPrev)
	}
	if lidb.idbCurr != nil {
		dst = append(dst, lidb.idbCurr)
	}
	return dst
}

func (s *Storage) hasLegacyIndexDBs() bool {
	return s.legacyIndexDBs.Load() != nil
}

func (s *Storage) getLegacyIndexDBs() *legacyIndexDBs {
	legacyIDBs := s.legacyIndexDBs.Load()
	legacyIDBs.incRef()
	return legacyIDBs
}

func (s *Storage) putLegacyIndexDBs(legacyIDBs *legacyIndexDBs) {
	legacyIDBs.decRef()
}

func (s *Storage) legacyCreateSnapshot(snapshotName, srcDir, dstDir string) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	idbSnapshot := filepath.Join(srcDir, indexdbDirname, snapshotsDirname, snapshotName)
	if legacyIDBs.idbPrev != nil {
		prevSnapshot := filepath.Join(idbSnapshot, legacyIDBs.idbPrev.name)
		legacyIDBs.idbPrev.tb.LegacyMustCreateSnapshotAt(prevSnapshot)
	}
	if legacyIDBs.idbCurr != nil {
		currSnapshot := filepath.Join(idbSnapshot, legacyIDBs.idbCurr.name)
		legacyIDBs.idbCurr.tb.LegacyMustCreateSnapshotAt(currSnapshot)
	}
	dstIdbDir := filepath.Join(dstDir, indexdbDirname)
	fs.MustSymlinkRelative(idbSnapshot, dstIdbDir)
}

func (s *Storage) legacyMustRotateIndexDB(currentTime time.Time) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		// No legacy indexDBs, nothing to rotate.
		return
	}

	legacyIDBs.idbPrev.scheduleToDrop()
	legacyIDBs.idbPrev.decRef()

	var rotatedLegacyIDBs *legacyIndexDBs

	if legacyIDBs.idbCurr != nil {
		rotatedLegacyIDBs = &legacyIndexDBs{
			idbPrev: legacyIDBs.idbCurr,
		}
	}
	s.legacyIndexDBs.Store(rotatedLegacyIDBs)

	// Update nextRotationTimestamp
	nextRotationTimestamp := currentTime.Unix() + s.retentionMsecs/1000
	s.legacyNextRotationTimestamp.Store(nextRotationTimestamp)
}

func (s *Storage) legacyDeleteSeries(qt *querytracer.Tracer, tfss []*TagFilters, maxMetrics int) ([]uint64, error) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		// No legacy indexDBs, nothing to delete.
		return nil, nil
	}

	var (
		dmisPrev []uint64
		dmisCurr []uint64
		err      error
	)

	if legacyIDBs.idbPrev != nil {
		qt.Printf("start deleting from previous legacy indexDB")
		dmisPrev, err = legacyIDBs.idbPrev.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return nil, err
		}
		qt.Printf("deleted %d metricIDs from previous legacy indexDB", len(dmisPrev))
	}

	if legacyIDBs.idbCurr != nil {
		qt.Printf("start deleting from current legacy indexDB")
		dmisCurr, err = legacyIDBs.idbCurr.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return nil, err
		}
		qt.Printf("deleted %d metricIDs from current legacy indexDB", len(dmisCurr))
	}

	return slices.Concat(dmisPrev, dmisCurr), nil
}

func (s *Storage) legacyDebugFlush() {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	if legacyIDBs.idbPrev != nil {
		legacyIDBs.idbPrev.tb.DebugFlush()
	}
	if legacyIDBs.idbCurr != nil {
		legacyIDBs.idbCurr.tb.DebugFlush()
	}
}

func (s *Storage) legacyNotifyReadWriteMode() {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	if legacyIDBs.idbPrev != nil {
		legacyIDBs.idbPrev.tb.NotifyReadWriteMode()
	}
	if legacyIDBs.idbCurr != nil {
		legacyIDBs.idbCurr.tb.NotifyReadWriteMode()
	}
}

func (s *Storage) legacyUpdateMetrics(m *Metrics) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	if legacyIDBs.idbPrev != nil {
		legacyIDBs.idbPrev.UpdateMetrics(&m.TableMetrics.IndexDBMetrics)
	}
	if legacyIDBs.idbCurr != nil {
		legacyIDBs.idbCurr.UpdateMetrics(&m.TableMetrics.IndexDBMetrics)
	}
}

func (s *Storage) legacyMustCloseIndexDBs() {
	legacyIDBs := s.legacyIndexDBs.Load()
	if legacyIDBs == nil {
		return
	}

	if legacyIDBs.idbPrev != nil {
		legacyIDBs.idbPrev.MustClose()
	}
	if legacyIDBs.idbCurr != nil {
		legacyIDBs.idbCurr.MustClose()
	}
}
