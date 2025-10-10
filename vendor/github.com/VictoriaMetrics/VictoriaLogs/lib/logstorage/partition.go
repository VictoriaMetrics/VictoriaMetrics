package logstorage

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot/snapshotutil"
)

// PartitionStats contains stats for the partition.
type PartitionStats struct {
	DatadbStats
	IndexdbStats
}

type partition struct {
	// s is the parent storage for the partition
	s *Storage

	// path is the path to the partition directory
	path string

	// name is the partition name. It is basically the directory name obtained from path.
	// It is used for creating keys for partition caches.
	name string

	// idb is indexdb used for the given partition
	idb *indexdb

	// ddb is the datadb used for the given partition
	ddb *datadb

	// The snapshotLock prevents from concurrent creation of snapshots,
	// since this may result in snapshots without recently added data,
	// which may be in the process of flushing to disk by concurrently running
	// snapshot process.
	snapshotLock sync.Mutex
}

// mustCreatePartition creates a partition at the given path.
//
// The created partition can be opened with mustOpenPartition() after is has been created.
//
// The created partition can be deleted with mustDeletePartition() when it is no longer needed.
func mustCreatePartition(path string) {
	fs.MustMkdirFailIfExist(path)

	indexdbPath := filepath.Join(path, indexdbDirname)
	mustCreateIndexdb(indexdbPath)

	datadbPath := filepath.Join(path, datadbDirname)
	mustCreateDatadb(datadbPath)

	fs.MustSyncPathAndParentDir(path)
}

// mustDeletePartition deletes partition at the given path.
//
// The partition must be closed with MustClose before deleting it.
func mustDeletePartition(path string) {
	fs.MustRemoveDir(path)
}

// mustOpenPartition opens partition at the given path for the given Storage.
//
// The returned partition must be closed when no longer needed with mustClosePartition() call.
func mustOpenPartition(s *Storage, path string) *partition {
	name := filepath.Base(path)

	indexdbPath := filepath.Join(path, indexdbDirname)
	isIndexDBExist := fs.IsPathExist(indexdbPath)

	datadbPath := filepath.Join(path, datadbDirname)
	isDatadbExist := fs.IsPathExist(datadbPath)

	if !isIndexDBExist {
		if isDatadbExist {
			logger.Panicf("FATAL: indexdb directory %s is missing, but datadb directory %s exists. "+
				"This indicates corruption. Manually remove the %s partition to resolve it (partition data will be lost)",
				indexdbPath, datadbPath, path)
		}

		logger.Warnf("creating missing indexdb directory %s, this could happen if VictoriaLogs shuts down uncleanly "+
			"(via OOM crash, a panic, SIGKILL or hardware shutdown) while creating new per-day partition", indexdbPath)
		mustCreateIndexdb(indexdbPath)
	}

	idb := mustOpenIndexdb(indexdbPath, name, s)

	// Start initializing the partition
	pt := &partition{
		s:    s,
		path: path,
		name: name,
		idb:  idb,
	}

	if !isDatadbExist {
		logger.Warnf("creating missing datadb directory %s, this could happen if VictoriaLogs shuts down uncleanly "+
			"(via OOM crash, a panic, SIGKILL or hardware shutdown) while creating new per-day partition", datadbPath)
		mustCreateDatadb(datadbPath)
	}

	pt.ddb = mustOpenDatadb(pt, datadbPath, s.flushInterval)

	return pt
}

// mustClosePartition closes pt.
//
// The caller must ensure that pt is no longer used before the call to mustClosePartition().
//
// The partition can be deleted if needed after it is closed via mustDeletePartition() call.
func mustClosePartition(pt *partition) {
	// Close indexdb
	mustCloseIndexdb(pt.idb)
	pt.idb = nil

	// Close datadb
	mustCloseDatadb(pt.ddb)
	pt.ddb = nil

	pt.name = ""
	pt.path = ""
	pt.s = nil
}

