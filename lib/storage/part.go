package storage

import (
	"fmt"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/blockcache"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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

// openFilePart opens file-based part from the given path.
func openFilePart(path string) (*part, error) {
	path = filepath.Clean(path)

	var ph partHeader
	if err := ph.ParseFromPath(path); err != nil {
		return nil, fmt.Errorf("cannot parse path to part: %w", err)
	}

	timestampsPath := path + "/timestamps.bin"
	timestampsFile := fs.MustOpenReaderAt(timestampsPath)
	timestampsSize := fs.MustFileSize(timestampsPath)

	valuesPath := path + "/values.bin"
	valuesFile := fs.MustOpenReaderAt(valuesPath)
	valuesSize := fs.MustFileSize(valuesPath)

	indexPath := path + "/index.bin"
	indexFile := fs.MustOpenReaderAt(indexPath)
	indexSize := fs.MustFileSize(indexPath)

	metaindexPath := path + "/metaindex.bin"
	metaindexFile, err := filestream.Open(metaindexPath, true)
	if err != nil {
		timestampsFile.MustClose()
		valuesFile.MustClose()
		indexFile.MustClose()
		return nil, fmt.Errorf("cannot open metaindex file: %w", err)
	}
	metaindexSize := fs.MustFileSize(metaindexPath)

	size := timestampsSize + valuesSize + indexSize + metaindexSize
	return newPart(&ph, path, size, metaindexFile, timestampsFile, valuesFile, indexFile)
}

// newPart returns new part initialized with the given arguments.
//
// The returned part calls MustClose on all the files passed to newPart
// when calling part.MustClose.
func newPart(ph *partHeader, path string, size uint64, metaindexReader filestream.ReadCloser, timestampsFile, valuesFile, indexFile fs.MustReadAtCloser) (*part, error) {
	var errors []error
	metaindex, err := unmarshalMetaindexRows(nil, metaindexReader)
	if err != nil {
		errors = append(errors, fmt.Errorf("cannot unmarshal metaindex data: %w", err))
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

	if len(errors) > 0 {
		// Return only the first error, since it has no sense in returning all errors.
		err = fmt.Errorf("cannot initialize part %q: %w", &p, errors[0])
		p.MustClose()
		return nil, err
	}

	return &p, nil
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
