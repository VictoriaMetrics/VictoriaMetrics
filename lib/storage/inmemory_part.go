package storage

import (
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// inmemoryPart represents in-memory partition.
type inmemoryPart struct {
	ph partHeader

	timestampsData bytesutil.ByteBuffer
	valuesData     bytesutil.ByteBuffer
	indexData      bytesutil.ByteBuffer
	metaindexData  bytesutil.ByteBuffer

	creationTime uint64
}

// Reset resets mp.
func (mp *inmemoryPart) Reset() {
	mp.ph.Reset()

	mp.timestampsData.Reset()
	mp.valuesData.Reset()
	mp.indexData.Reset()
	mp.metaindexData.Reset()

	mp.creationTime = 0
}

// MustStoreToDisk stores the mp to the given path on disk.
func (mp *inmemoryPart) MustStoreToDisk(path string) {
	fs.MustMkdirFailIfExist(path)

	timestampsPath := filepath.Join(path, timestampsFilename)
	fs.MustWriteSync(timestampsPath, mp.timestampsData.B)

	valuesPath := filepath.Join(path, valuesFilename)
	fs.MustWriteSync(valuesPath, mp.valuesData.B)

	indexPath := filepath.Join(path, indexFilename)
	fs.MustWriteSync(indexPath, mp.indexData.B)

	metaindexPath := filepath.Join(path, metaindexFilename)
	fs.MustWriteSync(metaindexPath, mp.metaindexData.B)

	mp.ph.MustWriteMetadata(path)

	fs.MustSyncPath(path)
	// Do not sync parent directory - it must be synced by the caller.
}

// InitFromRows initializes mp from the given rows.
func (mp *inmemoryPart) InitFromRows(rows []rawRow) {
	if len(rows) == 0 {
		logger.Panicf("BUG: Inmemory.InitFromRows must accept at least one row")
	}

	mp.Reset()
	rrm := getRawRowsMarshaler()
	rrm.marshalToInmemoryPart(mp, rows)
	putRawRowsMarshaler(rrm)
	mp.creationTime = fasttime.UnixTimestamp()
}

// NewPart creates new part from mp.
//
// It is safe calling NewPart multiple times.
// It is unsafe re-using mp while the returned part is in use.
func (mp *inmemoryPart) NewPart() *part {
	size := mp.size()
	return newPart(&mp.ph, "", size, mp.metaindexData.NewReader(), &mp.timestampsData, &mp.valuesData, &mp.indexData)
}

func (mp *inmemoryPart) size() uint64 {
	return uint64(cap(mp.timestampsData.B) + cap(mp.valuesData.B) + cap(mp.indexData.B) + cap(mp.metaindexData.B))
}

func getInmemoryPart() *inmemoryPart {
	select {
	case mp := <-mpPool:
		return mp
	default:
		return &inmemoryPart{}
	}
}

func putInmemoryPart(mp *inmemoryPart) {
	mp.Reset()
	select {
	case mpPool <- mp:
	default:
		// Drop mp in order to reduce memory usage.
	}
}

// Use chan instead of sync.Pool in order to reduce memory usage on systems with big number of CPU cores,
// since sync.Pool maintains per-CPU pool of inmemoryPart objects.
//
// The inmemoryPart object size can exceed 64KB, so it is better to use chan instead of sync.Pool for reducing memory usage.
var mpPool = make(chan *inmemoryPart, cgroup.AvailableCPUs())
