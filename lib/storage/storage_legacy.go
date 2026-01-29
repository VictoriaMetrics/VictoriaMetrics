package storage

import (
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

type legacyIndexDBs struct {
	idbPrev *legacyIndexDB
	idbCurr *legacyIndexDB
}

func (dbs *legacyIndexDBs) incRef() {
	if dbs == nil {
		// No legacy indexDBs, nothing to increment reference count.
		return
	}

	if dbs.idbPrev != nil {
		dbs.idbPrev.incRef()
	}
	if dbs.idbCurr != nil {
		dbs.idbCurr.incRef()
	}
}

func (dbs *legacyIndexDBs) decRef() {
	if dbs == nil {
		// No legacy indexDBs, nothing to decrement reference count.
		return
	}

	if dbs.idbPrev != nil {
		dbs.idbPrev.decRef()
	}
	if dbs.idbCurr != nil {
		dbs.idbCurr.decRef()
	}
}

func (dbs *legacyIndexDBs) appendTo(dst []indexDBWithType) []indexDBWithType {
	if dbs == nil {
		// No legacy indexDBs, nothing to append.
		return dst
	}

	if dbs.idbPrev != nil {
		idbt := indexDBWithType{
			idb: dbs.idbPrev.idb,
			t:   indexDBTypeLegacyPrev,
		}
		dst = append(dst, idbt)
	}
	if dbs.idbCurr != nil {
		idbt := indexDBWithType{
			idb: dbs.idbCurr.idb,
			t:   indexDBTypeLegacyCurr,
		}
		dst = append(dst, idbt)
	}
	return dst
}

func (dbs *legacyIndexDBs) getIDBPrev() *indexDB {
	if dbs == nil || dbs.idbPrev == nil {
		return nil
	}
	return dbs.idbPrev.idb
}

func (dbs *legacyIndexDBs) getIDBCurr() *indexDB {
	if dbs == nil || dbs.idbCurr == nil {
		return nil
	}
	return dbs.idbCurr.idb
}

func (s *Storage) mustOpenLegacyIndexDBTables(path string) *legacyIndexDBs {
	if !fs.IsPathExist(path) {
		return nil
	}

	// Search for the two most recent tables: prev and curr.

	// Placing the regexp inside the func in order to keep legacy code close to
	// each other and because this function is called only once on startup.
	indexDBTableNameRegexp := regexp.MustCompile("^[0-9A-F]{16}$")
	des := fs.MustReadDir(path)
	var tableNames []string
	for _, de := range des {
		if !fs.IsDirOrSymlink(de) {
			// Skip non-directories.
			continue
		}
		tableName := de.Name()
		if !indexDBTableNameRegexp.MatchString(tableName) {
			// Skip invalid directories.
			continue
		}
		tableDirPath := filepath.Join(path, tableName)
		if fs.IsPartiallyRemovedDir(tableDirPath) {
			// Finish the removal of partially deleted directory, which can occur
			// when the directory was removed during unclean shutdown.
			fs.MustRemoveDir(tableDirPath)
			continue
		}
		tableNames = append(tableNames, tableName)
	}
	sort.Slice(tableNames, func(i, j int) bool {
		return tableNames[i] < tableNames[j]
	})

	if len(tableNames) > 3 {
		// Remove all the tables except the last three tables.
		for _, tn := range tableNames[:len(tableNames)-3] {
			pathToRemove := filepath.Join(path, tn)
			logger.Infof("removing obsolete indexdb dir %q...", pathToRemove)
			fs.MustRemoveDir(pathToRemove)
			logger.Infof("removed obsolete indexdb dir %q", pathToRemove)
		}
		fs.MustSyncPath(path)
		tableNames = tableNames[len(tableNames)-3:]
	}
	if len(tableNames) == 3 {
		// Also remove next idb.
		pathToRemove := filepath.Join(path, tableNames[2])
		logger.Infof("removing next indexdb dir %q...", pathToRemove)
		fs.MustRemoveDir(pathToRemove)
		logger.Infof("removed next indexdb dir %q", pathToRemove)
		fs.MustSyncPath(path)
		tableNames = tableNames[:2]
	}

	numIDBs := len(tableNames)
	legacyIDBs := &legacyIndexDBs{}

	if numIDBs == 0 {
		return nil
	}

	if numIDBs > 1 {
		currPath := filepath.Join(path, tableNames[1])
		legacyIDBs.idbCurr = mustOpenLegacyIndexDB(currPath, s)
	}

	if numIDBs > 0 {
		prevPath := filepath.Join(path, tableNames[0])
		legacyIDBs.idbPrev = mustOpenLegacyIndexDB(prevPath, s)
	}

	return legacyIDBs
}

func (s *Storage) hasLegacyIndexDBs() bool {
	return s.legacyIndexDBs.Load() != nil
}

func (s *Storage) getLegacyIndexDBs() *legacyIndexDBs {
	legacyIDBs := s.legacyIndexDBs.Load()
	legacyIDBs.incRef()
	return legacyIDBs
}

func (s *Storage) putLegacyIndexDBs(legacyIDBs *legacyIndexDBs) {
	legacyIDBs.decRef()
}

func (s *Storage) legacyNextRetentionSeconds() int64 {
	return s.legacyNextRotationTimestamp.Load() - int64(fasttime.UnixTimestamp())
}

func (s *Storage) startLegacyRetentionWatcher() {
	if !s.hasLegacyIndexDBs() {
		return
	}
	s.legacyRetentionWatcherWG.Go(s.legacyRetentionWatcher)
}

func (s *Storage) legacyRetentionWatcher() {
	for {
		d := s.legacyNextRetentionSeconds()
		select {
		case <-s.stopCh:
			return
		case currentTime := <-time.After(time.Second * time.Duration(d)):
			s.legacyMustRotateIndexDB(currentTime)
			if !s.hasLegacyIndexDBs() {
				return
			}
		}
	}
}

// LegacySetRetentionTimezoneOffset sets the offset, which is used for
// calculating the time for legacy indexdb rotation.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/2574
func LegacySetRetentionTimezoneOffset(offset time.Duration) {
	legacyRetentionTimezoneOffsetSecs = int64(offset.Seconds())
}

var legacyRetentionTimezoneOffsetSecs int64

func legacyNextRetentionDeadlineSeconds(atSecs, retentionSecs, offsetSecs int64) int64 {
	// Round retentionSecs to days. This guarantees that per-day inverted index works as expected
	const secsPerDay = 24 * 3600
	retentionSecs = ((retentionSecs + secsPerDay - 1) / secsPerDay) * secsPerDay

	// Schedule the deadline to +4 hours from the next retention period start
	// because of historical reasons - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/248
	offsetSecs -= 4 * 3600

	// Make sure that offsetSecs doesn't exceed retentionSecs
	offsetSecs %= retentionSecs

	// align the retention deadline to multiples of retentionSecs
	// This makes the deadline independent of atSecs.
	deadline := ((atSecs + offsetSecs + retentionSecs - 1) / retentionSecs) * retentionSecs

	// Apply the provided offsetSecs
	deadline -= offsetSecs

	return deadline
}

func (s *Storage) legacyCreateSnapshot(snapshotName, srcDir, dstDir string) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	idbSnapshot := filepath.Join(srcDir, indexdbDirname, snapshotsDirname, snapshotName)
	if idbPrev := legacyIDBs.getIDBPrev(); idbPrev != nil {
		prevSnapshot := filepath.Join(idbSnapshot, idbPrev.name)
		idbPrev.tb.LegacyMustCreateSnapshotAt(prevSnapshot)
	}
	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		currSnapshot := filepath.Join(idbSnapshot, idbCurr.name)
		idbCurr.tb.LegacyMustCreateSnapshotAt(currSnapshot)
	}
	dstIdbDir := filepath.Join(dstDir, indexdbDirname)
	fs.MustSymlinkRelative(idbSnapshot, dstIdbDir)
}

