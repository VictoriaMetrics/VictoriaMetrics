package storage

func (s *Storage) legacyIDBs() (*indexDB, *indexDB) {
	return s.legacyIDBPrev.Load(), s.legacyIDBCurr.Load()
}

func (s *Storage) hasLegacyIDBs() bool {
	prev, curr := s.legacyIDBs()
	return prev != nil || curr != nil
}
