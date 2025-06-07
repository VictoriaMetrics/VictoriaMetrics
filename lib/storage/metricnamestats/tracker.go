package metricnamestats

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	// metricNameBufSize defines size buffer for metric name allocations
	metricNameBufSize = 16 * 1024
	statItemBufSize   = 1024
	// statKey + statItem + approx key-value at map in-memory size
	storeOverhead = 24 + 16 + 24
)

// Tracker implements in-memory tracker for timeseries metric names
// it tracks ingest and query requests for metric names
// and collects statistics
//
// main purpose of this tracker is to provide insights about metrics that have never been queried
type Tracker struct {
	maxSizeBytes uint64
	cachePath    string

	creationTs        atomic.Uint64
	currentSizeBytes  atomic.Uint64
	currentItemsCount atomic.Uint64

	// mu protect fields below
	mu sync.RWMutex

	store map[statKey]*statItem
	// holds batch allocations for statItems at store
	statItemBuf []statItem
	// holds batch allocations for metric names at statKey
	metricNamesBuf []byte

	// helper for tests
	getCurrentTs func() uint64
}

type statKey struct {
	accountID  uint32
	projectID  uint32
	metricName string
}

type statItem struct {
	requestsCount atomic.Uint64
	lastRequestTs atomic.Uint64
}

type recordForStore struct {
	AccountID     uint32
	ProjectID     uint32
	MetricName    string
	RequestsCount uint64
	LastRequestTs uint64
}

// MustLoadFrom inits tracker from the given on-disk path
func MustLoadFrom(loadPath string, maxSizeBytes uint64) *Tracker {
	mt, err := loadFrom(loadPath, maxSizeBytes)
	if err != nil {
		// just log error in case of any error and return empty object as other caches do
		logger.Errorf("metric names stats tracker file at path %s is invalid: %s; init new metric names stats tracker", loadPath, err)
		return newTracker(loadPath, maxSizeBytes)
	}

	return mt
}

func loadFrom(loadPath string, maxSizeBytes uint64) (*Tracker, error) {
	mt := newTracker(loadPath, maxSizeBytes)

	f, err := os.Open(loadPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("cannot access file content: %w", err)
	}
	// fast path
	if f == nil {
		return mt, nil
	}

	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("cannot create new gzip reader: %w", err)
	}
	reader := json.NewDecoder(zr)
	var storedMaxSizeBytes uint64
	if err := reader.Decode(&storedMaxSizeBytes); err != nil {
		if errors.Is(err, io.EOF) {
			return mt, nil
		}
		return nil, fmt.Errorf("cannot parse maxSizeBytes: %w", err)
	}
	if storedMaxSizeBytes > maxSizeBytes {
		logger.Infof("Resetting tracker state due to changed maxSizeBytes from %d to %d.", storedMaxSizeBytes, maxSizeBytes)
		return mt, nil
	}
	var creationTs uint64
	if err := reader.Decode(&creationTs); err != nil {
		return nil, fmt.Errorf("cannot parse creation timestamp: %w", err)
	}
	mt.creationTs.Store(creationTs)
	var cnt uint64
	var size uint64
	var r recordForStore
	for {
		if err := reader.Decode(&r); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("cannot parse state record: %w", err)
		}
		// during cache load, there is no need to hold lock
		si := mt.nextRecordLocked()

		si.lastRequestTs.Store(r.LastRequestTs)
		si.requestsCount.Store(r.RequestsCount)

		key := statKey{
			projectID:  r.ProjectID,
			accountID:  r.AccountID,
			metricName: mt.cloneMetricNameLocked([]byte(r.MetricName)),
		}
		mt.store[key] = si
		size += uint64(len(r.MetricName)) + storeOverhead
		cnt++
	}
	if err := zr.Close(); err != nil {
		return nil, fmt.Errorf("cannot close gzip reader: %w", err)
	}

	mt.currentSizeBytes.Store(size)
	mt.currentItemsCount.Store(cnt)
	logger.Infof("loaded state from disk, records: %d, total size: %d", cnt, size)
	return mt, nil
}

