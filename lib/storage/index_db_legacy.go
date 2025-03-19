package storage

import (
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
	gen, err := strconv.ParseUint(name, 16, 64)
	if err != nil {
		logger.Panicf("FATAL: cannot parse indexdb path %q: %s", path, err)
	}

	var alwaysReadOnly atomic.Bool
	alwaysReadOnly.Store(true)
	return mustOpenIndexDB(TimeRange{}, gen, name, path, s, &alwaysReadOnly)
}
