package storage

import (
	"fmt"
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

// StoreToDisk stores the mp to the given path on disk.
func (mp *inmemoryPart) StoreToDisk(path string) error {
	if err := fs.MkdirAllIfNotExist(path); err != nil {
		return fmt.Errorf("cannot create directory %q: %w", path, err)
	}
	timestampsPath := path + "/timestamps.bin"
	if err := fs.WriteFileAndSync(timestampsPath, mp.timestampsData.B); err != nil {
		return fmt.Errorf("cannot store timestamps: %w", err)
	}
	valuesPath := path + "/values.bin"
	if err := fs.WriteFileAndSync(valuesPath, mp.valuesData.B); err != nil {
		return fmt.Errorf("cannot store values: %w", err)
	}
	indexPath := path + "/index.bin"
	if err := fs.WriteFileAndSync(indexPath, mp.indexData.B); err != nil {
		return fmt.Errorf("cannot store index: %w", err)
	}
	metaindexPath := path + "/metaindex.bin"
	if err := fs.WriteFileAndSync(metaindexPath, mp.metaindexData.B); err != nil {
		return fmt.Errorf("cannot store metaindex: %w", err)
	}
	if err := mp.ph.writeMinDedupInterval(path); err != nil {
		return fmt.Errorf("cannot store min dedup interval: %w", err)
	}
	// Sync parent directory in order to make sure the written files remain visible after hardware reset
	parentDirPath := filepath.Dir(path)
	fs.MustSyncPath(parentDirPath)
	return nil
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
func (mp *inmemoryPart) NewPart() (*part, error) {
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