func (s *Storage) legacyMustRotateIndexDB(currentTime time.Time) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		// No legacy indexDBs, nothing to rotate.
		return
	}

	legacyIDBs.idbPrev.scheduleToDrop()
	legacyIDBs.idbPrev.decRef()

	var rotatedLegacyIDBs *legacyIndexDBs

	if legacyIDBs.idbCurr != nil {
		rotatedLegacyIDBs = &legacyIndexDBs{
			idbPrev: legacyIDBs.idbCurr,
		}
	}
	s.legacyIndexDBs.Store(rotatedLegacyIDBs)

	// Update nextRotationTimestamp
	nextRotationTimestamp := currentTime.Unix() + s.retentionMsecs/1000
	s.legacyNextRotationTimestamp.Store(nextRotationTimestamp)
}

func (s *Storage) legacyDeleteSeries(qt *querytracer.Tracer, tfss []*TagFilters, maxMetrics int) (*uint64set.Set, error) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		// No legacy indexDBs, nothing to delete.
		return nil, nil
	}

	all := &uint64set.Set{}

	if idbPrev := legacyIDBs.getIDBPrev(); idbPrev != nil {
		qt.Printf("start deleting from previous legacy indexDB")
		dmis, err := idbPrev.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return nil, err
		}
		qt.Printf("deleted %d metricIDs from previous legacy indexDB", dmis.Len())
		all.UnionMayOwn(dmis)
	}

	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		qt.Printf("start deleting from current legacy indexDB")
		dmis, err := idbCurr.DeleteSeries(qt, tfss, maxMetrics)
		if err != nil {
			return nil, err
		}
		qt.Printf("deleted %d metricIDs from current legacy indexDB", dmis.Len())
		all.UnionMayOwn(dmis)
	}

	return all, nil
}