func (mt *Tracker) nextRecordLocked() *statItem {
	n := len(mt.statItemBuf) + 1
	if n > cap(mt.statItemBuf) {
		// allocate a new slice instead of reallocating exist
		// it saves memory and reduces GC pressure
		mt.statItemBuf = make([]statItem, 0, statItemBufSize)
		n = 1
	}
	mt.statItemBuf = mt.statItemBuf[:n]
	st := &mt.statItemBuf[n-1]

	return st
}

// cloneMetricNameLocked uses the same idea as strings.Clone.
// But instead of direct []byte allocation for each cloned string,
// it allocates metricNamesBuf, copies provide metricGroup into it
// and uses string *byte references for it via subslice.
func (mt *Tracker) cloneMetricNameLocked(metricName []byte) string {
	if len(metricName) > metricNameBufSize {
		// metricName is too large for default buffer
		// directly allocate it on heap as strings.Clone does
		b := make([]byte, len(metricName))
		copy(b, metricName)
		return bytesutil.ToUnsafeString(b)
	}
	idx := len(mt.metricNamesBuf)
	n := len(metricName) + len(mt.metricNamesBuf)
	if n > cap(mt.metricNamesBuf) {
		// allocate a new slice instead of reallocting exist
		// it saves memory and reduces GC pressure
		mt.metricNamesBuf = make([]byte, 0, metricNameBufSize)
		idx = 0
	}
	mt.metricNamesBuf = append(mt.metricNamesBuf, metricName...)
	return bytesutil.ToUnsafeString(mt.metricNamesBuf[idx:])
}

// MustClose closes tracker and saves state on disk
func (mt *Tracker) MustClose() {
	if mt == nil {
		return
	}
	if err := mt.saveLocked(); err != nil {
		logger.Panicf("cannot save tracker state at path=%q: %s", mt.cachePath, err)
	}
}

