package storage

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
		n := memory.Allowed() / 1024 / 1024 / 8
		if n < 16 {
			n = 16
		}
		maxCachedIndexBlocksPerPart = n
	})
	return maxCachedIndexBlocksPerPart
}

var (
	maxCachedIndexBlocksPerPart     int
	maxCachedIndexBlocksPerPartOnce sync.Once
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

	ibCache *indexBlockCache
}

// openFilePart opens file-based part from the given path.
func openFilePart(path string) (*part, error) {
	path = filepath.Clean(path)

	var ph partHeader
	if err := ph.ParseFromPath(path); err != nil {
		return nil, fmt.Errorf("cannot parse path to part: %w", err)
	}

	timestampsPath := path + "/timestamps.bin"
	timestampsFile, err := fs.OpenReaderAt(timestampsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open timestamps file: %w", err)
	}
	timestampsSize := fs.MustFileSize(timestampsPath)

	valuesPath := path + "/values.bin"
	valuesFile, err := fs.OpenReaderAt(valuesPath)
	if err != nil {
		timestampsFile.MustClose()
		return nil, fmt.Errorf("cannot open values file: %w", err)
	}
	valuesSize := fs.MustFileSize(valuesPath)

	indexPath := path + "/index.bin"
	indexFile, err := fs.OpenReaderAt(indexPath)
	if err != nil {
		timestampsFile.MustClose()
		valuesFile.MustClose()
		return nil, fmt.Errorf("cannot open index file: %w", err)
	}
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
	p.ibCache = newIndexBlockCache()

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

	isBig := p.ph.RowsCount > maxRowsPerSmallPart()
	p.ibCache.MustClose(isBig)
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

func putIndexBlock(ib *indexBlock) {
	ib.bhs = ib.bhs[:0]
	indexBlockPool.Put(ib)
}

var indexBlockPool sync.Pool

type indexBlockCache struct {
	// Put atomic counters to the top of struct in order to align them to 8 bytes on 32-bit architectures.
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

	ib *indexBlock
}

func newIndexBlockCache() *indexBlockCache {
	var ibc indexBlockCache
	ibc.m = make(map[uint64]*indexBlockCacheEntry)

	ibc.cleanerStopCh = make(chan struct{})
	ibc.cleanerWG.Add(1)
	go func() {
		defer ibc.cleanerWG.Done()
		ibc.cleaner()
	}()
	return &ibc
}

func (ibc *indexBlockCache) MustClose(isBig bool) {
	close(ibc.cleanerStopCh)
	ibc.cleanerWG.Wait()

	// It is safe returning ibc.m itemst to the pool, since Reset must
	// be called only when no other goroutines access ibc entries.
	for _, ibe := range ibc.m {
		putIndexBlock(ibe.ib)
	}
	ibc.m = nil
}

// cleaner periodically cleans least recently used items.
func (ibc *indexBlockCache) cleaner() {
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

func (ibc *indexBlockCache) cleanByTimeout() {
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

func (ibc *indexBlockCache) Get(k uint64) *indexBlock {
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

func (ibc *indexBlockCache) Put(k uint64, ib *indexBlock) {
	ibc.mu.Lock()

	// Clean superflouos cache entries.
	if overflow := len(ibc.m) - getMaxCachedIndexBlocksPerPart(); overflow > 0 {
		// Remove 10% of items from the cache.
		overflow = int(float64(len(ibc.m)) * 0.1)
		for k := range ibc.m {
			// Do not call putIndexBlock on ibc.m entries, since they may be used by concurrent goroutines.
			delete(ibc.m, k)
			overflow--
			if overflow == 0 {
				break
			}
		}
	}

	// Store frequently requested ib in the cache.
	ibe := &indexBlockCacheEntry{
		lastAccessTime: fasttime.UnixTimestamp(),
		ib:             ib,
	}
	ibc.m[k] = ibe
	ibc.mu.Unlock()
}

func (ibc *indexBlockCache) Requests() uint64 {
	return atomic.LoadUint64(&ibc.requests)
}

func (ibc *indexBlockCache) Misses() uint64 {
	return atomic.LoadUint64(&ibc.misses)
}

func (ibc *indexBlockCache) Len() uint64 {
	ibc.mu.Lock()
	n := uint64(len(ibc.m))
	ibc.mu.Unlock()
	return n
}
