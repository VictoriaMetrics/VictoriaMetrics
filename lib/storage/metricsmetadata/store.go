package metricsmetadata

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

const (
	// storeBucketsCount is the number of bucketsLock for the store.
	storeBucketsCount = 512
	// size of buffer to be used for cloning metric names
	metricNameBufSize = 2 * 1024
	// Metadata items which were not ingested for storeMetadataTTL are deleted from the store.
	storeMetadataTTL = 10 * time.Minute
	// storeRotationInterval is the interval for swapping current and prev bucketsLock.
	storeRotationInterval = 15 * time.Minute
)

type metadataKey struct {
	accountID        uint32
	projectID        uint32
	metricFamilyName string
}

func (k metadataKey) hash(tmp []byte) ([]byte, uint64) {
	tmp = encoding.MarshalUint32(tmp, k.accountID)
	tmp = encoding.MarshalUint32(tmp, k.projectID)
	tmp = append(tmp, k.metricFamilyName...)
	return tmp, xxhash.Sum64(tmp)
}

// MetadataStoreMetrics contains metrics for the store.
type MetadataStoreMetrics struct {
	ItemsCurrent     int64
	CurrentSizeBytes uint64
	MaxSizeBytes     uint64
}

type bucket struct {
	mu                     sync.RWMutex
	metricsMetadataStorage map[metadataKey][]Row
	timingInfo             map[metadataKey]uint64

	metricNamesBuf []byte

	itemsCurrent   *atomic.Int64
	itemsTotalSize *atomic.Int64
}

func (b *bucket) resetLocked() {
	clear(b.timingInfo)
	clear(b.metricsMetadataStorage)
}

// cloneMetricNameLocked uses the same idea as strings.Clone.
// But instead of direct []byte allocation for each cloned string,
// it allocates metricNamesBuf, copies provided metricName into it
// and uses string *byte references for it via subslice.
func (b *bucket) cloneMetricNameLocked(metricName []byte) string {
	if len(metricName) > metricNameBufSize {
		// metricName is too large for default buffer
		// directly allocate it on heap as strings.Clone does
		b := make([]byte, len(metricName))
		copy(b, metricName)
		return bytesutil.ToUnsafeString(b)
	}
	idx := len(b.metricNamesBuf)
	n := len(metricName) + len(b.metricNamesBuf)
	if n > cap(b.metricNamesBuf) {
		// allocate a new slice instead of reallocting exist
		// it saves memory and reduces GC pressure
		b.metricNamesBuf = make([]byte, 0, metricNameBufSize)
		idx = 0
	}
	b.metricNamesBuf = append(b.metricNamesBuf, metricName...)
	return bytesutil.ToUnsafeString(b.metricNamesBuf[idx:])
}

func (b *bucket) get(dst []Row, limit, limitPerMetric int64, metric string, keepFilter func(k metadataKey) bool) []Row {
	if limit < 0 {
		limit = 0
	}
	if limitPerMetric < 0 {
		limitPerMetric = 0
	}

	metricLen := len(metric)
	b.mu.RLock()
	defer b.mu.RUnlock()

	for k, rows := range b.metricsMetadataStorage {
		if metricLen > 0 && k.metricFamilyName != metric {
			continue
		}

		perMetric := int64(0)
		for _, r := range rows {
			if keepFilter != nil && !keepFilter(k) {
				continue
			}

			if limit > 0 && len(dst) >= int(limit) {
				return dst
			}

			dst = append(dst, r)
			perMetric++

			if limitPerMetric > 0 && perMetric >= limitPerMetric {
				break
			}
		}
	}

	return dst
}