func (pt *partition) mustAddRows(lr *LogRows) {
	// Register rows in indexdb
	var pendingRows []int
	streamIDs := lr.streamIDs
	for i := range lr.timestamps {
		streamID := &streamIDs[i]
		if pt.hasStreamIDInCache(streamID) {
			continue
		}
		if len(pendingRows) == 0 || !streamIDs[pendingRows[len(pendingRows)-1]].equal(streamID) {
			pendingRows = append(pendingRows, i)
		}
	}
	if len(pendingRows) > 0 {
		logNewStreams := pt.s.logNewStreams
		streamTagsCanonicals := lr.streamTagsCanonicals
		sort.Slice(pendingRows, func(i, j int) bool {
			return streamIDs[pendingRows[i]].less(&streamIDs[pendingRows[j]])
		})
		for i, rowIdx := range pendingRows {
			streamID := &streamIDs[rowIdx]
			if i > 0 && streamIDs[pendingRows[i-1]].equal(streamID) {
				continue
			}
			if pt.hasStreamIDInCache(streamID) {
				continue
			}
			if !pt.idb.hasStreamID(streamID) {
				streamTagsCanonical := streamTagsCanonicals[rowIdx]
				pt.idb.mustRegisterStream(streamID, streamTagsCanonical)
				if logNewStreams {
					pt.logNewStream(streamTagsCanonical, lr.rows[rowIdx])
				}
			}
			pt.putStreamIDToCache(streamID)
		}
	}

	// Add rows to datadb
	pt.ddb.mustAddRows(lr)
	if pt.s.logIngestedRows {
		pt.logIngestedRows(lr)
	}
}

func (pt *partition) logNewStream(streamTagsCanonical string, fields []Field) {
	streamTags := getStreamTagsString(streamTagsCanonical)
	line := MarshalFieldsToJSON(nil, fields)
	logger.Infof("partition %s: new stream %s for log entry %s", pt.path, streamTags, line)
}

func (pt *partition) logIngestedRows(lr *LogRows) {
	for i := range lr.rows {
		s := lr.GetRowString(i)
		logger.Infof("partition %s: new log entry %s", pt.path, s)
	}
}

func (pt *partition) hasStreamIDInCache(sid *streamID) bool {
	bb := bbPool.Get()
	bb.B = pt.marshalStreamIDCacheKey(bb.B, sid)
	_, ok := pt.s.streamIDCache.Get(bb.B)
	bbPool.Put(bb)

	return ok
}

func (pt *partition) putStreamIDToCache(sid *streamID) {
	bb := bbPool.Get()
	bb.B = pt.marshalStreamIDCacheKey(bb.B, sid)
	pt.s.streamIDCache.Set(bb.B, nil)
	bbPool.Put(bb)
}

func (pt *partition) marshalStreamIDCacheKey(dst []byte, sid *streamID) []byte {
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(pt.name))
	dst = sid.marshal(dst)
	return dst
}

// debugFlush makes sure that all the recently ingested data data becomes searchable
func (pt *partition) debugFlush() {
	pt.ddb.debugFlush()
	pt.idb.debugFlush()
}

// mustCreateSnapshot creates snapshot for the the given pt and returns full path to the created snapshot.
func (pt *partition) mustCreateSnapshot() string {
	logger.Infof("creating a snapshot for partition %q", pt.name)
	startTime := time.Now()

	pt.snapshotLock.Lock()
	defer pt.snapshotLock.Unlock()

	snapshotName := snapshotutil.NewName()
	dstDir := filepath.Join(pt.path, snapshotsDirname, snapshotName)
	fs.MustMkdirFailIfExist(dstDir)

	dstIndexdbDir := filepath.Join(dstDir, indexdbDirname)
	pt.idb.mustCreateSnapshotAt(dstIndexdbDir)

	dstDatadbDir := filepath.Join(dstDir, datadbDirname)
	pt.ddb.mustCreateSnapshotAt(dstDatadbDir)

	fs.MustSyncPathAndParentDir(dstDir)

	snapshotPath, err := filepath.Abs(dstDir)
	if err != nil {
		logger.Panicf("FATAL: cannot obtain absolute path to snapshot: %s", err)
	}

	logger.Infof("created a snapshot for partition %q at %q in %.3f seconds", pt.name, dstDir, time.Since(startTime).Seconds())

	return snapshotPath
}

func (pt *partition) updateStats(ps *PartitionStats) {
	pt.ddb.updateStats(&ps.DatadbStats)
	pt.idb.updateStats(&ps.IndexdbStats)
}

// mustForceMerge runs forced merge for all the parts in pt.
func (pt *partition) mustForceMerge() {
	pt.ddb.mustForceMergeAllParts()
}

func getPartitionDayFromName(name string) (int64, error) {
	t, err := time.Parse(partitionNameFormat, name)
	if err != nil {
		return 0, fmt.Errorf("cannot parse partition name %q; it must have the format YYYYMMDD: %w", name, err)
	}
	day := t.UTC().UnixNano() / nsecsPerDay
	return day, nil
}

func getPartitionNameFromDay(day int64) string {
	name := time.Unix(0, day*nsecsPerDay).UTC().Format(partitionNameFormat)
	return name
}

const partitionNameFormat = "20060102"
