package mergeset

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/filestream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

func getMaxCachedIndexBlocksPerPart() int {
	maxCachedIndexBlocksPerPartOnce.Do(func() {
		n := memory.Allowed() / 1024 / 1024 / 4
		if n == 0 {
			n = 10
		}
		maxCachedIndexBlocksPerPart = n
	})
	return maxCachedIndexBlocksPerPart
}

var (
	maxCachedIndexBlocksPerPart     int
	maxCachedIndexBlocksPerPartOnce sync.Once
)

func getMaxCachedInmemoryBlocksPerPart() int {
	maxCachedInmemoryBlocksPerPartOnce.Do(func() {
		n := memory.Allowed() / 1024 / 1024 / 4
		if n == 0 {
			n = 10
		}
		maxCachedInmemoryBlocksPerPart = n
	})
	return maxCachedInmemoryBlocksPerPart
}

var (
	maxCachedInmemoryBlocksPerPart     int
	maxCachedInmemoryBlocksPerPartOnce sync.Once
)

type part struct {
	ph partHeader

	path string

	size uint64

	mrs []metaindexRow

	indexFile fs.ReadAtCloser
	itemsFile fs.ReadAtCloser
	lensFile  fs.ReadAtCloser

	idxbCache indexBlockCache
	ibCache   inmemoryBlockCache
}

func openFilePart(path string) (*part, error) {
	path = filepath.Clean(path)

	var ph partHeader
	if err := ph.ParseFromPath(path); err != nil {
		return nil, fmt.Errorf("cannot parse path to part: %s", err)
	}

	metaindexPath := path + "/metaindex.bin"
	metaindexFile, err := filestream.Open(metaindexPath, true)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %s", metaindexPath, err)
	}
	metaindexSize := fs.MustFileSize(metaindexPath)

	indexPath := path + "/index.bin"
	indexFile, err := fs.OpenReaderAt(indexPath)
	if err != nil {
		metaindexFile.MustClose()
		return nil, fmt.Errorf("cannot open %q: %s", indexPath, err)
	}
	indexSize := fs.MustFileSize(indexPath)

	itemsPath := path + "/items.bin"
	itemsFile, err := fs.OpenReaderAt(itemsPath)
	if err != nil {
		metaindexFile.MustClose()
		indexFile.MustClose()
		return nil, fmt.Errorf("cannot open %q: %s", itemsPath, err)
	}
	itemsSize := fs.MustFileSize(itemsPath)

	lensPath := path + "/lens.bin"
	lensFile, err := fs.OpenReaderAt(lensPath)
	if err != nil {
		metaindexFile.MustClose()
		indexFile.MustClose()
		itemsFile.MustClose()
		return nil, fmt.Errorf("cannot open %q: %s", lensPath, err)
	}
	lensSize := fs.MustFileSize(lensPath)

	size := metaindexSize + indexSize + itemsSize + lensSize
	return newPart(&ph, path, size, metaindexFile, indexFile, itemsFile, lensFile)
}

func newPart(ph *partHeader, path string, size uint64, metaindexReader filestream.ReadCloser, indexFile, itemsFile, lensFile fs.ReadAtCloser) (*part, error) {
	var errors []error
	mrs, err := unmarshalMetaindexRows(nil, metaindexReader)
	if err != nil {
		errors = append(errors, fmt.Errorf("cannot unmarshal metaindexRows: %s", err))
	}
	metaindexReader.MustClose()

	p := &part{
		path: path,
		size: size,
		mrs:  mrs,

		indexFile: indexFile,
		itemsFile: itemsFile,
		lensFile:  lensFile,
	}
	p.ph.CopyFrom(ph)
	p.idxbCache.Init()
	p.ibCache.Init()

	if len(errors) > 0 {
		// Return only the first error, since it has no sense in returning all errors.
		err := fmt.Errorf("error opening part %s: %s", p.path, errors[0])
		p.MustClose()
		return nil, err
	}
	return p, nil
}