// saveLocked stores in-memory state of tracker on disk
func (mt *Tracker) saveLocked() error {
	// Create dir if it doesn't exist in the same manner as other caches doing
	dir, fileName := filepath.Split(mt.cachePath)
	if _, err := os.Stat(dir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot stat %q: %s", dir, err)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create dir %q: %s", dir, err)
		}
	}

	// create temp directory in the same directory where original file located
	// it's needed to mitigate cross block-device rename error.
	tempDir, err := os.MkdirTemp(dir, "metricnamestats.tmp.")
	if err != nil {
		return fmt.Errorf("cannot create tempDir for state save: %w", err)
	}
	defer func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}()

	f, err := os.Create(filepath.Join(tempDir, fileName))
	if err != nil {
		return fmt.Errorf("cannot open file for state save: %w", err)
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	writer := json.NewEncoder(zw)
	if err := writer.Encode(mt.maxSizeBytes); err != nil {
		return fmt.Errorf("cannot save encoded maxSizeBytes: %w", err)
	}
	if err := writer.Encode(mt.creationTs.Load()); err != nil {
		return fmt.Errorf("cannot save encoded creation timestamp: %w", err)
	}

	var r recordForStore
	for sk, si := range mt.store {
		r.AccountID = sk.accountID
		r.ProjectID = sk.projectID
		r.MetricName = sk.metricName
		r.LastRequestTs = si.lastRequestTs.Load()
		r.RequestsCount = si.requestsCount.Load()
		if err := writer.Encode(r); err != nil {
			return fmt.Errorf("cannot save encoded state record: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("cannot flush writer state: %w", err)
	}
	// atomically save result
	if err := os.Rename(f.Name(), mt.cachePath); err != nil {
		return fmt.Errorf("cannot move temporary file %q to %q: %s", f.Name(), mt.cachePath, err)
	}
	return nil
}

// TrackerMetrics holds metrics to report
type TrackerMetrics struct {
	CurrentSizeBytes  uint64
	CurrentItemsCount uint64
	MaxSizeBytes      uint64
}

// UpdateMetrics writes internal metrics to the provided object
func (mt *Tracker) UpdateMetrics(dst *TrackerMetrics) {
	if mt == nil {
		return
	}
	dst.CurrentSizeBytes = mt.currentSizeBytes.Load()
	dst.CurrentItemsCount = mt.currentItemsCount.Load()
	dst.MaxSizeBytes = mt.maxSizeBytes
}

// IsEmpty checks if internal state has any records
func (mt *Tracker) IsEmpty() bool {
	return mt.currentItemsCount.Load() == 0
}

// Reset cleans stats, saves cache state and executes provided func
func (mt *Tracker) Reset(onReset func()) {
	if mt == nil {
		return
	}
	logger.Infof("resetting metric names tracker state")
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.initEmpty()
	if err := mt.saveLocked(); err != nil {
		logger.Panicf("during Tracker reset cannot save state: %s", err)
	}
	onReset()
}

func (mt *Tracker) initEmpty() {
	mt.store = make(map[statKey]*statItem)
	mt.metricNamesBuf = make([]byte, 0, metricNameBufSize)
	mt.statItemBuf = make([]statItem, 0, statItemBufSize)
	mt.currentSizeBytes.Store(0)
	mt.currentItemsCount.Store(0)
	mt.creationTs.Store(mt.getCurrentTs())
}

func newTracker(loadPath string, maxSizeBytes uint64) *Tracker {
	mt := &Tracker{
		maxSizeBytes: maxSizeBytes,
		cachePath:    loadPath,
		getCurrentTs: fasttime.UnixTimestamp,
	}
	mt.initEmpty()
	return mt
}

// RegisterIngestRequest tracks metric name ingestion
func (mt *Tracker) RegisterIngestRequest(accountID, projectID uint32, metricName []byte) {
	if mt == nil {
		return
	}
	if mt.cacheIsFull() {
		return
	}

	sk := statKey{
		accountID:  accountID,
		projectID:  projectID,
		metricName: bytesutil.ToUnsafeString(metricName),
	}
	mt.mu.RLock()
	_, ok := mt.store[sk]
	mt.mu.RUnlock()
	if ok {
		return
	}

	mt.mu.Lock()
	// key could be already ingested concurrently
	_, ok = mt.store[sk]
	if ok {
		mt.mu.Unlock()
		return
	}
	si := mt.nextRecordLocked()
	sk.metricName = mt.cloneMetricNameLocked(metricName)
	mt.store[sk] = si
	mt.mu.Unlock()

	mt.currentSizeBytes.Add(uint64(len(metricName)) + storeOverhead)
	mt.currentItemsCount.Add(1)
}

// RegisterQueryRequest tracks metric name at query request
func (mt *Tracker) RegisterQueryRequest(accountID, projectID uint32, metricName []byte) {
	if mt == nil {
		return
	}
	mt.mu.RLock()
	key := statKey{
		accountID:  accountID,
		projectID:  projectID,
		metricName: bytesutil.ToUnsafeString(metricName),
	}
	si, ok := mt.store[key]
	mt.mu.RUnlock()
	if !ok {
		return
	}
	si.lastRequestTs.Store(mt.getCurrentTs())
	si.requestsCount.Add(1)
}

func (mt *Tracker) cacheIsFull() bool {
	return mt.currentSizeBytes.Load() > mt.maxSizeBytes
}

// GetStatsForTenant returns stats response for the tracked metrics for given tenant
func (mt *Tracker) GetStatsForTenant(accountID, projectID uint32, limit, le int, matchPattern string) StatsResult {
	var result StatsResult
	if mt == nil {
		return result
	}
	var matchRe *regexp.Regexp
	if len(matchPattern) > 0 {
		var err error
		matchRe, err = regexp.Compile(matchPattern)
		if err != nil {
			logger.Fatalf("BUG: expected valid regex=%q: %s", matchPattern, err)
		}
	}
	mt.mu.RLock()

	result = mt.getStatsLocked(limit, func(sk *statKey, si *statItem) bool {
		if sk.accountID != accountID || sk.projectID != projectID {
			return false
		}
		if le >= 0 && int(si.requestsCount.Load()) > le {
			return false
		}
		if matchRe != nil && !matchRe.MatchString(sk.metricName) {
			return false
		}
		return true
	})
	mt.mu.RUnlock()

	result.sort()
	return result
}

// GetStatRecordsForNames returns stats records for the given metric names and tenant
func (mt *Tracker) GetStatRecordsForNames(accountID, projectID uint32, metricNames []string) []StatRecord {
	if mt == nil {
		return nil
	}
	mt.mu.RLock()
	records := make([]StatRecord, 0, len(metricNames))
	for _, mn := range metricNames {
		sk := statKey{
			accountID:  accountID,
			projectID:  projectID,
			metricName: mn,
		}
		si, ok := mt.store[sk]
		if !ok {
			continue
		}
		records = append(records, StatRecord{
			MetricName:    mn,
			RequestsCount: si.requestsCount.Load(),
			LastRequestTs: si.lastRequestTs.Load(),
		})
	}
	mt.mu.RUnlock()
	return records
}

// GetStats returns stats response for the tracked metrics
//
// DeduplicateMergeRecords must be called at cluster version on returned result.
func (mt *Tracker) GetStats(limit, le int, matchPattern string) StatsResult {
	var result StatsResult
	if mt == nil {
		return result
	}
	mt.mu.RLock()
	var matchRe *regexp.Regexp
	if len(matchPattern) > 0 {
		var err error
		matchRe, err = regexp.Compile(matchPattern)
		if err != nil {
			logger.Fatalf("BUG: expected valid regex=%q: %s", matchPattern, err)
		}
	}
	result = mt.getStatsLocked(limit, func(sk *statKey, si *statItem) bool {
		if le >= 0 && int(si.requestsCount.Load()) > le {
			return false
		}
		if matchRe != nil && !matchRe.MatchString(sk.metricName) {
			return false
		}
		return true
	})
	mt.mu.RUnlock()

	result.sort()
	return result
}

func (mt *Tracker) getStatsLocked(limit int, predicate func(sk *statKey, si *statItem) bool) StatsResult {
	var result StatsResult

	result.CollectedSinceTs = mt.creationTs.Load()
	result.TotalRecords = mt.currentItemsCount.Load()
	result.MaxSizeBytes = mt.maxSizeBytes
	result.CurrentSizeBytes = mt.currentSizeBytes.Load()

	for sk, si := range mt.store {
		if len(result.Records) >= limit {
			return result
		}
		if predicate(&sk, si) {
			result.Records = append(result.Records, StatRecord{
				MetricName:    sk.metricName,
				RequestsCount: si.requestsCount.Load(),
				LastRequestTs: si.lastRequestTs.Load(),
			})
		}
	}
	return result
}

// StatsResult defines stats result for GetStats request
type StatsResult struct {
	CollectedSinceTs uint64
	TotalRecords     uint64
	MaxSizeBytes     uint64
	CurrentSizeBytes uint64
	Records          []StatRecord
}

// StatRecord defines stat record for given metric name
type StatRecord struct {
	MetricName    string
	RequestsCount uint64
	LastRequestTs uint64
}

func (sr *StatsResult) sort() {
	sort.Slice(sr.Records, func(i, j int) bool {
		return sr.Records[i].MetricName < sr.Records[j].MetricName
	})
}

// DeduplicateMergeRecords performs merging duplicate records by metric name
//
// It is usual case for global tenant request at cluster version.
func (sr *StatsResult) DeduplicateMergeRecords() {
	if len(sr.Records) < 2 {
		return
	}
	tmp := sr.Records[:0]
	// deduplication uses sliding indexes
	//
	// records:
	// [ 0    1    2    3    4    5    6   ]
	//
	// [ mn1, mn2, mn2, mn2, mn3, mn4, mn4 ]
	//
	//	0     1
	//	0          2
	//	           2    3
	//	           2         4
	//	           2              5
	//	                          5    6
	//
	// result:
	//
	//	[0,1,4,5]

	i := 0
	j := 1
	rCurr := sr.Records[i]
	rNext := sr.Records[j]
	for {
		if rCurr.MetricName == rNext.MetricName {
			rCurr.RequestsCount += rNext.RequestsCount
			if rCurr.LastRequestTs < rNext.LastRequestTs {
				rCurr.LastRequestTs = rNext.LastRequestTs
			}
			j++
			if j >= len(sr.Records) {
				tmp = append(tmp, rCurr)
				break
			}
		} else {
			tmp = append(tmp, rCurr)
			i = j
			rCurr = sr.Records[i]
			j++
			if j >= len(sr.Records) {
				tmp = append(tmp, rNext)
				break
			}
		}
		rNext = sr.Records[j]
	}
	sr.Records = tmp
}

// Sort sorts records by metric name and requests count
func (sr *StatsResult) Sort() {
	sort.Slice(sr.Records, func(i, j int) bool {
		if sr.Records[i].RequestsCount == sr.Records[j].RequestsCount {
			return sr.Records[i].MetricName < sr.Records[j].MetricName
		}
		return sr.Records[i].RequestsCount < sr.Records[j].RequestsCount
	})
}

// Merge adds records from given src
//
// It expected src to be sorted by metricName
func (sr *StatsResult) Merge(src *StatsResult) {
	if sr.CollectedSinceTs < src.CollectedSinceTs {
		sr.CollectedSinceTs = src.CollectedSinceTs
	}
	sr.TotalRecords += src.TotalRecords
	sr.CurrentSizeBytes += src.CurrentSizeBytes
	sr.MaxSizeBytes += src.MaxSizeBytes

	if len(src.Records) == 0 {
		return
	}
	if len(sr.Records) == 0 {
		sr.Records = append(sr.Records, src.Records...)
		return
	}
	// merge sorted elements into new slice
	// records:
	// [ mn1, mn2, mn3, mn4, mn6 ]
	// [ mn2, mn4, mn5 ]
	//   0
	//   0
	// [ ]
	//        1
	//   0
	// [ mn1 ]
	//             2
	//        1
	// [ mn1, mn2 ]
	//                  3
	//        1
	// [ mn1, mn2, mn3 ]
	//                       4
	//             2
	// [ mn1, mn2, mn3, mn4 ]
	//                       4
	//                 -
	// [ mn1, mn2, mn3, mn4, mn5 ]
	//
	// [ mn1, mn2, mn3, mn4, mn5, mn6 ]
	i := 0
	j := 0
	// TODO: probably, we can append src records to sr instead of allocating new slice
	// it will require to perform sort on sr and probably will use more CPU, but less memory
	result := make([]StatRecord, 0, len(sr.Records))
	for {
		if i >= len(sr.Records) {
			result = append(result, src.Records[j:]...)
			break
		}
		if j >= len(src.Records) {
			result = append(result, sr.Records[i:]...)
			break
		}
		left, right := sr.Records[i], src.Records[j]
		switch {
		case left.MetricName == right.MetricName:
			left.RequestsCount += right.RequestsCount
			if left.LastRequestTs < right.LastRequestTs {
				left.LastRequestTs = right.LastRequestTs
			}
			result = append(result, left)
			i++
			j++
		case left.MetricName < right.MetricName:
			result = append(result, left)
			i++
		case left.MetricName > right.MetricName:
			result = append(result, right)
			j++
		}
	}
	sr.Records = result
}