func (b *bucket) add(key metadataKey, mr Row, lastIngestion uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key.metricFamilyName = b.cloneMetricNameLocked(mr.MetricFamilyName)
	b.timingInfo[key] = lastIngestion
	trackRowIngested := func(newItem bool) {
		b.itemsCurrent.Add(1)
		totalAdd := rowSize(mr)
		if newItem {
			// storage and timing info entries
			totalAdd += 2 * keySize(key)
		}
		b.itemsTotalSize.Add(totalAdd)
	}

	metadataRows, ok := b.metricsMetadataStorage[key]
	if !ok {
		b.metricsMetadataStorage[key] = make([]Row, 0, 1)
		b.metricsMetadataStorage[key] = append(b.metricsMetadataStorage[key], mr)
		trackRowIngested(true)
		return
	}

	found := false
	for _, v := range metadataRows {
		if v.Type == mr.Type && bytes.Equal(mr.Unit, v.Unit) && bytes.Equal(mr.Help, v.Help) {
			found = true
			break
		}
	}

	if found {
		return
	}
	b.metricsMetadataStorage[key] = append(metadataRows, mr)
	trackRowIngested(false)
}

// Store for metrics metadata
type Store struct {
	currentBuckets atomic.Pointer[[]*bucket]
	prevBuckets    atomic.Pointer[[]*bucket]

	itemsCurrent   atomic.Int64
	itemsTotalSize atomic.Int64

	rotationInterval time.Duration
	rotationStopCh   chan struct{}

	maxSizeBytes         int
	maxSizeWatcherStopCh chan struct{}

	backgroundTasks sync.WaitGroup
}

// NewStore returns new initialized Store.
func NewStore(maxSizeBytes int) *Store {
	s := &Store{
		rotationInterval:     storeRotationInterval,
		rotationStopCh:       make(chan struct{}),
		maxSizeWatcherStopCh: make(chan struct{}),
		maxSizeBytes:         maxSizeBytes,
	}

	var (
		current = make([]*bucket, storeBucketsCount)
		prev    = make([]*bucket, storeBucketsCount)
	)
	for i := range storeBucketsCount {
		current[i] = &bucket{
			metricsMetadataStorage: make(map[metadataKey][]Row),
			timingInfo:             make(map[metadataKey]uint64),

			itemsCurrent:   &s.itemsCurrent,
			itemsTotalSize: &s.itemsTotalSize,
		}
		prev[i] = &bucket{
			metricsMetadataStorage: make(map[metadataKey][]Row),
			timingInfo:             make(map[metadataKey]uint64),

			itemsCurrent:   &s.itemsCurrent,
			itemsTotalSize: &s.itemsTotalSize,
		}
	}
	s.currentBuckets.Store(&current)
	s.prevBuckets.Store(&prev)

	s.backgroundTasks.Add(2)
	go s.runRotationScheduler()
	go s.runMaxSizeWatcher()

	return s
}

// MustClose closes the store and waits for all background tasks to finish.
func (s *Store) MustClose() {
	close(s.rotationStopCh)
	close(s.maxSizeWatcherStopCh)
	s.backgroundTasks.Wait()
}

// Add adds rows to the store.
func (s *Store) Add(rows []Row) error {
	if len(rows) == 0 {
		return nil
	}

	now := fasttime.UnixTimestamp()
	bb := bbPool.Get()
	for _, mr := range rows {
		key := metadataKey{
			accountID:        mr.AccountID,
			projectID:        mr.ProjectID,
			metricFamilyName: bytesutil.ToUnsafeString(mr.MetricFamilyName),
		}
		var bucketIDx uint64
		bb.B, bucketIDx = key.hash(bb.B[:0])
		buckets := *s.currentBuckets.Load()
		bucketIDx %= uint64(len(buckets))
		buckets[bucketIDx].add(key, mr, now)
	}
	bbPool.Put(bb)

	return nil
}

var bbPool bytesutil.ByteBufferPool

func (s *Store) runMaxSizeWatcher() {
	defer s.backgroundTasks.Done()

	t := time.NewTicker(10 * time.Second)
	defer t.Stop()

	usageThreshold := 0.9
	maxUsage := int64(float64(s.maxSizeBytes) * usageThreshold)

	defaultTTL := uint64(storeMetadataTTL.Seconds())

	for {
		select {
		case <-t.C:
			currentSize := s.itemsTotalSize.Load()
			if currentSize > maxUsage {
				now := fasttime.UnixTimestamp()
				threshold := now - defaultTTL
				s.evict(threshold)
			}

			// check if force rotation helped to reclaim memory
			// if not - delete items for half of the TTL
			currentSize = s.itemsTotalSize.Load()
			if currentSize > maxUsage {
				now := fasttime.UnixTimestamp()
				threshold := now - defaultTTL/2
				s.evict(threshold)
			}

			// last resort, delete everything
			currentSize = s.itemsTotalSize.Load()
			if currentSize > maxUsage {
				s.evict(fasttime.UnixTimestamp())
			}
		case <-s.maxSizeWatcherStopCh:
			return
		}
	}
}

