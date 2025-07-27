package mergeset

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

var idxbCache = blockcache.NewCache(getMaxIndexBlocksCacheSize)
var ibCache = blockcache.NewCache(getMaxInmemoryBlocksCacheSize)
var ibSparseCache = blockcache.NewCache(getMaxInmemoryBlocksSparseCacheSize)

// SetIndexBlocksCacheSize overrides the default size of indexdb/indexBlocks cache
func SetIndexBlocksCacheSize(size int) {
	maxIndexBlockCacheSize = size
}

func getMaxIndexBlocksCacheSize() int {
	maxIndexBlockCacheSizeOnce.Do(func() {
		if maxIndexBlockCacheSize <= 0 {
			maxIndexBlockCacheSize = int(0.10 * float64(memory.Allowed()))
		}
	})
	return maxIndexBlockCacheSize
}

var (
	maxIndexBlockCacheSize     int
	maxIndexBlockCacheSizeOnce sync.Once
)

// SetDataBlocksCacheSize overrides the default size of indexdb/dataBlocks cache
func SetDataBlocksCacheSize(size int) {
	maxInmemoryBlockCacheSize = size
}

func getMaxInmemoryBlocksCacheSize() int {
	maxInmemoryBlockCacheSizeOnce.Do(func() {
		if maxInmemoryBlockCacheSize <= 0 {
			maxInmemoryBlockCacheSize = int(0.25 * float64(memory.Allowed()))
		}
	})
	return maxInmemoryBlockCacheSize
}

// SetDataBlocksSparseCacheSize overrides the default size of indexdb/dataBlocksSparse cache
func SetDataBlocksSparseCacheSize(size int) {
	maxInmemorySparseMergeCacheSize = size
}

func getMaxInmemoryBlocksSparseCacheSize() int {
	maxInmemoryBlockSparseCacheSizeOnce.Do(func() {
		if maxInmemorySparseMergeCacheSize <= 0 {
			maxInmemorySparseMergeCacheSize = int(0.05 * float64(memory.Allowed()))
		}
	})
	return maxInmemorySparseMergeCacheSize
}

var (
	maxInmemoryBlockCacheSize     int
	maxInmemoryBlockCacheSizeOnce sync.Once

	maxInmemorySparseMergeCacheSize     int
	maxInmemoryBlockSparseCacheSizeOnce sync.Once
)

type part struct {
	ph partHeader

	path string

	size uint64

	mrs []metaindexRow

	indexFile fs.MustReadAtCloser
	itemsFile fs.MustReadAtCloser
	lensFile  fs.MustReadAtCloser
}

func mustOpenFilePart(path string) *part {
	var ph partHeader
	ph.MustReadMetadata(path)

	metaindexPath := filepath.Join(path, metaindexFilename)
	metaindexFile := filestream.MustOpen(metaindexPath, true)
	metaindexSize := fs.MustFileSize(metaindexPath)

	indexPath := filepath.Join(path, indexFilename)
	indexFile := fs.MustOpenReaderAt(indexPath)
	indexSize := fs.MustFileSize(indexPath)

	itemsPath := filepath.Join(path, itemsFilename)
	itemsFile := fs.MustOpenReaderAt(itemsPath)
	itemsSize := fs.MustFileSize(itemsPath)

	lensPath := filepath.Join(path, lensFilename)
	lensFile := fs.MustOpenReaderAt(lensPath)
	lensSize := fs.MustFileSize(lensPath)

	size := metaindexSize + indexSize + itemsSize + lensSize
	return newPart(&ph, path, size, metaindexFile, indexFile, itemsFile, lensFile)
}

func newPart(ph *partHeader, path string, size uint64, metaindexReader filestream.ReadCloser, indexFile, itemsFile, lensFile fs.MustReadAtCloser) *part {
	mrs, err := unmarshalMetaindexRows(nil, metaindexReader)
	if err != nil {
		logger.Panicf("FATAL: cannot unmarshal metaindexRows from %q: %s", path, err)
	}
	metaindexReader.MustClose()

	var p part
	p.path = path
	p.size = size
	p.mrs = mrs

	p.indexFile = indexFile
	p.itemsFile = itemsFile
	p.lensFile = lensFile

	p.ph.CopyFrom(ph)
	return &p
}

func (p *part) MustClose() {
	// Close files in parallel in order to speed up this process on storage systems with high latency
	// such as NFS or Cepth.
	cs := []fs.MustCloser{
		p.indexFile,
		p.itemsFile,
		p.lensFile,
	}
	fs.MustCloseParallel(cs)

	idxbCache.RemoveBlocksForPart(p)
	ibCache.RemoveBlocksForPart(p)
	ibSparseCache.RemoveBlocksForPart(p)
}

type indexBlock struct {
	bhs []blockHeader

	// The buffer for holding the data referred by bhs
	buf []byte
}

func (idxb *indexBlock) SizeBytes() int {
	bhs := idxb.bhs[:cap(idxb.bhs)]
	n := int(unsafe.Sizeof(*idxb))
	for i := range bhs {
		n += bhs[i].SizeBytes()
	}
	return n
}
