package storage

import (
	"path/filepath"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

type legacyIndexDBs struct {
	idbPrev *legacyIndexDB
	idbCurr *legacyIndexDB
}

func (dbs *legacyIndexDBs) incRef() {
	if dbs == nil {
		// No legacy indexDBs, nothing to increment reference count.
		return
	}

	if dbs.idbPrev != nil {
		dbs.idbPrev.incRef()
	}
	if dbs.idbCurr != nil {
		dbs.idbCurr.incRef()
	}
}

func (dbs *legacyIndexDBs) decRef() {
	if dbs == nil {
		// No legacy indexDBs, nothing to decrement reference count.
		return
	}

	if dbs.idbPrev != nil {
		dbs.idbPrev.decRef()
	}
	if dbs.idbCurr != nil {
		dbs.idbCurr.decRef()
	}
}

func (dbs *legacyIndexDBs) appendTo(dst []*indexDB) []*indexDB {
	if dbs == nil {
		// No legacy indexDBs, nothing to append.
		return dst
	}

	if dbs.idbPrev != nil {
		dst = append(dst, dbs.idbPrev.idb)
	}
	if dbs.idbCurr != nil {
		dst = append(dst, dbs.idbCurr.idb)
	}
	return dst
}

func (dbs *legacyIndexDBs) getIDBPrev() *indexDB {
	if dbs == nil || dbs.idbPrev == nil {
		return nil
	}
	return dbs.idbPrev.idb
}

func (dbs *legacyIndexDBs) getIDBCurr() *indexDB {
	if dbs == nil || dbs.idbCurr == nil {
		return nil
	}
	return dbs.idbCurr.idb
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
	if idbPrev := legacyIDBs.getIDBPrev(); idbPrev != nil {
		prevSnapshot := filepath.Join(idbSnapshot, idbPrev.name)
		idbPrev.tb.LegacyMustCreateSnapshotAt(prevSnapshot)
	}
	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		currSnapshot := filepath.Join(idbSnapshot, idbCurr.name)
		idbCurr.tb.LegacyMustCreateSnapshotAt(currSnapshot)
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

func (s *Storage) legacyDeleteSeries(qt *querytracer.Tracer, tfss []*TagFilters, maxMetrics int) (*uint64set.Set, error) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		// No legacy indexDBs, nothing to delete.
		return nil, nil
	}

	all := &uint64set.Set{}

	if idbPrev := legacyIDBs.getIDBPrev(); idbPrev != nil {
		qt.Printf("start deleting from previous legacy indexDB")
		dmis, err := idbPrev.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return nil, err
		}
		qt.Printf("deleted %d metricIDs from previous legacy indexDB", dmis.Len())
		all.UnionMayOwn(dmis)
	}

	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		qt.Printf("start deleting from current legacy indexDB")
		dmis, err := idbCurr.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return nil, err
		}
		qt.Printf("deleted %d metricIDs from current legacy indexDB", dmis.Len())
		all.UnionMayOwn(dmis)
	}

	return all, nil
}

func (s *Storage) legacyDebugFlush() {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	if idbPrev := legacyIDBs.getIDBPrev(); idbPrev != nil {
		idbPrev.tb.DebugFlush()
	}
	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		idbCurr.tb.DebugFlush()
	}
}

func (s *Storage) legacyNotifyReadWriteMode() {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	if idbPrev := legacyIDBs.getIDBPrev(); idbPrev != nil {
		idbPrev.tb.NotifyReadWriteMode()
	}
	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		idbCurr.tb.NotifyReadWriteMode()
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