func (s *Store) runRotationScheduler() {
	defer s.backgroundTasks.Done()

	ticker := time.NewTicker(s.rotationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := fasttime.UnixTimestamp()
			threshold := now - uint64(storeMetadataTTL.Seconds())
			s.evict(threshold)
		case <-s.rotationStopCh:
			return
		}
	}
}

// evict deletes items which are were not ingested after threshold.
func (s *Store) evict(threshold uint64) {
	// Moves items from src to dst if they are not expired yet.
	move := func(src, dst []*bucket) {
		for i := range src {
			b := src[i]
			b.mu.Lock()
			for key, lastIngestion := range b.timingInfo {
				itemsToDelete := uint64(len(b.metricsMetadataStorage[key]))
				s.itemsCurrent.Add(-int64(itemsToDelete))
				sizeDiff := -2 * keySize(key)
				for _, row := range b.metricsMetadataStorage[key] {
					rs := rowSize(row)
					sizeDiff -= rs
				}
				s.itemsTotalSize.Add(sizeDiff)

				if lastIngestion < threshold {
					continue
				}
				for _, row := range b.metricsMetadataStorage[key] {
					dst[i].add(key, row, lastIngestion)
				}
			}
			b.resetLocked()
			b.mu.Unlock()
		}
	}

	// Pre-fill prevBuckets with current items which are not expired yet
	// And store as current items to direct new writes to these items
	currentBuckets := *s.currentBuckets.Load()
	prevBuckets := *s.prevBuckets.Load()
	move(currentBuckets, prevBuckets)
	s.currentBuckets.Store(&prevBuckets)
	s.prevBuckets.Store(&currentBuckets)

	// Move items which might have been written to currentBuckets before swap once again
	// And reset currentBuckets to empty
	move(currentBuckets, prevBuckets)
}

func (s *Store) get(limit, limitPerMetric int64, metric string, keepFilter func(k metadataKey) bool) []Row {
	if limit < 0 {
		limit = 0
	}
	if limitPerMetric < 0 {
		limitPerMetric = 0
	}

	prealloc := int(limit)
	if limit == 0 {
		// assume that we will return all entries
		prealloc = int(s.itemsCurrent.Load())
	}
	res := make([]Row, 0, prealloc)
	buckets := *s.currentBuckets.Load()
	for i := range buckets {
		res = buckets[i].get(res, limit, limitPerMetric, metric, keepFilter)
	}

	return res
}

// GetForTenant returns rows for the given tenant, metric and limits.
func (s *Store) GetForTenant(accountID, projectID uint32, limit, limitPerMetric int64, metric string) []Row {
	keepFilter := func(k metadataKey) bool {
		return k.accountID == accountID && k.projectID == projectID
	}
	return s.get(limit, limitPerMetric, metric, keepFilter)
}

// Get returns rows for the given metric and limits.
func (s *Store) Get(limit, limitPerMetric int64, metric string) []Row {
	return s.get(limit, limitPerMetric, metric, nil)
}

// UpdateMetrics updates dst with metrics store metrics.
func (s *Store) UpdateMetrics(dst *MetadataStoreMetrics) {
	dst.ItemsCurrent = s.itemsCurrent.Load()
	dst.CurrentSizeBytes = uint64(s.itemsTotalSize.Load())
	dst.MaxSizeBytes = uint64(s.maxSizeBytes)
}

const (
	perItemOverhead = int64(int(unsafe.Sizeof(Row{})) + 24) // 24 bytes for map overhead
)

func rowSize(r Row) int64 {
	return perItemOverhead + int64(len(r.MetricFamilyName)+len(r.Help)+len(r.Unit))
}

func keySize(k metadataKey) int64 {
	return int64(unsafe.Sizeof(k)) + int64(len(k.metricFamilyName))
}
