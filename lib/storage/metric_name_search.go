package storage

import (
	"sync"
)

// metricNameSearch is used for for searching a metricName by a metricID in curr
// and prev indexDBs. If useSparseCache is false the name is first searched in
// metricNameCache and also stored in that cache when found in one of the
// indexDBs.
//
// Most index search methods invoked only once per API call. For example, one
// request to /api/v1/series results in one invocation of
// Storage.SearchMetricNames() method. However, searching is metricName by
// metricID is done multiple times per API call. For example, data search
// performs the the metricName search for each data block (see search.go).
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
	idbPrev        *indexDB
	idbCurr        *indexDB
	useSparseCache bool
}

// search searches the metricName of a metricID.
func (s *metricNameSearch) search(dst []byte, metricID uint64) ([]byte, bool) {
	if !s.useSparseCache {
		metricName := s.storage.getMetricNameFromCache(dst, metricID)
		if len(metricName) > len(dst) {
			return metricName, true
		}
	}

	dst, found := s.idbCurr.searchMetricName(dst, metricID, s.useSparseCache)
	if found {
		if !s.useSparseCache {
			s.storage.putMetricNameToCache(metricID, dst)
		}
		return dst, true
	}

	// Fallback to previous indexDB.
	dst, found = s.idbPrev.searchMetricName(dst, metricID, s.useSparseCache)
	if found {
		if !s.useSparseCache {
			s.storage.putMetricNameToCache(metricID, dst)
		}
		return dst, true
	}

	// Not deleting metricID if no corresponding metricName has been found
	// because it is not known which indexDB metricID belongs to.
	// For cases when this does happen see indexDB.SearchMetricNames() and
	// indexDB.getTSIDsFromMetricIDs()).

	return dst, false
}

var mnsPool = &sync.Pool{
	New: func() any {
		return &metricNameSearch{}
	},
}

func getMetricNameSearch(storage *Storage, useSparseCache bool) *metricNameSearch {
	s := mnsPool.Get().(*metricNameSearch)
	s.storage = storage
	s.idbPrev, s.idbCurr = storage.getPrevAndCurrIndexDBs()
	s.useSparseCache = useSparseCache
	return s
}

func putMetricNameSearch(s *metricNameSearch) {
	s.storage.putPrevAndCurrIndexDBs(s.idbPrev, s.idbCurr)
	s.storage = nil
	s.idbPrev = nil
	s.idbCurr = nil
	s.useSparseCache = false
	mnsPool.Put(s)
}
