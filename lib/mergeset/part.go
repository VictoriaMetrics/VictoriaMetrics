package mergeset

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
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

	indexFile fs.MustReadAtCloser
	itemsFile fs.MustReadAtCloser
	lensFile  fs.MustReadAtCloser

	idxbCache *indexBlockCache
	ibCache   *inmemoryBlockCache
}

func openFilePart(path string) (*part, error) {
	path = filepath.Clean(path)

	var ph partHeader
	if err := ph.ParseFromPath(path); err != nil {
		return nil, fmt.Errorf("cannot parse path to part: %w", err)
	}

	metaindexPath := path + "/metaindex.bin"
	metaindexFile, err := filestream.Open(metaindexPath, true)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %w", metaindexPath, err)
	}
	metaindexSize := fs.MustFileSize(metaindexPath)

	indexPath := path + "/index.bin"
	indexFile, err := fs.OpenReaderAt(indexPath)
	if err != nil {
		metaindexFile.MustClose()
		return nil, fmt.Errorf("cannot open %q: %w", indexPath, err)
	}
	indexSize := fs.MustFileSize(indexPath)

	itemsPath := path + "/items.bin"
	itemsFile, err := fs.OpenReaderAt(itemsPath)
	if err != nil {
		metaindexFile.MustClose()
		indexFile.MustClose()
		return nil, fmt.Errorf("cannot open %q: %w", itemsPath, err)
	}
	itemsSize := fs.MustFileSize(itemsPath)

	lensPath := path + "/lens.bin"
	lensFile, err := fs.OpenReaderAt(lensPath)
	if err != nil {
		metaindexFile.MustClose()
		indexFile.MustClose()
		itemsFile.MustClose()
		return nil, fmt.Errorf("cannot open %q: %w", lensPath, err)
	}
	lensSize := fs.MustFileSize(lensPath)

	size := metaindexSize + indexSize + itemsSize + lensSize
	return newPart(&ph, path, size, metaindexFile, indexFile, itemsFile, lensFile)
}

func newPart(ph *partHeader, path string, size uint64, metaindexReader filestream.ReadCloser, indexFile, itemsFile, lensFile fs.MustReadAtCloser) (*part, error) {
	var errors []error
	mrs, err := unmarshalMetaindexRows(nil, metaindexReader)
	if err != nil {
		errors = append(errors, fmt.Errorf("cannot unmarshal metaindexRows: %w", err))
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
	p.idxbCache = newIndexBlockCache()
	p.ibCache = newInmemoryBlockCache()

	if len(errors) > 0 {
		// Return only the first error, since it has no sense in returning all errors.
		err := fmt.Errorf("error opening part %s: %w", p.path, errors[0])
		p.MustClose()
		return nil, err
	}
	return &p, nil
}

func (p *part) MustClose() {
	p.indexFile.MustClose()
	p.itemsFile.MustClose()
	p.lensFile.MustClose()

	p.idxbCache.MustClose()
	p.ibCache.MustClose()
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
	// Atomically updated counters must go first in the struct, so they are properly
	// aligned to 8 bytes on 32-bit architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212
	requests uint64
	misses   uint64

	m  map[uint64]*indexBlockCacheEntry
	mu sync.RWMutex

	cleanerStopCh chan struct{}
	cleanerWG     sync.WaitGroup
}

type indexBlockCacheEntry struct {
	// Atomically updated counters must go first in the struct, so they are properly
	// aligned to 8 bytes on 32-bit architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212
	lastAccessTime uint64

	idxb *indexBlock
}

func newIndexBlockCache() *indexBlockCache {
	var idxbc indexBlockCache
	idxbc.m = make(map[uint64]*indexBlockCacheEntry)
	idxbc.cleanerStopCh = make(chan struct{})
	idxbc.cleanerWG.Add(1)
	go func() {
		defer idxbc.cleanerWG.Done()
		idxbc.cleaner()
	}()
	return &idxbc
}

func (idxbc *indexBlockCache) MustClose() {
	close(idxbc.cleanerStopCh)
	idxbc.cleanerWG.Wait()

	// It is safe returning idxbc.m to pool, since the Reset must be called
	// when the idxbc entries are no longer accessed by concurrent goroutines.
	for _, idxbe := range idxbc.m {
		putIndexBlock(idxbe.idxb)
	}
	idxbc.m = nil
}

// cleaner periodically cleans least recently used items.
func (idxbc *indexBlockCache) cleaner() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			idxbc.cleanByTimeout()
		case <-idxbc.cleanerStopCh:
			return
		}
	}
}

