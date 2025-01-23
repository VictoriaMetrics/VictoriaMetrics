package metricnamestats

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Tracker implements in-memory tracker for timeseries metric names
// it tracks ingest and query requests for metric names
// and collects statistic
//
// main purpose of this tracker is to provide insights about never queired metrics
type Tracker struct {
	creationTs        atomic.Uint64
	maxSizeBytes      uint64
	currentSizeBytes  atomic.Uint64
	currentItemsCount atomic.Uint64
	store             *sync.Map
	cachePath         string

	// protects Reset call
	mu sync.Mutex
	// helper for tests
	getCurrentTs func() uint64
}

type statItem struct {
	requestsCount atomic.Uint64
	lastRequestTs atomic.Uint64
}

// MustLoadFrom inits tracker from the given on-disk path
func MustLoadFrom(loadPath string, maxSizeBytes uint64) *Tracker {
	t, err := loadFrom(loadPath, maxSizeBytes)
	if err != nil {
		logger.Fatalf("unexpected error at tracker state load from path=%q: %s", loadPath, err)
	}
	return t
}
func loadFrom(loadPath string, maxSizeBytes uint64) (*Tracker, error) {
	ut := &Tracker{
		maxSizeBytes: maxSizeBytes,
		store:        &sync.Map{},
		cachePath:    loadPath,
		getCurrentTs: fasttime.UnixTimestamp,
	}
	ut.creationTs.Store(fasttime.UnixTimestamp())
	fin, err := os.Open(loadPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("cannot read file content: %w", err)
	}
	defer fin.Close()
	if fin != nil {
		reader := json.NewDecoder(fin)
		var creationTs uint64
		if err := reader.Decode(&creationTs); err != nil {
			if errors.Is(err, io.EOF) {
				return ut, nil
			}
			return nil, fmt.Errorf("cannot parse creation timestamp: %w", err)
		}
		ut.creationTs.Store(creationTs)
		var r StatRecord
		var cnt uint64
		var size uint64
		for {
			if err := reader.Decode(&r); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("cannot parse state record: %w", err)
			}
			var mi statItem
			mi.requestsCount.Store(r.RequestCount)
			mi.lastRequestTs.Store(r.LastRequestTs)
			key := r.MetricName
			ut.store.Store(key, &mi)

			size += uint64(len(key))
			cnt++

			r.LastRequestTs = 0
			r.RequestCount = 0
			r.MetricName = ""
		}
		ut.currentSizeBytes.Store(size)
		ut.currentItemsCount.Store(cnt)
		logger.Infof("loaded state from disk, records: %d, total size: %d", cnt, size)
	}
	return ut, nil
}

// MustClose closes tracker
func (mt *Tracker) MustClose() {
	if err := mt.save(); err != nil {
		logger.Panicf("cannot save tracker state at path=%q: %s", mt.cachePath, err)
	}
	mt.store = nil
}

// save stores in-memory state of tracker on disk
func (mt *Tracker) save() error {
	fout, err := os.OpenFile(mt.cachePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("cannot open file for state save: %w", err)
	}
	defer fout.Close()
	writer := json.NewEncoder(fout)
	if err := writer.Encode(mt.creationTs.Load()); err != nil {
		return fmt.Errorf("cannot save encoded creation timestamp: %w", err)
	}
	var firstErr error
	mt.store.Range(func(key, value any) bool {
		mi := value.(*statItem)
		r := StatRecord{
			MetricName:    key.(string),
			RequestCount:  mi.requestsCount.Load(),
			LastRequestTs: mi.lastRequestTs.Load(),
		}
		if err := writer.Encode(r); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("cannot save encoded state record: %w", err)
			}
			return false
		}
		return true
	})
	if firstErr != nil {
		return err
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
	dst.CurrentSizeBytes = mt.currentSizeBytes.Load()
	dst.CurrentItemsCount = mt.currentItemsCount.Load()
	dst.MaxSizeBytes = mt.maxSizeBytes
}

// IsEmpty checks if internal state has any records
func (mt *Tracker) IsEmpty() bool {
	return mt.currentItemsCount.Load() > 0
}

// Reset cleans stats and saves cache state
func (mt *Tracker) Reset() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.store.Clear()
	mt.currentSizeBytes.Store(0)
	mt.currentItemsCount.Store(0)
	mt.creationTs.Store(mt.getCurrentTs())
	if err := mt.save(); err != nil {
		logger.Panicf("during Tracker reset cannot save state: %s", err)
	}
}

// RegisterIngestRequest tracks metric name ingestion
func (mt *Tracker) RegisterIngestRequest(metricGroup []byte) {
	_, ok := mt.store.Load(string(metricGroup))
	if !ok {
		if mt.shouldSkipNewItemAdd() {
			return
		}
		var mi statItem
		mt.store.Store(string(metricGroup), &mi)
		mt.currentSizeBytes.Add(uint64(len(metricGroup)))
		mt.currentItemsCount.Add(1)
	}
}

// RegisterQueryRequest tracks metric name at query
//
// query requests tracking is approximate
// it may lose newly registered metric during concurrent ingestion
func (mt *Tracker) RegisterQueryRequest(metricGroup []byte) {
	v, ok := mt.store.Load(string(metricGroup))
	if !ok {
		if mt.shouldSkipNewItemAdd() {
			return
		}
		var mi statItem
		mi.requestsCount.Store(1)
		mi.lastRequestTs.Store(mt.getCurrentTs())
		mt.store.Store(string(metricGroup), &mi)
		mt.currentSizeBytes.Add(uint64(len(metricGroup)))
		mt.currentItemsCount.Add(1)
		return
	}
	vi := v.(*statItem)
	vi.requestsCount.Add(1)
	vi.lastRequestTs.Store(mt.getCurrentTs())
}

func (mt *Tracker) shouldSkipNewItemAdd() bool {
	return mt.currentSizeBytes.Load() > mt.maxSizeBytes
}

// GetStats returns stats response for the tracked metrics
func (mt *Tracker) GetStats(limit int, lte uint64) StatsResult {
	var result StatsResult
	result.CollectedSinceTs = mt.creationTs.Load()
	result.TotalRecords = mt.currentItemsCount.Load()
	mt.store.Range(func(key, value any) bool {
		if len(result.Records) >= limit {
			return false
		}
		mi := value.(*statItem)
		v := mi.requestsCount.Load()
		if lte < 0 || v <= lte {
			result.Records = append(result.Records, StatRecord{
				MetricName:    key.(string),
				RequestCount:  v,
				LastRequestTs: mi.lastRequestTs.Load(),
			})
		}
		return true
	})
	result.sort()
	return result
}

// StatsResult defines stats result for GetStats request
type StatsResult struct {
	CollectedSinceTs uint64
	TotalRecords     uint64
	Records          []StatRecord
}

func (sr *StatsResult) sort() {
	sort.Slice(sr.Records, func(i, j int) bool {
		if sr.Records[i].RequestCount == sr.Records[j].RequestCount {
			return sr.Records[i].MetricName < sr.Records[j].MetricName
		}
		return sr.Records[i].RequestCount < sr.Records[j].RequestCount
	})
}

// StatRecord defines stat record for given metric name
type StatRecord struct {
	MetricName    string
	RequestCount  uint64
	LastRequestTs uint64
}
