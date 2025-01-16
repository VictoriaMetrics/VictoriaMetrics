package metricnamestats

import (
	"encoding/json"
	"errors"
	"io"
	"os"
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
	creationTs       uint64
	maxSizeBytes     uint64
	currentSizeBytes uint64
	store            *sync.Map
	cachePath        string

	// helper for tests
	getCurrentTs func() uint64
}

type statItem struct {
	requestsCount uint64
	lastRequestTs uint64
}

// MustLoadFrom inits tracker from the given on-disk path
func MustLoadFrom(loadPath string) *Tracker {
	ut := &Tracker{
		creationTs:   fasttime.UnixTimestamp(),
		store:        &sync.Map{},
		cachePath:    loadPath,
		getCurrentTs: fasttime.UnixTimestamp,
	}
	fin, err := os.Open(loadPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}
	defer fin.Close()
	if fin != nil {
		reader := json.NewDecoder(fin)
		if err := reader.Decode(&ut.creationTs); err != nil {
			if errors.Is(err, io.EOF) {
				return ut
			}
		}
		var r StatRecord
		var cnt int
		var size uint64
		for {
			if err := reader.Decode(&r); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				panic(err)
			}
			mi := statItem{
				requestsCount: r.RequestCount,
				lastRequestTs: r.LastRequestTs,
			}
			key := r.MetricName
			ut.store.Store(key, &mi)
			size += uint64(len(key))
			cnt++
			r.LastRequestTs = 0
			r.RequestCount = 0
			r.MetricName = ""
		}
		ut.currentSizeBytes = size
		logger.Infof("loaded state from disk, records: %d, total size: %d", cnt, size)
	}
	return ut
}

// Save stores in-memory state of tracker on disk
func (mt *Tracker) Save() error {
	fout, err := os.OpenFile(mt.cachePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		panic(err)
	}
	defer fout.Close()
	writer := json.NewEncoder(fout)
	if err := writer.Encode(atomic.LoadUint64(&mt.creationTs)); err != nil {
		return err
	}
	var firstErr error
	mt.store.Range(func(key, value any) bool {
		mi := value.(*statItem)
		r := StatRecord{
			MetricName:    key.(string),
			RequestCount:  atomic.LoadUint64(&mi.requestsCount),
			LastRequestTs: atomic.LoadUint64(&mi.lastRequestTs),
		}
		if err := writer.Encode(r); err != nil {
			if firstErr == nil {
				firstErr = err
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

// SizeBytes returns approximate size in bytes
func (mt *Tracker) SizeBytes() uint64 {
	return atomic.LoadUint64(&mt.currentSizeBytes)
}

// Reset cleans stats
// TODO: @f41gh7 introduce API
func (mt *Tracker) Reset() {
	mt.store.Clear()
	atomic.StoreUint64(&mt.currentSizeBytes, 0)
	atomic.StoreUint64(&mt.creationTs, mt.getCurrentTs())
}

// RegisterIngestRequest tracks metric name ingestion
func (mt *Tracker) RegisterIngestRequest(metricGroup []byte) {
	_, ok := mt.store.Load(string(metricGroup))
	if !ok {
		var mi statItem
		mt.store.Store(string(metricGroup), &mi)
		atomic.AddUint64(&mt.currentSizeBytes, uint64(len(metricGroup)))
	}
}

// RegisterQueryRequest tracks metric name at query
//
// query requests tracking is approximate
// it may lose newly registered metric during concurrent ingestion
func (mt *Tracker) RegisterQueryRequest(metricGroup []byte) {
	v, ok := mt.store.Load(string(metricGroup))
	if !ok {
		mi := statItem{
			requestsCount: 1,
			lastRequestTs: mt.getCurrentTs(),
		}
		mt.store.Store(string(metricGroup), &mi)
		atomic.AddUint64(&mt.currentSizeBytes, uint64(len(metricGroup)))
		return
	}
	vi := v.(*statItem)
	atomic.AddUint64(&vi.requestsCount, 1)
	atomic.StoreUint64(&vi.lastRequestTs, mt.getCurrentTs())
}

// StatsResult defines stats result for GetStats request
type StatsResult struct {
	CollectedSinceTs int64
	Records          []StatRecord
}

// StatRecord defines stat record for given metric name
type StatRecord struct {
	MetricName    string
	RequestCount  uint64
	LastRequestTs uint64
}

// GetStats returns stats response for the tracked metrics
func (mt *Tracker) GetStats(limit int, lte uint64) StatsResult {
	var result StatsResult
	result.CollectedSinceTs = int64(atomic.LoadUint64(&mt.creationTs))
	mt.store.Range(func(key, value any) bool {
		if len(result.Records) >= limit {
			return false
		}
		mi := value.(*statItem)
		v := atomic.LoadUint64(&mi.requestsCount)
		if v <= lte {
			result.Records = append(result.Records, StatRecord{
				MetricName:    key.(string),
				RequestCount:  v,
				LastRequestTs: atomic.LoadUint64(&mi.lastRequestTs),
			})
		}
		return true
	})
	return result
}