func (s *Storage) legacyDebugFlush() {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	if idbPrev := legacyIDBs.getIDBPrev(); idbPrev != nil {
		idbPrev.tb.DebugFlush()
	}
	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		idbCurr.tb.DebugFlush()
	}
}

func (s *Storage) legacyNotifyReadWriteMode() {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	if idbPrev := legacyIDBs.getIDBPrev(); idbPrev != nil {
		idbPrev.tb.NotifyReadWriteMode()
	}
	if idbCurr := legacyIDBs.getIDBCurr(); idbCurr != nil {
		idbCurr.tb.NotifyReadWriteMode()
	}
}

func (s *Storage) legacyUpdateMetrics(m *Metrics) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs == nil {
		return
	}

	if legacyIDBs.idbPrev != nil {
		legacyIDBs.idbPrev.UpdateMetrics(&m.TableMetrics.IndexDBMetrics)
	}
	if legacyIDBs.idbCurr != nil {
		legacyIDBs.idbCurr.UpdateMetrics(&m.TableMetrics.IndexDBMetrics)
	}
}

func (s *Storage) legacyMustCloseIndexDBs() {
	legacyIDBs := s.legacyIndexDBs.Load()
	if legacyIDBs == nil {
		return
	}

	if legacyIDBs.idbPrev != nil {
		legacyIDBs.idbPrev.MustClose()
	}
	if legacyIDBs.idbCurr != nil {
		legacyIDBs.idbCurr.MustClose()
	}
}

func (s *Storage) legacyGetTSDBStatus(qt *querytracer.Tracer, tfss []*TagFilters, date uint64, focusLabel string, topN, maxMetrics int, deadline uint64) (*TSDBStatus, error) {
	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	if legacyIDBs.getIDBCurr() != nil {
		idbName := legacyIDBs.getIDBCurr().name
		qt.Printf("collect TSDB status in current legacy indexDB %s", idbName)
		res, err := legacyIDBs.getIDBCurr().GetTSDBStatus(qt, tfss, date, focusLabel, topN, maxMetrics, deadline)
		if err != nil {
			return nil, err
		}
		if res.hasEntries() {
			qt.Printf("collected TSDB status in current legacy indexDB %s", idbName)
			return res, nil
		} else {
			qt.Printf("TSDB status was not found in current legacy indexDB %s", idbName)
		}
	}

	if legacyIDBs.getIDBPrev() != nil {
		idbName := legacyIDBs.getIDBPrev().name
		qt.Printf("collect TSDB status in previous legacy indexDB %s", idbName)
		res, err := legacyIDBs.getIDBPrev().GetTSDBStatus(qt, tfss, date, focusLabel, topN, maxMetrics, deadline)
		if err != nil {
			return nil, err
		}
		if res.hasEntries() {
			qt.Printf("collected TSDB status in previous legacy indexDB %s", idbName)
			return res, nil
		} else {
			qt.Printf("TSDB status was not found in previous legacy indexDB %s", idbName)
		}
	}

	return &TSDBStatus{}, nil
}
