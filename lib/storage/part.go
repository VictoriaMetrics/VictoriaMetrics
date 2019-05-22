package storage

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

	timestampsFile fs.ReadAtCloser
	valuesFile     fs.ReadAtCloser
	indexFile      fs.ReadAtCloser

	metaindex []metaindexRow

	ibCache indexBlockCache
}

// openFilePart opens file-based part from the given path.
func openFilePart(path string) (*part, error) {
	path = filepath.Clean(path)

	var ph partHeader
	if err := ph.ParseFromPath(path); err != nil {
		return nil, fmt.Errorf("cannot parse path to part: %s", err)
	}

	timestampsPath := path + "/timestamps.bin"
	timestampsFile, err := fs.OpenReaderAt(timestampsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open timestamps file: %s", err)
	}

	valuesPath := path + "/values.bin"
	valuesFile, err := fs.OpenReaderAt(valuesPath)
	if err != nil {
		timestampsFile.MustClose()
		return nil, fmt.Errorf("cannot open values file: %s", err)
	}

	indexPath := path + "/index.bin"
	indexFile, err := fs.OpenReaderAt(indexPath)
	if err != nil {
		timestampsFile.MustClose()
		valuesFile.MustClose()
		return nil, fmt.Errorf("cannot open index file: %s", err)
	}

	metaindexPath := path + "/metaindex.bin"
	metaindexFile, err := filestream.Open(metaindexPath, true)
	if err != nil {
		timestampsFile.MustClose()
		valuesFile.MustClose()
		indexFile.MustClose()
		return nil, fmt.Errorf("cannot open metaindex file: %s", err)
	}

	return newPart(&ph, path, metaindexFile, timestampsFile, valuesFile, indexFile)
}

// newPart returns new part initialized with the given arguments.
//
// The returned part calls MustClose on all the files passed to newPart
// when calling part.MustClose.
func newPart(ph *partHeader, path string, metaindexReader filestream.ReadCloser, timestampsFile, valuesFile, indexFile fs.ReadAtCloser) (*part, error) {
	var errors []error
	metaindex, err := unmarshalMetaindexRows(nil, metaindexReader)
	if err != nil {
		errors = append(errors, fmt.Errorf("cannot unmarshal metaindex data: %s", err))
	}
	metaindexReader.MustClose()

	p := &part{
		ph:             *ph,
		path:           path,
		timestampsFile: timestampsFile,
		valuesFile:     valuesFile,
		indexFile:      indexFile,

		metaindex: metaindex,
	}

	if len(errors) > 0 {
		// Return only the first error, since it has no sense in returning all errors.
		err = fmt.Errorf("cannot initialize part %q: %s", p, errors[0])
		p.MustClose()
		return nil, err
	}

	p.ibCache.Init()

	return p, nil
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

	isBig := p.ph.RowsCount > maxRowsPerSmallPart
	p.ibCache.Reset(isBig)
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
	m         map[uint64]*indexBlock
	missesMap map[uint64]uint64
	mu        sync.RWMutex

	requests uint64
	misses   uint64
}

func (ibc *indexBlockCache) Init() {
	ibc.m = make(map[uint64]*indexBlock)
	ibc.missesMap = make(map[uint64]uint64)
	ibc.requests = 0
	ibc.misses = 0
}

func (ibc *indexBlockCache) Reset(isBig bool) {
	if isBig {
		atomic.AddUint64(&bigIndexBlockCacheRequests, ibc.requests)
		atomic.AddUint64(&bigIndexBlockCacheMisses, ibc.misses)
	} else {
		atomic.AddUint64(&smallIndexBlockCacheRequests, ibc.requests)
		atomic.AddUint64(&smallIndexBlockCacheMisses, ibc.misses)
	}
	// It is safe returning ibc.m itemst to the pool, since Reset must
	// be called only when no other goroutines access ibc entries.
	for _, ib := range ibc.m {
		putIndexBlock(ib)
	}
	ibc.Init()
}

var (
	bigIndexBlockCacheRequests uint64
	bigIndexBlockCacheMisses   uint64

	smallIndexBlockCacheRequests uint64
	smallIndexBlockCacheMisses   uint64
)

func (ibc *indexBlockCache) Get(k uint64) *indexBlock {
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

func (ibc *indexBlockCache) Put(k uint64, ib *indexBlock) bool {
	ibc.mu.Lock()

	if ibc.missesMap[k] < 2 {
		// Do not store infrequently accessed ib in the cache,
		// so it don't evict frequently accessed items.
		ibc.mu.Unlock()
		return false
	}

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
	if overflow := len(ibc.missesMap) - 8*getMaxCachedIndexBlocksPerPart(); overflow > 0 {
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

	// Store frequently requested ib in the cache.
	delete(ibc.missesMap, k)
	ibc.m[k] = ib
	ibc.mu.Unlock()
	return true
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
