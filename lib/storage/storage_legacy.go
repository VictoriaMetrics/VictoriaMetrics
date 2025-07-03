package storage

import (
	"slices"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
)

func (s *Storage) hasLegacyIndexDBs() bool {
	return s.legacyIDBPrev.Load() != nil || s.legacyIDBCurr.Load() != nil
}

func (s *Storage) getLegacyIndexDBs() (prev, curr *indexDB) {
	s.legacyIDBLock.Lock()
	defer s.legacyIDBLock.Unlock()
	prev, curr = s.legacyIDBPrev.Load(), s.legacyIDBCurr.Load()
	if prev != nil {
		prev.incRef()
	}
	if curr != nil {
		curr.incRef()
	}
	return prev, curr
}

func (s *Storage) putLegacyIndexDBs(prev, curr *indexDB) {
	if prev != nil {
		prev.decRef()
	}
	if curr != nil {
		curr.decRef()
	}
}

func (s *Storage) legacyMustRotateIndexDB(currentTime time.Time) {
	idbPrev, idbCurr := s.legacyIDBPrev.Load(), s.legacyIDBCurr.Load()
	if idbPrev == nil {
		return
	}

	s.legacyIDBLock.Lock()
	defer s.legacyIDBLock.Unlock()

	s.legacyIDBPrev.Store(idbCurr)
	s.legacyIDBCurr.Store(nil)
	idbPrev.scheduleToDrop()
	idbPrev.decRef()

	// Update nextRotationTimestamp
	nextRotationTimestamp := currentTime.Unix() + s.retentionMsecs/1000
	s.legacyNextRotationTimestamp.Store(nextRotationTimestamp)
}

func (s *Storage) legacyDeleteSeries(qt *querytracer.Tracer, tfss []*TagFilters, maxMetrics int) ([]uint64, error) {
	idbPrev, idbCurr := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(idbPrev, idbCurr)

	var (
		dmisPrev []uint64
		dmisCurr []uint64
		err      error
	)

	if idbPrev != nil {
		qt.Printf("start deleting from previous legacy indexDB")
		dmisPrev, err = idbPrev.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return nil, err
		}
		qt.Printf("deleted %d metricIDs from previous legacy indexDB", len(dmisPrev))
	}

	if idbCurr != nil {
		qt.Printf("start deleting from current legacy indexDB")
		dmisCurr, err = idbCurr.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return nil, err
		}
		qt.Printf("deleted %d metricIDs from current legacy indexDB", len(dmisCurr))
	}

	return slices.Concat(dmisPrev, dmisCurr), nil
}

func (s *Storage) legacyDebugFlush() {
	legacyIDBPrev, legacyIDBCurr := s.getLegacyIndexDBs()
	if legacyIDBPrev != nil {
		legacyIDBPrev.tb.DebugFlush()
	}
	if legacyIDBCurr != nil {
		legacyIDBCurr.tb.DebugFlush()
	}
	s.putLegacyIndexDBs(legacyIDBPrev, legacyIDBCurr)
}