func (idxbc *indexBlockCache) cleanByTimeout() {
	currentTime := fasttime.UnixTimestamp()
	idxbc.mu.Lock()
	for k, idxbe := range idxbc.m {
		// Delete items accessed more than 10 minutes ago.
		if currentTime-atomic.LoadUint64(&idxbe.lastAccessTime) > 10*60 {
			delete(idxbc.m, k)
		}
	}
	idxbc.mu.Unlock()
}

func (idxbc *indexBlockCache) Get(k uint64) *indexBlock {
	atomic.AddUint64(&idxbc.requests, 1)
	idxbc.mu.RLock()
	idxbe := idxbc.m[k]
	idxbc.mu.RUnlock()

	if idxbe != nil {
		currentTime := fasttime.UnixTimestamp()
		if atomic.LoadUint64(&idxbe.lastAccessTime) != currentTime {
			atomic.StoreUint64(&idxbe.lastAccessTime, currentTime)
		}
		return idxbe.idxb
	}
	atomic.AddUint64(&idxbc.misses, 1)
	return nil
}

// Put puts idxb under the key k into idxbc.
func (idxbc *indexBlockCache) Put(k uint64, idxb *indexBlock) {
	idxbc.mu.Lock()

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

	// Store idxb in the cache.
	idxbe := &indexBlockCacheEntry{
		lastAccessTime: fasttime.UnixTimestamp(),
		idxb:           idxb,
	}
	idxbc.m[k] = idxbe
	idxbc.mu.Unlock()
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
	// Atomically updated counters must go first in the struct, so they are properly
	// aligned to 8 bytes on 32-bit architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212
	requests uint64
	misses   uint64

	m  map[inmemoryBlockCacheKey]*inmemoryBlockCacheEntry
	mu sync.RWMutex

	cleanerStopCh chan struct{}
	cleanerWG     sync.WaitGroup
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

type inmemoryBlockCacheEntry struct {
	// Atomically updated counters must go first in the struct, so they are properly
	// aligned to 8 bytes on 32-bit architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212
	lastAccessTime uint64

	ib *inmemoryBlock
}

func newInmemoryBlockCache() *inmemoryBlockCache {
	var ibc inmemoryBlockCache
	ibc.m = make(map[inmemoryBlockCacheKey]*inmemoryBlockCacheEntry)

	ibc.cleanerStopCh = make(chan struct{})
	ibc.cleanerWG.Add(1)
	go func() {
		defer ibc.cleanerWG.Done()
		ibc.cleaner()
	}()
	return &ibc
}

func (ibc *inmemoryBlockCache) MustClose() {
	close(ibc.cleanerStopCh)
	ibc.cleanerWG.Wait()

	// It is safe returning ibc.m entries to pool, since the Reset function may be called
	// only if no other goroutines access ibc entries.
	for _, ibe := range ibc.m {
		putInmemoryBlock(ibe.ib)
	}
	ibc.m = nil
}

// cleaner periodically cleans least recently used items.
func (ibc *inmemoryBlockCache) cleaner() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ibc.cleanByTimeout()
		case <-ibc.cleanerStopCh:
			return
		}
	}
}

func (ibc *inmemoryBlockCache) cleanByTimeout() {
	currentTime := fasttime.UnixTimestamp()
	ibc.mu.Lock()
	for k, ibe := range ibc.m {
		// Delete items accessed more than 10 minutes ago.
		if currentTime-atomic.LoadUint64(&ibe.lastAccessTime) > 10*60 {
			delete(ibc.m, k)
		}
	}
	ibc.mu.Unlock()
}

func (ibc *inmemoryBlockCache) Get(k inmemoryBlockCacheKey) *inmemoryBlock {
	atomic.AddUint64(&ibc.requests, 1)

	ibc.mu.RLock()
	ibe := ibc.m[k]
	ibc.mu.RUnlock()

	if ibe != nil {
		currentTime := fasttime.UnixTimestamp()
		if atomic.LoadUint64(&ibe.lastAccessTime) != currentTime {
			atomic.StoreUint64(&ibe.lastAccessTime, currentTime)
		}
		return ibe.ib
	}
	atomic.AddUint64(&ibc.misses, 1)
	return nil
}

// Put puts ib under key k into ibc.
func (ibc *inmemoryBlockCache) Put(k inmemoryBlockCacheKey, ib *inmemoryBlock) {
	ibc.mu.Lock()

	// Clean superflouos entries in cache.
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

	// Store ib in the cache.
	ibe := &inmemoryBlockCacheEntry{
		lastAccessTime: fasttime.UnixTimestamp(),
		ib:             ib,
	}
	ibc.m[k] = ibe
	ibc.mu.Unlock()
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