func (p *part) MustClose() {
	p.indexFile.MustClose()
	p.itemsFile.MustClose()
	p.lensFile.MustClose()

	p.idxbCache.Reset()
	p.ibCache.Reset()
}

type indexBlock struct {
	bhs []blockHeader
}

func getIndexBlock() *indexBlock {
	v := indexBlockPool.Get()
	if v == nil {
		return &indexBlock{}
	}
	return v.(*indexBlock)
}

func putIndexBlock(idxb *indexBlock) {
	idxb.bhs = idxb.bhs[:0]
	indexBlockPool.Put(idxb)
}

var indexBlockPool sync.Pool

type indexBlockCache struct {
	m         map[uint64]*indexBlock
	missesMap map[uint64]uint64
	mu        sync.RWMutex

	requests uint64
	misses   uint64
}

func (idxbc *indexBlockCache) Init() {
	idxbc.m = make(map[uint64]*indexBlock)
	idxbc.missesMap = make(map[uint64]uint64)
	idxbc.requests = 0
	idxbc.misses = 0
}

func (idxbc *indexBlockCache) Reset() {
	atomic.AddUint64(&indexBlockCacheRequests, idxbc.requests)
	atomic.AddUint64(&indexBlockCacheMisses, idxbc.misses)
	// It is safe returning idxbc.m to pool, since the Reset must be called
	// when the idxbc entries are no longer accessed by concurrent goroutines.
	for _, idxb := range idxbc.m {
		putIndexBlock(idxb)
	}
	idxbc.Init()
}

var (
	indexBlockCacheRequests uint64
	indexBlockCacheMisses   uint64
)

func (idxbc *indexBlockCache) Get(k uint64) *indexBlock {
	atomic.AddUint64(&idxbc.requests, 1)
	idxbc.mu.RLock()
	idxb := idxbc.m[k]
	idxbc.mu.RUnlock()

	if idxb != nil {
		return idxb
	}
	atomic.AddUint64(&idxbc.misses, 1)
	idxbc.mu.Lock()
	idxbc.missesMap[k]++
	idxbc.mu.Unlock()
	return nil
}

// Put puts idxb under the key k into idxbc.
//
// Returns true if the idxb has been put into idxbc.
func (idxbc *indexBlockCache) Put(k uint64, idxb *indexBlock) bool {
	idxbc.mu.Lock()

	if idxbc.missesMap[k] < 2 {
		// Do not pollute cache with infrequently accessed items, since they may
		// evict frequently accessed items.
		idxbc.mu.Unlock()
		return false
	}

	// Remove superflouos entries.
	if overflow := len(idxbc.m) - getMaxCachedIndexBlocksPerPart(); overflow > 0 {
		// Remove 10% of items from the cache.
		overflow = int(float64(len(idxbc.m)) * 0.1)
		for k := range idxbc.m {
			// Do not return idxb to pool, since these entries may be used
			// by concurrent goroutines.
			delete(idxbc.m, k)
			overflow--
			if overflow == 0 {
				break
			}
		}
	}
	if overflow := len(idxbc.missesMap) - 8*getMaxCachedIndexBlocksPerPart(); overflow > 0 {
		// Remove 10% of items from the cache.
		overflow = int(float64(len(idxbc.missesMap)) * 0.1)
		for k := range idxbc.missesMap {
			delete(idxbc.missesMap, k)
			overflow--
			if overflow == 0 {
				break
			}
		}
	}

	// Store the frequently accessed idxb in the cache.
	delete(idxbc.missesMap, k)
	idxbc.m[k] = idxb
	idxbc.mu.Unlock()
	return true
}

func (idxbc *indexBlockCache) Len() uint64 {
	idxbc.mu.RLock()
	n := len(idxbc.m)
	idxbc.mu.RUnlock()
	return uint64(n)
}

func (idxbc *indexBlockCache) Requests() uint64 {
	return atomic.LoadUint64(&idxbc.requests)
}

func (idxbc *indexBlockCache) Misses() uint64 {
	return atomic.LoadUint64(&idxbc.misses)
}

