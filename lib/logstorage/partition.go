package logstorage

import (
	"bytes"
	"path/filepath"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
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
}

// mustDeletePartition deletes partition at the given path.
//
// The partition must be closed with MustClose before deleting it.
func mustDeletePartition(path string) {
	fs.MustRemoveAll(path)
}

// mustOpenPartition opens partition at the given path for the given Storage.
//
// The returned partition must be closed when no longer needed with mustClosePartition() call.
func mustOpenPartition(s *Storage, path string) *partition {
	name := filepath.Base(path)

	// Open indexdb
	indexdbPath := filepath.Join(path, indexdbDirname)
	idb := mustOpenIndexdb(indexdbPath, name, s)

	// Start initializing the partition
	pt := &partition{
		s:    s,
		path: path,
		name: name,
		idb:  idb,
	}

	// Open datadb
	datadbPath := filepath.Join(path, datadbDirname)
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

func (pt *partition) logNewStream(streamTagsCanonical []byte, fields []Field) {
	streamTags := getStreamTagsString(streamTagsCanonical)
	rf := RowFormatter(fields)
	logger.Infof("partition %s: new stream %s for log entry %s", pt.path, streamTags, &rf)
}

func (pt *partition) logIngestedRows(lr *LogRows) {
	for i := range lr.rows {
		s := lr.GetRowString(i)
		logger.Infof("partition %s: new log entry %s", pt.path, s)
	}
}

func (pt *partition) hasStreamIDInCache(sid *streamID) bool {
	var result [1]byte

	bb := bbPool.Get()
	bb.B = pt.marshalStreamIDCacheKey(bb.B, sid)
	value := pt.s.streamIDCache.Get(result[:0], bb.B)
	bbPool.Put(bb)

	return bytes.Equal(value, okValue)
}

func (pt *partition) putStreamIDToCache(sid *streamID) {
	bb := bbPool.Get()
	bb.B = pt.marshalStreamIDCacheKey(bb.B, sid)
	pt.s.streamIDCache.Set(bb.B, okValue)
	bbPool.Put(bb)
}

func (pt *partition) marshalStreamIDCacheKey(dst []byte, sid *streamID) []byte {
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(pt.name))
	dst = sid.marshal(dst)
	return dst
}

var okValue = []byte("1")

// debugFlush makes sure that all the recently ingested data data becomes searchable
func (pt *partition) debugFlush() {
	pt.ddb.debugFlush()
	pt.idb.debugFlush()
}

func (pt *partition) updateStats(ps *PartitionStats) {
	pt.ddb.updateStats(&ps.DatadbStats)
	pt.idb.updateStats(&ps.IndexdbStats)
}
