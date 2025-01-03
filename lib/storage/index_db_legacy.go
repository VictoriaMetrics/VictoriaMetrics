package storage

import (
	"path/filepath"
	"strconv"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// mustOpenLegacyIndexDB opens legacy index db from the given path.
// Legacy IndexDB is previous implementation of inverted index, it is monolith,
// (not broken down into partitions) and contains records for the entire
// retention period.
//
// Legacy IndexDB operates in read-only mode to enable retrieval of index
// records that have been written before the Partition IndexDB was implemented.
// And once the retention period is over, the Legacy IndexDB will be discarded.
// As retention periods can last for years, Legacy IndexDB code will remain here
// for very long time.
//
// The last segment of the path should contain unique hex value which
// will be then used as indexDB.generation
func mustOpenLegacyIndexDB(path string, s *Storage, isReadOnly *atomic.Bool) *indexDB {
	name := filepath.Base(path)
	gen, err := strconv.ParseUint(name, 16, 64)
	if err != nil {
		logger.Panicf("FATAL: cannot parse indexdb path %q: %s", path, err)
	}

	return mustOpenIndexDB(gen, name, path, s, isReadOnly)
}