type inmemoryBlockCache struct {
	m         map[inmemoryBlockCacheKey]*inmemoryBlock
	missesMap map[inmemoryBlockCacheKey]uint64
	mu        sync.RWMutex

	requests uint64
	misses   uint64
}

type inmemoryBlockCacheKey struct {
	firstItem        string
	itemsBlockOffset uint64
}

func (ibck *inmemoryBlockCacheKey) Init(bh *blockHeader) {
	ibck.firstItem = ""
	if bh.itemsBlockSize == 0 {
		ibck.firstItem = string(bh.firstItem)
	}
	ibck.itemsBlockOffset = bh.itemsBlockOffset
}

func (ibc *inmemoryBlockCache) Init() {
	ibc.m = make(map[inmemoryBlockCacheKey]*inmemoryBlock)
	ibc.missesMap = make(map[inmemoryBlockCacheKey]uint64)
	ibc.requests = 0
	ibc.misses = 0
}

func (ibc *inmemoryBlockCache) Reset() {
	atomic.AddUint64(&inmemoryBlockCacheRequests, ibc.requests)
	atomic.AddUint64(&inmemoryBlockCacheMisses, ibc.misses)
	// It is safe returning ibc.m entries to pool, since the Reset function may be called
	// only if no other goroutines access ibc entries.
	for _, ib := range ibc.m {
		putInmemoryBlock(ib)
	}
	ibc.Init()
}

var (
	inmemoryBlockCacheRequests uint64
	inmemoryBlockCacheMisses   uint64
)

func (ibc *inmemoryBlockCache) Get(k inmemoryBlockCacheKey) *inmemoryBlock {
	atomic.AddUint64(&ibc.requests, 1)

	ibc.mu.RLock()
	ib := ibc.m[k]
	ibc.mu.RUnlock()

	if ib != nil {
		return ib
	}
	atomic.AddUint64(&ibc.misses, 1)
	ibc.mu.Lock()
	ibc.missesMap[k]++
	ibc.mu.Unlock()
	return nil
}

// Put puts ib under key k into ibc.
//
// Returns true if ib was put into ibc.
func (ibc *inmemoryBlockCache) Put(k inmemoryBlockCacheKey, ib *inmemoryBlock) bool {
	ibc.mu.Lock()

	if ibc.missesMap[k] < 2 {
		// Do not cache entry with low number of accesses, since it may evict
		// frequently accessed entries from the cache.
		ibc.mu.Unlock()
		return false
	}

	// Clean superflouos entries in ibc.m and ibc.missesMap.
	if overflow := len(ibc.m) - getMaxCachedInmemoryBlocksPerPart(); overflow > 0 {
		// Remove 10% of items from the cache.
		overflow = int(float64(len(ibc.m)) * 0.1)
		for k := range ibc.m {
			// do not call putInmemoryBlock(ib), since the ib
			// may be used by concurrent goroutines.
			delete(ibc.m, k)
			overflow--
			if overflow == 0 {
				break
			}
		}
	}
	if overflow := len(ibc.missesMap) - 8*getMaxCachedInmemoryBlocksPerPart(); overflow > 0 {
		// Remove 10% of items from the cache.
		overflow = int(float64(len(ibc.missesMap)) * 0.1)
		for k := range ibc.missesMap {
			delete(ibc.missesMap, k)
			overflow--
			if overflow == 0 {
				break
			}
		}
	}

	// The entry is frequently accessed, so store it in the cache.
	delete(ibc.missesMap, k)
	ibc.m[k] = ib
	ibc.mu.Unlock()
	return true
}

func (ibc *inmemoryBlockCache) Len() uint64 {
	ibc.mu.RLock()
	n := len(ibc.m)
	ibc.mu.RUnlock()
	return uint64(n)
}

func (ibc *inmemoryBlockCache) Requests() uint64 {
	return atomic.LoadUint64(&ibc.requests)
}

func (ibc *inmemoryBlockCache) Misses() uint64 {
	return atomic.LoadUint64(&ibc.misses)
}
