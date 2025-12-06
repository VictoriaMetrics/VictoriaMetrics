package storage

import (
	"sync"
)

// metricNameSearch is used for searching a metricName by a metricID in
// partition and legacy indexDBs. If useSparseCache is false the name is first
// searched in metricNameCache and also stored in that cache when found in one
// of the indexDBs.
//
// Most index search methods invoked only once per API call. For example, one
// request to /api/v1/series results in one invocation of
// Storage.SearchMetricNames() method. However, searching a metricName by
// metricID is done multiple times per API call. For example, data search
// performs the metricName search for each data block (see search.go).
//
// Additionally, each search method must get a snapshot of idbs so that they
// don't get rotated in the middle of the search. This works for methods that
// are invoked only once per API call. But for searching metricName by metricID
// happens many times per API call and the idb snapshot must be taken
// outside the search method.
//
// To address this, the state of the metricNameSearch type holds the search
// context: the idb snapshot along with other options. This not only ensures
// that idbs won't change during the API call but also avoids performance
// degradation that may be caused by getting the idb snapshot every time the
// search method is invoked (due do mutex locks).
type metricNameSearch struct {
	storage        *Storage
	ptws           []*partitionWrapper
	idbs           []*indexDB
	legacyIDBs     *legacyIndexDBs
	useSparseCache bool
}

// search searches the metricName of a metricID.
func (s *metricNameSearch) search(dst []byte, metricID uint64) ([]byte, bool) {
	if !s.useSparseCache {
		n := len(dst)
		dst = s.storage.getMetricNameFromCache(dst, metricID)
		if len(dst) > n {
			return dst, true
		}
	}

	var found bool

	// This will be just one idb most of the time since a typical time range
	// fits within a single month.
	for _, idb := range s.idbs {
		dst, found = idb.searchMetricName(dst, metricID, s.useSparseCache)
		if found {
			if !s.useSparseCache {
				s.storage.putMetricNameToCache(metricID, dst)
			}
			return dst, true
		}
	}

	// Fallback to current legacy indexDB.
	if idb := s.legacyIDBs.getIDBCurr(); idb != nil {
		dst, found = idb.searchMetricName(dst, metricID, s.useSparseCache)
		if found {
			if !s.useSparseCache {
				s.storage.putMetricNameToCache(metricID, dst)
			}
			return dst, true
		}
	}

	// Fallback to previous legacy indexDB.
	if idb := s.legacyIDBs.getIDBPrev(); idb != nil {
		dst, found = idb.searchMetricName(dst, metricID, s.useSparseCache)
		if found {
			if !s.useSparseCache {
				s.storage.putMetricNameToCache(metricID, dst)
			}
			return dst, true
		}
	}

	// Not deleting metricID if no corresponding metricName has been found
	// because it is not known which indexDB metricID belongs to.
	// For cases when this does happen see indexDB.SearchMetricNames() and
	// indexDB.SearchTSIDs().

	return dst, false
}

var mnsPool = &sync.Pool{
	New: func() any {
		return &metricNameSearch{}
	},
}

func getMetricNameSearch(storage *Storage, tr TimeRange, useSparseCache bool) *metricNameSearch {
	s := mnsPool.Get().(*metricNameSearch)
	s.storage = storage
	s.ptws = storage.tb.GetPartitions(tr)
	for _, ptw := range s.ptws {
		s.idbs = append(s.idbs, ptw.pt.idb)
	}
	s.legacyIDBs = storage.getLegacyIndexDBs()
	s.useSparseCache = useSparseCache
	return s
}

func putMetricNameSearch(s *metricNameSearch) {
	s.storage.tb.PutPartitions(s.ptws)
	s.storage.putLegacyIndexDBs(s.legacyIDBs)
	s.storage = nil
	s.ptws = nil
	s.idbs = nil
	s.legacyIDBs = nil
	s.useSparseCache = false
	mnsPool.Put(s)
}
