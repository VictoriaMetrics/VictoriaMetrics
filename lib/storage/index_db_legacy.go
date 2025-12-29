package storage

import (
	"math"
	"path/filepath"
	"strconv"
	"sync/atomic"

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
