package storage

import (
	"math"
	"path/filepath"
	"strconv"
	"sync/atomic"

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
