package storage

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// mustOpenLegacyIndexDBReadOnly opens legacy index db from the given path in
// read-only mode.
//
// The last segment of the path should contain unique hex value which
// will be then used as indexDB.generation
func mustOpenLegacyIndexDBReadOnly(path string, s *Storage) *indexDB {
	name := filepath.Base(path)
	id, err := strconv.ParseUint(name, 16, 64)
	if err != nil {
		logger.Panicf("FATAL: cannot parse indexdb path %q: %s", path, err)
	}

	tr := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: math.MaxInt64,
	}
	var alwaysReadOnly atomic.Bool
	alwaysReadOnly.Store(true)
	return mustOpenIndexDB(id, tr, name, path, s, &alwaysReadOnly)
}

// appendTo scans through all index db items and appends them to the given dstIdbs:
//   - global index items go to all dstIdbs.
//   - per-day - only to dstIdbs covering this particular day.
func (db *indexDB) appendTo(dstIdbs []*indexDB) error {
	// FIXME: measure/optimize performance if this function is used for real indexDBs
	is := db.getIndexSearch(noDeadline)
	defer db.putIndexSearch(is)

	ts := is.ts
	ts.Seek(nil)
	for ts.NextItem() {
		item := ts.Item
		switch item[0] {
		case nsPrefixDateToMetricID, nsPrefixDateTagToMetricIDs, nsPrefixDateMetricNameToTSID:
			date := encoding.UnmarshalUint64(item[1:])
			for _, dstIdb := range dstIdbs {
				minDate, maxDate := dstIdb.tr.DateRange()
				if minDate <= date && maxDate >= date {
					dstIdb.tb.AddItems([][]byte{item})
				}
			}

		default:
			for _, dstIdb := range dstIdbs {
				dstIdb.tb.AddItems([][]byte{item})
			}
		}
	}

	if err := ts.Error(); err != nil {
		return fmt.Errorf("could not append indexDB %q: %w", db.name, err)
	}

	// FIXME: ugly, but we need to reload all added deleted metric IDs
	for _, dstIdb := range dstIdbs {
		dstIdb.tb.DebugFlush()
		dstIdb.loadDeletedMetricIDs()
		dstIdb.invalidateTagFiltersCache()
	}

	return nil
}
