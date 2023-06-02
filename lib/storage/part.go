package storage

import (
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/blockcache"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

var ibCache = blockcache.NewCache(getMaxIndexBlocksCacheSize)

func getMaxIndexBlocksCacheSize() int {
	maxIndexBlockCacheSizeOnce.Do(func() {
		maxIndexBlockCacheSize = int(0.1 * float64(memory.Allowed()))
	})
	return maxIndexBlockCacheSize
}

var (
	maxIndexBlockCacheSize     int
	maxIndexBlockCacheSizeOnce sync.Once
)

// part represents a searchable part containing time series data.
type part struct {
	ph partHeader

	// Filesystem path to the part.
	//
	// Empty for in-memory part.
	path string

	// Total size in bytes of part data.
	size uint64

	timestampsFile fs.MustReadAtCloser
	valuesFile     fs.MustReadAtCloser
	indexFile      fs.MustReadAtCloser

	metaindex []metaindexRow
}

// mustOpenFilePart opens file-based part from the given path.
func mustOpenFilePart(path string) *part {
	path = filepath.Clean(path)

	var ph partHeader
	ph.MustReadMetadata(path)

	timestampsPath := filepath.Join(path, timestampsFilename)
	timestampsFile := fs.MustOpenReaderAt(timestampsPath)
	timestampsSize := fs.MustFileSize(timestampsPath)

	valuesPath := filepath.Join(path, valuesFilename)
	valuesFile := fs.MustOpenReaderAt(valuesPath)
	valuesSize := fs.MustFileSize(valuesPath)

	indexPath := filepath.Join(path, indexFilename)
	indexFile := fs.MustOpenReaderAt(indexPath)
	indexSize := fs.MustFileSize(indexPath)

	metaindexPath := filepath.Join(path, metaindexFilename)
	metaindexFile := filestream.MustOpen(metaindexPath, true)
	metaindexSize := fs.MustFileSize(metaindexPath)

	size := timestampsSize + valuesSize + indexSize + metaindexSize
	return newPart(&ph, path, size, metaindexFile, timestampsFile, valuesFile, indexFile)
}

// newPart returns new part initialized with the given arguments.
//
// The returned part calls MustClose on all the files passed to newPart
// when calling part.MustClose.
func newPart(ph *partHeader, path string, size uint64, metaindexReader filestream.ReadCloser, timestampsFile, valuesFile, indexFile fs.MustReadAtCloser) *part {
	metaindex, err := unmarshalMetaindexRows(nil, metaindexReader)
	if err != nil {
		logger.Panicf("FATAL: cannot unmarshal metaindex data from %q: %s", path, err)
	}
	metaindexReader.MustClose()

	var p part
	p.ph = *ph
	p.path = path
	p.size = size
	p.timestampsFile = timestampsFile
	p.valuesFile = valuesFile
	p.indexFile = indexFile
	p.metaindex = metaindex

	return &p
}

// String returns human-readable representation of p.
func (p *part) String() string {
	if len(p.path) > 0 {
		return p.path
	}
	return p.ph.String()
}

// MustClose closes all the part files.
func (p *part) MustClose() {
	p.timestampsFile.MustClose()
	p.valuesFile.MustClose()
	p.indexFile.MustClose()

	ibCache.RemoveBlocksForPart(p)
}

type indexBlock struct {
	bhs []blockHeader
}

func (idxb *indexBlock) SizeBytes() int {
	return cap(idxb.bhs) * int(unsafe.Sizeof(blockHeader{}))
}
