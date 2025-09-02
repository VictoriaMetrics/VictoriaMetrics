package metricsmetadata

import (
	"bytes"
	"container/heap"
	"sort"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

const (
	// bucketsCount is the number of buckets for the store.
	bucketsCount = 8
	// size of buffer to be used for cloning metric name and help
	metricNameHelpBufSize = 4 * 1024

	// size of rows buffer to be used for cloning Row
	rowsBufSize = 512

	metadataExpireDuration = time.Hour
)

var bbPool bytesutil.ByteBufferPool

// Store for metrics metadata
type Store struct {
	buckets [bucketsCount]*bucket

	maxSizeBytes  int
	cleanerStopCh chan struct{}

	wg sync.WaitGroup
}

// NewStore returns new initialized Store.
func NewStore(maxSizeBytes int) *Store {
	s := &Store{
		cleanerStopCh: make(chan struct{}),
		maxSizeBytes:  maxSizeBytes,
	}

	maxShardBytes := maxSizeBytes / bucketsCount
	for i := range bucketsCount {
		s.buckets[i] = &bucket{
			perTenantStorage: make(map[uint64]map[string]*Row),
			maxSizeBytes:     int64(maxShardBytes),
		}
	}
	s.wg.Add(1)
	go s.cleaner()
	return s
}

// MustClose closes the store and waits for all background tasks to finish.
func (s *Store) MustClose() {
	close(s.cleanerStopCh)
	s.wg.Wait()
}

// Add adds rows to the store.
func (s *Store) Add(rows []Row) {
	if len(rows) == 0 {
		return
	}

	now := fasttime.UnixTimestamp()
	bb := bbPool.Get()
	for _, mr := range rows {
		var bucketIDx uint64
		bucketIDx = xxhash.Sum64(mr.MetricFamilyName)
		bucketIDx %= bucketsCount
		s.buckets[bucketIDx].add(&mr, now)
	}
	bbPool.Put(bb)
}

// GetForTenant returns rows for the given tenant, limit and optional metricName
//
// can only be used for cluster version
func (s *Store) GetForTenant(accountID, projectID uint32, limit int, metricName string) []*Row {
	tenantID := encodeTenantID(accountID, projectID)
	if len(metricName) > 0 {
		return s.getRowForTenantIDByMetricName(tenantID, metricName)
	}

	totalItems := s.totalItems()
	dst := make([]*Row, 0, totalItems)
	for _, b := range s.buckets {
		b.mu.Lock()
		ts, ok := b.perTenantStorage[tenantID]
		if !ok {
			b.mu.Unlock()
			continue
		}
		for _, v := range ts {
			dst = append(dst, v)
		}
		b.mu.Unlock()
	}
	sortRows(dst)
	if limit > 0 && len(dst) > limit {
		dst = dst[:limit]
	}
	return dst
}

func (s *Store) getRowForTenantIDByMetricName(tenantID uint64, metricName string) []*Row {
	for _, b := range s.buckets {
		b.mu.Lock()
		ts, ok := b.perTenantStorage[tenantID]
		if !ok {
			b.mu.Unlock()
			continue
		}
		row := ts[metricName]
		b.mu.Unlock()
		if row != nil {
			return []*Row{row}
		}
	}
	return nil
}

// Get returns rows for the given limit and optional metricName
func (s *Store) Get(limit int, metricName string) []*Row {
	if len(metricName) > 0 {
		return s.getFirstRowByMetricName(metricName)
	}
	totalItems := s.totalItems()
	dst := make([]*Row, 0, totalItems)
	for _, b := range s.buckets {
		b.mu.Lock()
		dst = append(dst, b.lah...)
		b.mu.Unlock()
	}
	sortRows(dst)
	if limit > 0 && len(dst) > limit {
		dst = dst[:limit]
	}
	return dst
}

func (s *Store) getFirstRowByMetricName(metricName string) []*Row {
	for _, b := range s.buckets {
		b.mu.Lock()
		for _, ts := range b.perTenantStorage {
			row := ts[metricName]
			if row != nil {
				// return first matched row
				b.mu.Unlock()
				return []*Row{row}
			}
		}
		b.mu.Unlock()
	}
	return nil
}

// MetadataStoreMetrics contains metrics for the store.
type MetadataStoreMetrics struct {
	ItemsCurrent     uint64
	CurrentSizeBytes uint64
	MaxSizeBytes     uint64
}

// UpdateMetrics updates dst with metrics store metrics.
func (s *Store) UpdateMetrics(dst *MetadataStoreMetrics) {
	for _, b := range s.buckets {
		dst.CurrentSizeBytes += uint64(b.itemsTotalSize.Load())
		dst.ItemsCurrent += uint64(b.itemsCurrent.Load())
	}
	dst.MaxSizeBytes = uint64(s.maxSizeBytes)
}

func (s *Store) cleaner() {
	defer s.wg.Done()
	d := timeutil.AddJitterToDuration(time.Minute)
	ticker := time.NewTicker(d)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanerStopCh:
			return
		case <-ticker.C:
			s.cleanByTimeout()
		}
	}
}

