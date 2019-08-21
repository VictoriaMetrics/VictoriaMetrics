package promql

import (
	"crypto/rand"
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/VictoriaMetrics/metrics"
)

var disableCache = flag.Bool("search.disableCache", false, "Whether to disable response caching. This may be useful during data backfilling")

var rollupResultCacheV = &rollupResultCache{
	c: workingsetcache.New(1024*1024, time.Hour), // This is a cache for testing.
}
var rollupResultCachePath string

func getRollupResultCacheSize() int {
	rollupResultCacheSizeOnce.Do(func() {
		n := memory.Allowed() / 16
		if n <= 0 {
			n = 1024 * 1024
		}
		rollupResultCacheSize = n
	})
	return rollupResultCacheSize
}

var (
	rollupResultCacheSize     int
	rollupResultCacheSizeOnce sync.Once
)

// InitRollupResultCache initializes the rollupResult cache
func InitRollupResultCache(cachePath string) {
	rollupResultCachePath = cachePath
	startTime := time.Now()
	cacheSize := getRollupResultCacheSize()
	var c *workingsetcache.Cache
	if len(rollupResultCachePath) > 0 {
		logger.Infof("loading rollupResult cache from %q...", rollupResultCachePath)
		c = workingsetcache.Load(rollupResultCachePath, cacheSize, time.Hour)
	} else {
		c = workingsetcache.New(cacheSize, time.Hour)
	}
	if *disableCache {
		c.Stop()
		c = nil
	}

	stats := &fastcache.Stats{}
	var statsLock sync.Mutex
	var statsLastUpdate time.Time
	fcs := func() *fastcache.Stats {
		statsLock.Lock()
		defer statsLock.Unlock()

		if time.Since(statsLastUpdate) < time.Second {
			return stats
		}
		var fcs fastcache.Stats
		c.UpdateStats(&fcs)
		stats = &fcs
		statsLastUpdate = time.Now()
		return stats
	}
	if len(rollupResultCachePath) > 0 {
		logger.Infof("loaded rollupResult cache from %q in %s; entriesCount: %d, sizeBytes: %d",
			rollupResultCachePath, time.Since(startTime), fcs().EntriesCount, fcs().BytesSize)
	}

	metrics.NewGauge(`vm_cache_entries{type="promql/rollupResult"}`, func() float64 {
		return float64(fcs().EntriesCount)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="promql/rollupResult"}`, func() float64 {
		return float64(fcs().BytesSize)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="promql/rollupResult"}`, func() float64 {
		return float64(fcs().GetCalls)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="promql/rollupResult"}`, func() float64 {
		return float64(fcs().Misses)
	})

	rollupResultCacheV = &rollupResultCache{
		c: c,
	}
}

// StopRollupResultCache closes the rollupResult cache.
func StopRollupResultCache() {
	if len(rollupResultCachePath) == 0 {
		if !*disableCache {
			rollupResultCacheV.c.Stop()
			rollupResultCacheV.c = nil
		}
		return
	}
	logger.Infof("saving rollupResult cache to %q...", rollupResultCachePath)
	startTime := time.Now()
	if err := rollupResultCacheV.c.Save(rollupResultCachePath); err != nil {
		logger.Errorf("cannot close rollupResult cache at %q: %s", rollupResultCachePath, err)
		return
	}
	var fcs fastcache.Stats
	rollupResultCacheV.c.UpdateStats(&fcs)
	rollupResultCacheV.c.Stop()
	rollupResultCacheV.c = nil
	logger.Infof("saved rollupResult cache to %q in %s; entriesCount: %d, sizeBytes: %d",
		rollupResultCachePath, time.Since(startTime), fcs.EntriesCount, fcs.BytesSize)
}

type rollupResultCache struct {
	c *workingsetcache.Cache
}

var rollupResultCacheResets = metrics.NewCounter(`vm_cache_resets_total{type="promql/rollupResult"}`)

// ResetRollupResultCache resets rollup result cache.
func ResetRollupResultCache() {
	rollupResultCacheResets.Inc()
	rollupResultCacheV.c.Reset()
}

func (rrc *rollupResultCache) Get(funcName string, ec *EvalConfig, me *metricExpr, iafc *incrementalAggrFuncContext, window int64) (tss []*timeseries, newStart int64) {
	if *disableCache || !ec.mayCache() {
		return nil, ec.Start
	}

	// Obtain tss from the cache.
	bb := bbPool.Get()
	defer bbPool.Put(bb)

	bb.B = marshalRollupResultCacheKey(bb.B[:0], funcName, me, iafc, window, ec.Step)
	metainfoBuf := rrc.c.Get(nil, bb.B)
	if len(metainfoBuf) == 0 {
		return nil, ec.Start
	}
	var mi rollupResultCacheMetainfo
	if err := mi.Unmarshal(metainfoBuf); err != nil {
		logger.Panicf("BUG: cannot unmarshal rollupResultCacheMetainfo: %s; it looks like it was improperly saved", err)
	}
	key := mi.GetBestKey(ec.Start, ec.End)
	if key.prefix == 0 && key.suffix == 0 {
		return nil, ec.Start
	}
	bb.B = key.Marshal(bb.B[:0])
	compressedResultBuf := resultBufPool.Get()
	defer resultBufPool.Put(compressedResultBuf)
	compressedResultBuf.B = rrc.c.GetBig(compressedResultBuf.B[:0], bb.B)
	if len(compressedResultBuf.B) == 0 {
		mi.RemoveKey(key)
		metainfoBuf = mi.Marshal(metainfoBuf[:0])
		bb.B = marshalRollupResultCacheKey(bb.B[:0], funcName, me, iafc, window, ec.Step)
		rrc.c.Set(bb.B, metainfoBuf)
		return nil, ec.Start
	}
	// Decompress into newly allocated byte slice, since tss returned from unmarshalTimeseriesFast
	// refers to the byte slice, so it cannot be returned to the resultBufPool.
	resultBuf, err := encoding.DecompressZSTD(nil, compressedResultBuf.B)
	if err != nil {
		logger.Panicf("BUG: cannot decompress resultBuf from rollupResultCache: %s; it looks like it was improperly saved", err)
	}
	tss, err = unmarshalTimeseriesFast(resultBuf)
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal timeseries from rollupResultCache: %s; it looks like it was improperly saved", err)
	}

	// Extract values for the matching timestamps
	timestamps := tss[0].Timestamps
	i := 0
	for i < len(timestamps) && timestamps[i] < ec.Start {
		i++
	}
	if i == len(timestamps) {
		// no matches.
		return nil, ec.Start
	}
	if timestamps[i] != ec.Start {
		// The cached range doesn't cover the requested range.
		return nil, ec.Start
	}

	j := len(timestamps) - 1
	for j >= 0 && timestamps[j] > ec.End {
		j--
	}
	j++
	if j <= i {
		// no matches.
		return nil, ec.Start
	}

	for _, ts := range tss {
		ts.Timestamps = ts.Timestamps[i:j]
		ts.Values = ts.Values[i:j]
	}

	timestamps = tss[0].Timestamps
	newStart = timestamps[len(timestamps)-1] + ec.Step
	return tss, newStart
}

var resultBufPool bytesutil.ByteBufferPool

func (rrc *rollupResultCache) Put(funcName string, ec *EvalConfig, me *metricExpr, iafc *incrementalAggrFuncContext, window int64, tss []*timeseries) {
	if *disableCache || len(tss) == 0 || !ec.mayCache() {
		return
	}

	// Remove values up to currentTime - step - maxSilenceInterval,
	// since these values may be added later.
	timestamps := tss[0].Timestamps
	deadline := (time.Now().UnixNano() / 1e6) - ec.Step - maxSilenceInterval
	i := len(timestamps) - 1
	for i >= 0 && timestamps[i] > deadline {
		i--
	}
	i++
	if i == 0 {
		// Nothing to store in the cache.
		return
	}
	if i < len(timestamps) {
		timestamps = timestamps[:i]
		// Make a copy of tss and remove unfit values
		rvs := copyTimeseriesShallow(tss)
		for _, ts := range rvs {
			ts.Timestamps = ts.Timestamps[:i]
			ts.Values = ts.Values[:i]
		}
		tss = rvs
	}

	// Store tss in the cache.
	maxMarshaledSize := getRollupResultCacheSize() / 4
	resultBuf := resultBufPool.Get()
	defer resultBufPool.Put(resultBuf)
	resultBuf.B = marshalTimeseriesFast(resultBuf.B[:0], tss, maxMarshaledSize, ec.Step)
	if len(resultBuf.B) == 0 {
		tooBigRollupResults.Inc()
		return
	}
	compressedResultBuf := resultBufPool.Get()
	defer resultBufPool.Put(compressedResultBuf)
	compressedResultBuf.B = encoding.CompressZSTDLevel(compressedResultBuf.B[:0], resultBuf.B, 1)

	bb := bbPool.Get()
	defer bbPool.Put(bb)

	var key rollupResultCacheKey
	key.prefix = rollupResultCacheKeyPrefix
	key.suffix = atomic.AddUint64(&rollupResultCacheKeySuffix, 1)
	bb.B = key.Marshal(bb.B[:0])
	rrc.c.SetBig(bb.B, compressedResultBuf.B)

	bb.B = marshalRollupResultCacheKey(bb.B[:0], funcName, me, iafc, window, ec.Step)
	metainfoBuf := rrc.c.Get(nil, bb.B)
	var mi rollupResultCacheMetainfo
	if len(metainfoBuf) > 0 {
		if err := mi.Unmarshal(metainfoBuf); err != nil {
			logger.Panicf("BUG: cannot unmarshal rollupResultCacheMetainfo: %s; it looks like it was improperly saved", err)
		}
	}
	mi.AddKey(key, timestamps[0], timestamps[len(timestamps)-1])
	metainfoBuf = mi.Marshal(metainfoBuf[:0])
	rrc.c.Set(bb.B, metainfoBuf)
}

var (
	rollupResultCacheKeyPrefix = func() uint64 {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			// do not use logger.Panicf, since it isn't initialized yet.
			panic(fmt.Errorf("FATAL: cannot read random data for rollupResultCacheKeyPrefix: %s", err))
		}
		return encoding.UnmarshalUint64(buf[:])
	}()
	rollupResultCacheKeySuffix = uint64(time.Now().UnixNano())
)

var tooBigRollupResults = metrics.NewCounter("vm_too_big_rollup_results_total")

// Increment this value every time the format of the cache changes.
const rollupResultCacheVersion = 6

func marshalRollupResultCacheKey(dst []byte, funcName string, me *metricExpr, iafc *incrementalAggrFuncContext, window, step int64) []byte {
	dst = append(dst, rollupResultCacheVersion)
	if iafc == nil {
		dst = append(dst, 0)
	} else {
		dst = append(dst, 1)
		dst = iafc.ae.AppendString(dst)
	}
	dst = encoding.MarshalUint64(dst, uint64(len(funcName)))
	dst = append(dst, funcName...)
	dst = encoding.MarshalInt64(dst, window)
	dst = encoding.MarshalInt64(dst, step)
	for i := range me.TagFilters {
		dst = me.TagFilters[i].Marshal(dst)
	}
	return dst
}

// mergeTimeseries concatenates b with a and returns the result.
//
// Preconditions:
// - a mustn't intersect with b.
// - a timestamps must be smaller than b timestamps.
//
// Postconditions:
// - a and b cannot be used after returning from the call.
func mergeTimeseries(a, b []*timeseries, bStart int64, ec *EvalConfig) []*timeseries {
	sharedTimestamps := ec.getSharedTimestamps()
	if bStart == ec.Start {
		// Nothing to merge - b covers all the time range.
		// Verify b is correct.
		for _, tsB := range b {
			tsB.denyReuse = true
			tsB.Timestamps = sharedTimestamps
			if len(tsB.Values) != len(tsB.Timestamps) {
				logger.Panicf("BUG: unexpected number of values in b; got %d; want %d", len(tsB.Values), len(tsB.Timestamps))
			}
		}
		return b
	}

	m := make(map[string]*timeseries, len(a))
	bb := bbPool.Get()
	defer bbPool.Put(bb)
	for _, ts := range a {
		bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
		m[string(bb.B)] = ts
	}

	rvs := make([]*timeseries, 0, len(a))
	for _, tsB := range b {
		var tmp timeseries
		tmp.denyReuse = true
		tmp.Timestamps = sharedTimestamps
		tmp.Values = make([]float64, 0, len(tmp.Timestamps))
		// Do not use MetricName.CopyFrom for performance reasons.
		// It is safe to make shallow copy, since tsB must no longer used.
		tmp.MetricName = tsB.MetricName

		bb.B = marshalMetricNameSorted(bb.B[:0], &tsB.MetricName)
		tsA := m[string(bb.B)]
		if tsA == nil {
			tStart := ec.Start
			for tStart < bStart {
				tmp.Values = append(tmp.Values, nan)
				tStart += ec.Step
			}
		} else {
			tmp.Values = append(tmp.Values, tsA.Values...)
			delete(m, string(bb.B))
		}
		tmp.Values = append(tmp.Values, tsB.Values...)
		if len(tmp.Values) != len(tmp.Timestamps) {
			logger.Panicf("BUG: unexpected values after merging new values; got %d; want %d", len(tmp.Values), len(tmp.Timestamps))
		}
		rvs = append(rvs, &tmp)
	}

	// Copy the remaining timeseries from m.
	for _, tsA := range m {
		var tmp timeseries
		tmp.denyReuse = true
		tmp.Timestamps = sharedTimestamps
		// Do not use MetricName.CopyFrom for performance reasons.
		// It is safe to make shallow copy, since tsA must no longer used.
		tmp.MetricName = tsA.MetricName
		tmp.Values = append(tmp.Values, tsA.Values...)

		tStart := bStart
		for tStart <= ec.End {
			tmp.Values = append(tmp.Values, nan)
			tStart += ec.Step
		}
		if len(tmp.Values) != len(tmp.Timestamps) {
			logger.Panicf("BUG: unexpected values in the result after adding cached values; got %d; want %d", len(tmp.Values), len(tmp.Timestamps))
		}
		rvs = append(rvs, &tmp)
	}
	return rvs
}

type rollupResultCacheMetainfo struct {
	entries []rollupResultCacheMetainfoEntry
}

func (mi *rollupResultCacheMetainfo) Marshal(dst []byte) []byte {
	dst = encoding.MarshalUint32(dst, uint32(len(mi.entries)))
	for i := range mi.entries {
		dst = mi.entries[i].Marshal(dst)
	}
	return dst
}

func (mi *rollupResultCacheMetainfo) Unmarshal(src []byte) error {
	if len(src) < 4 {
		return fmt.Errorf("cannot unmarshal len(etries) from %d bytes; need at least %d bytes", len(src), 4)
	}
	entriesLen := int(encoding.UnmarshalUint32(src))
	src = src[4:]
	if n := entriesLen - cap(mi.entries); n > 0 {
		mi.entries = append(mi.entries[:cap(mi.entries)], make([]rollupResultCacheMetainfoEntry, n)...)
	}
	mi.entries = mi.entries[:entriesLen]
	for i := 0; i < entriesLen; i++ {
		tail, err := mi.entries[i].Unmarshal(src)
		if err != nil {
			return fmt.Errorf("cannot unmarshal entry #%d: %s", i, err)
		}
		src = tail
	}
	if len(src) > 0 {
		return fmt.Errorf("unexpected non-empty tail left; len(tail)=%d", len(src))
	}
	return nil
}

func (mi *rollupResultCacheMetainfo) GetBestKey(start, end int64) rollupResultCacheKey {
	if start > end {
		logger.Panicf("BUG: start cannot exceed end; got %d vs %d", start, end)
	}
	var bestKey rollupResultCacheKey
	bestD := int64(1<<63 - 1)
	for i := range mi.entries {
		e := &mi.entries[i]
		if start < e.start || end <= e.start {
			continue
		}
		d := start - e.start
		if d < bestD {
			bestD = d
			bestKey = e.key
		}
	}
	return bestKey
}

func (mi *rollupResultCacheMetainfo) AddKey(key rollupResultCacheKey, start, end int64) {
	if start > end {
		logger.Panicf("BUG: start cannot exceed end; got %d vs %d", start, end)
	}
	mi.entries = append(mi.entries, rollupResultCacheMetainfoEntry{
		start: start,
		end:   end,
		key:   key,
	})
	if len(mi.entries) > 30 {
		// Remove old entries.
		mi.entries = append(mi.entries[:0], mi.entries[10:]...)
	}
}

func (mi *rollupResultCacheMetainfo) RemoveKey(key rollupResultCacheKey) {
	for i := range mi.entries {
		if mi.entries[i].key == key {
			mi.entries = append(mi.entries[:i], mi.entries[i+1:]...)
			return
		}
	}
}

type rollupResultCacheMetainfoEntry struct {
	start int64
	end   int64
	key   rollupResultCacheKey
}

func (mie *rollupResultCacheMetainfoEntry) Marshal(dst []byte) []byte {
	dst = encoding.MarshalInt64(dst, mie.start)
	dst = encoding.MarshalInt64(dst, mie.end)
	dst = encoding.MarshalUint64(dst, mie.key.prefix)
	dst = encoding.MarshalUint64(dst, mie.key.suffix)
	return dst
}

func (mie *rollupResultCacheMetainfoEntry) Unmarshal(src []byte) ([]byte, error) {
	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal start from %d bytes; need at least %d bytes", len(src), 8)
	}
	mie.start = encoding.UnmarshalInt64(src)
	src = src[8:]

	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal end from %d bytes; need at least %d bytes", len(src), 8)
	}
	mie.end = encoding.UnmarshalInt64(src)
	src = src[8:]

	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal key prefix from %d bytes; need at least %d bytes", len(src), 8)
	}
	mie.key.prefix = encoding.UnmarshalUint64(src)
	src = src[8:]

	if len(src) < 8 {
		return src, fmt.Errorf("cannot unmarshal key suffix from %d bytes; need at least %d bytes", len(src), 8)
	}
	mie.key.suffix = encoding.UnmarshalUint64(src)
	src = src[8:]

	return src, nil
}

// rollupResultCacheKey must be globally unique across vmselect nodes,
// so it has prefix and suffix.
type rollupResultCacheKey struct {
	prefix uint64
	suffix uint64
}

func (k *rollupResultCacheKey) Marshal(dst []byte) []byte {
	dst = append(dst, rollupResultCacheVersion)
	dst = encoding.MarshalUint64(dst, k.prefix)
	dst = encoding.MarshalUint64(dst, k.suffix)
	return dst
}
