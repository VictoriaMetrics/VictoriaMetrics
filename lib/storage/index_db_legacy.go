package storage

import (
	"bytes"
	"math"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// legacyIndexDB is a wrapper around indexDB that provides reference counting
type legacyIndexDB struct {
	// The number of references to legacyIndexDB struct.
	refCount atomic.Int32

	// if the mustDrop is set to true, then the legacyIndexDB must be dropped after refCount reaches zero.
	mustDrop atomic.Bool

	idb *indexDB
}

func (db *legacyIndexDB) incRef() {
	db.refCount.Add(1)
}

func (db *legacyIndexDB) decRef() {
	n := db.refCount.Add(-1)
	if n < 0 {
		logger.Panicf("BUG: %q negative refCount: %d", db.idb.name, n)
	}
	if n > 0 {
		return
	}

	tbPath := db.idb.tb.Path()
	db.idb.MustClose()

	if !db.mustDrop.Load() {
		return
	}

	logger.Infof("dropping indexDB %q", tbPath)
	fs.MustRemoveDir(tbPath)
	logger.Infof("indexDB %q has been dropped", tbPath)
}

func (db *legacyIndexDB) scheduleToDrop() {
	db.mustDrop.Store(true)
}

func (db *legacyIndexDB) MustClose() {
	rc := db.refCount.Load()
	if rc != 1 {
		logger.Fatalf("BUG: %q unexpected legacy indexDB refCount: %d", db.idb.name, rc)
	}
	db.decRef()
}

func (db *legacyIndexDB) UpdateMetrics(m *IndexDBMetrics) {
	db.idb.UpdateMetrics(m)
	m.IndexDBRefCount += uint64(db.refCount.Load())
}

// mustOpenLegacyIndexDB opens legacy index db from the given path.
//
// The last segment of the path should contain unique hex value which
// will be then used as indexDB.generation
func mustOpenLegacyIndexDB(path string, s *Storage) *legacyIndexDB {
	name := filepath.Base(path)
	id, err := strconv.ParseUint(name, 16, 64)
	if err != nil {
		logger.Panicf("FATAL: cannot parse indexdb path %q: %s", path, err)
	}

	tr := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: math.MaxInt64,
	}
	idb := mustOpenIndexDB(id, tr, name, path, s, &s.isReadOnly, true)
	legacyIDB := &legacyIndexDB{idb: idb}
	legacyIDB.incRef()
	return legacyIDB
}

func (db *indexDB) legacyContainsDate(date uint64) bool {
	var tr TimeRange
	if date == globalIndexDate {
		tr = globalIndexTimeRange
	} else {
		tr.MinTimestamp = int64(date) * msecPerDay
		tr.MaxTimestamp = int64(date+1)*msecPerDay - 1
	}
	return db.legacyContainsTimeRange(tr)
}

func (db *indexDB) legacyContainsTimeRange(tr TimeRange) bool {
	if tr == globalIndexTimeRange {
		return true
	}

	if !db.noRegisterNewSeries.Load() {
		// indexDB could register new time series - it is not safe to cache minMissingTimestamp
		return true
	}

	// vmsingle does not have tenants and therefore has just one key.
	// While vmstorage can potentially have many tenants and the actual
	// accountID and projectID will be set from the request.
	key := TenantToken{
		AccountID: 0,
		ProjectID: 0,
	}
	db.legacyMinMissingTimestampByKeyLock.Lock()
	minMissingTimestamp, ok := db.legacyMinMissingTimestampByKey[key]
	db.legacyMinMissingTimestampByKeyLock.Unlock()

	if ok && tr.MinTimestamp >= minMissingTimestamp {
		// Fast path.
		return false
	}

	// Slow path.
	is := db.getIndexSearch(noDeadline)
	defer db.putIndexSearch(is)
	if is.legacyContainsTimeRange(tr) {
		return true
	}

	db.legacyMinMissingTimestampByKeyLock.Lock()
	minMissingTimestamp, ok = db.legacyMinMissingTimestampByKey[key]
	if !ok || tr.MinTimestamp < minMissingTimestamp {
		db.legacyMinMissingTimestampByKey[key] = tr.MinTimestamp
	}
	db.legacyMinMissingTimestampByKeyLock.Unlock()

	return false
}

func (is *indexSearch) legacyContainsTimeRange(tr TimeRange) bool {
	// Verify whether the tr.MinTimestamp is included into `ts` or is smaller than the minimum date stored in `ts`.
	// Do not check whether tr.MaxTimestamp is included into `ts` or is bigger than the max date stored in `ts` for performance reasons.
	// This means that this func can return true if `tr` is located below the min date stored in `ts`.
	// This is OK, since this case isn't encountered too much in practice.
	// The main practical case allows skipping searching in prev indexdb (`ts`) when `tr`
	// is located above the max date stored there.
	minDate := uint64(tr.MinTimestamp) / msecPerDay

	kb := &is.kb
	kb.B = is.marshalCommonPrefix(kb.B[:0], nsPrefixDateToMetricID)
	prefix := kb.B
	kb.B = encoding.MarshalUint64(kb.B, minDate)

	ts := &is.ts
	ts.Seek(kb.B)
	if !ts.NextItem() {
		if err := ts.Error(); err != nil {
			logger.Panicf("FATAL: error when searching for minDate=%d, prefix %q: %s", minDate, kb.B, err)
		}
		return false
	}
	if !bytes.HasPrefix(ts.Item, prefix) {
		// minDate exceeds max date from ts.
		return false
	}
	return true
}