func (s *Store) cleanByTimeout() {
	for _, b := range s.buckets {
		b.cleanByTimeout()
	}
}

func (s *Store) totalItems() int {
	var itemsCount int
	for _, b := range s.buckets {
		itemsCount += int(b.itemsCurrent.Load())
	}
	return itemsCount
}

type bucket struct {
	maxSizeBytes   int64
	itemsCurrent   atomic.Int64
	itemsTotalSize atomic.Int64

	// mu protects fields below
	mu               sync.Mutex
	perTenantStorage map[uint64]map[string]*Row

	// The heap for removing the least recently used entries from metricsMetadataStorage.
	lah lastAccessHeap

	metricNamesBuf []byte
	rowsBuff       []Row
}

func (b *bucket) cloneRowLocked(src *Row) *Row {
	if len(b.rowsBuff) >= cap(b.rowsBuff) {
		// allocate a new slice instead of reallocting exist
		// it saves memory and reduces GC pressure
		b.rowsBuff = make([]Row, 0, rowsBufSize)
	}
	b.rowsBuff = b.rowsBuff[:len(b.rowsBuff)+1]
	mrDst := &b.rowsBuff[len(b.rowsBuff)-1]

	// allocate metricName and help in one go
	mrDst.MetricFamilyName, mrDst.Help = b.cloneMetricNameHelpLocked(src.MetricFamilyName, src.Help)
	mrDst.ProjectID = src.ProjectID
	mrDst.AccountID = src.AccountID
	mrDst.Unit = internUnit(src.Unit)
	mrDst.Type = src.Type

	return mrDst
}

// cloneMetricNameHelpLocked uses the same idea as strings.Clone.
// But instead of direct []byte allocation for each cloned string,
// it allocates metricNamesBuf, copies provided metricName and help into it
// and uses string *byte references for it via subslice.
//
// allocating metricName and help as a single buffer allows GC to free memory for
// row in the same time
func (b *bucket) cloneMetricNameHelpLocked(metricName, help []byte) ([]byte, []byte) {
	if len(metricName) > metricNameHelpBufSize {
		// metricName is too large for default buffer
		// directly allocate it on heap as strings.Clone does
		b := make([]byte, len(metricName)+len(help))
		copy(b, metricName)
		copy(b[len(metricName):], help)
		return b[:len(metricName)], b[len(metricName):]
	}
	idx := len(b.metricNamesBuf)
	n := len(metricName) + len(b.metricNamesBuf) + len(help)
	if n > cap(b.metricNamesBuf) {
		// allocate a new slice instead of reallocting exist
		// it saves memory and reduces GC pressure
		b.metricNamesBuf = make([]byte, 0, metricNameHelpBufSize)
		idx = 0
	}
	b.metricNamesBuf = append(b.metricNamesBuf, metricName...)
	b.metricNamesBuf = append(b.metricNamesBuf, help...)
	return b.metricNamesBuf[idx : idx+len(metricName)], b.metricNamesBuf[idx+len(metricName):]
}

