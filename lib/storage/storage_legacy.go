package storage

import "time"

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
