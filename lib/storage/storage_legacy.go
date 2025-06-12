package storage

import (
	"math"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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

// migrateLegacyIndexes will create hardlinks to the legacy IndexDB parts and add them to all current partitioned IndexDBs
func (s *Storage) migrateLegacyIndexes() {
	idbs := s.tb.GetIndexDBs(TimeRange{MinTimestamp: 0, MaxTimestamp: math.MaxInt64})
	defer s.tb.PutIndexDBs(idbs)

	idbsNames := make([]string, 0, len(idbs))
	for _, idb := range idbs {
		idbsNames = append(idbsNames, idb.name)
	}

	for {
		idbPrev := s.legacyIDBPrev.Load()
		if idbPrev == nil {
			break
		}

		logger.Infof("Migrating legacy indexDB %s to partitioned indexes %v", idbPrev.name, idbsNames)
		idbPrev.mustAppendSnapshotTo(idbs)
		s.legacyMustRotateIndexDB(time.Now())
	}

	s.legacyDeletedMetricIDs = nil
}