func (b *bucket) add(mr *Row, lastIngestion uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	tenantID := encodeTenantID(mr.AccountID, mr.ProjectID)

	store, ok := b.perTenantStorage[tenantID]
	if !ok {
		store = make(map[string]*Row, rowsBufSize)
		b.perTenantStorage[tenantID] = store
	}

	if existMR, ok := store[string(mr.MetricFamilyName)]; ok {
		if !bytes.Equal(existMR.Help, mr.Help) || !bytes.Equal(existMR.Unit, mr.Unit) || existMR.Type != mr.Type {
			// in case of metadata update, allocate the new row instead of mutation
			// since it could be referenced by get request
			// and it could lead to data race
			mrDst := b.cloneRowLocked(mr)
			mrDst.heapIdx = existMR.heapIdx
			store[bytesutil.ToUnsafeString(mrDst.MetricFamilyName)] = mrDst
			b.lah[mrDst.heapIdx] = mrDst

			b.itemsTotalSize.Add(rowSize(mrDst) - rowSize(existMR))
			existMR = mrDst
		}
		existMR.lastAccessTime = lastIngestion
		heap.Fix(&b.lah, existMR.heapIdx)
		return
	}

	mrDst := b.cloneRowLocked(mr)
	mrDst.heapIdx = len(b.lah)
	mrDst.lastAccessTime = lastIngestion

	heap.Push(&b.lah, mrDst)

	b.itemsCurrent.Add(1)
	b.itemsTotalSize.Add(rowSize(mrDst))
	store[bytesutil.ToUnsafeString(mrDst.MetricFamilyName)] = mrDst

	if b.itemsTotalSize.Load() > b.maxSizeBytes {
		b.removeLeastRecentlyAccessedItemLocked()
	}
}

func (b *bucket) cleanByTimeout() {
	// Delete items accessed more than three minutes ago.
	// This time should be enough for repeated queries.
	deadline := fasttime.UnixTimestamp() - uint64(metadataExpireDuration/time.Second)
	b.mu.Lock()
	defer b.mu.Unlock()

	for len(b.lah) > 0 {
		if deadline < b.lah[0].lastAccessTime {
			break
		}
		b.removeLeastRecentlyAccessedItemLocked()
	}
}

func (b *bucket) removeLeastRecentlyAccessedItemLocked() {
	e := b.lah[0]
	b.itemsTotalSize.Add(-rowSize(e))
	b.itemsCurrent.Add(-1)

	tenantID := encodeTenantID(e.AccountID, e.ProjectID)
	delete(b.perTenantStorage[tenantID], string(e.MetricFamilyName))
	heap.Pop(&b.lah)
}

const (
	perItemOverhead = int64(int(unsafe.Sizeof(Row{})) + 24) // 24 bytes for map overhead
)

func rowSize(r *Row) int64 {
	return perItemOverhead + int64(len(r.MetricFamilyName)+len(r.Help)+len(r.Unit))
}

func sortRows(rows []*Row) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].lastAccessTime == rows[j].lastAccessTime {
			return string(rows[i].MetricFamilyName) < string(rows[j].MetricFamilyName)
		}
		return rows[i].lastAccessTime < rows[j].lastAccessTime
	})
}

// lastAccessHeap implements heap.Interface
type lastAccessHeap []*Row

func (lah *lastAccessHeap) Len() int {
	return len(*lah)
}
func (lah *lastAccessHeap) Swap(i, j int) {
	h := *lah
	a := h[i]
	b := h[j]
	a.heapIdx = j
	b.heapIdx = i
	h[i] = b
	h[j] = a
}
func (lah *lastAccessHeap) Less(i, j int) bool {
	h := *lah
	return h[i].lastAccessTime < h[j].lastAccessTime
}
func (lah *lastAccessHeap) Push(x any) {
	e := x.(*Row)
	h := *lah
	e.heapIdx = len(h)
	*lah = append(h, e)
}
func (lah *lastAccessHeap) Pop() any {
	h := *lah
	e := h[len(h)-1]

	// Remove the reference to deleted entry, so Go GC could free up memory occupied by the deleted entry.
	h[len(h)-1] = nil

	*lah = h[:len(h)-1]
	return e
}

func encodeTenantID(accountID, projectID uint32) uint64 {
	return uint64(accountID)<<32 | uint64(projectID)
}

var unitInternStore sync.Map

// units are statically defined and cannot have high cardinality
func internUnit(unit []byte) []byte {
	v, ok := unitInternStore.Load(string(unit))
	if ok {
		return v.([]byte)
	}
	b := make([]byte, len(unit))
	copy(b, unit)
	unitInternStore.Store(string(b), b)
	return b
}
