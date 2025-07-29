package metricsmetadata

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type metadataKey struct {
	accountID        uint32
	projectID        uint32
	metricFamilyName string
}

type metricTimingInfo struct {
	LastIngestionTime    uint64 `json:"lastIngestionTime"`
	AvgIngestionInterval uint64 `json:"avgIngestionInterval"`
	IngestionCount       int64  `json:"ingestionCount"`
}

type recordForStore struct {
	AccountID        uint32
	ProjectID        uint32
	MetricFamilyName string
	Rows             []Row
	TimingInfo       metricTimingInfo
}

type MetadataStoreMetrics struct {
	ItemsTotal         uint64
	ItemsIngestedTotal uint64
	ItemsDeduplicated  uint64
	ItemsDeleted       uint64
	ItemsSizeBytes     uint64
}

type Store struct {
	MetricsMetadataStorage    map[metadataKey][]Row
	MetricTimingInfo          map[metadataKey]metricTimingInfo
	metricMetadataStorageLock sync.RWMutex

	storagePath string

	itemsIngestedTotal uint64
	itemsDeduplicated  uint64
	itemsDeleted       uint64
	itemsCurrentTotal  uint64

	cleanupInterval time.Duration
	cleanupStopCh   chan struct{}
	cleanupWG       sync.WaitGroup
}

func NewStore() *Store {
	s := &Store{
		MetricsMetadataStorage: make(map[metadataKey][]Row),
		MetricTimingInfo:       make(map[metadataKey]metricTimingInfo),
		cleanupInterval:        5 * time.Minute,
		cleanupStopCh:          make(chan struct{}),
	}

	s.cleanupWG.Add(1)
	go s.runCleanupScheduler()

	return s
}

func MustLoadFrom(path string) *Store {
	s := NewStore()
	err := s.LoadFrom(path)
	if err != nil {
		logger.Panicf("cannot load metrics metadata from %q: %s", path, err)
	}
	return s
}

func (s *Store) MustClose() {
	close(s.cleanupStopCh)
	s.cleanupWG.Wait()
	s.metricMetadataStorageLock.Lock()
	s.mustSaveLocked()
	s.metricMetadataStorageLock.Unlock()
}

func (s *Store) Add(rows []Row) error {
	if len(rows) == 0 {
		return nil
	}

	s.metricMetadataStorageLock.Lock()
	defer s.metricMetadataStorageLock.Unlock()

	// Update timing for all metrics that were touched
	now := fasttime.UnixTimestamp()

	for _, mr := range rows {
		key := metadataKey{
			accountID:        mr.AccountID,
			projectID:        mr.ProjectID,
			metricFamilyName: bytesutil.ToUnsafeString(mr.MetricFamilyName),
		}
		s.updateMetricTimingLocked(key, now)

		metadataRows, ok := s.MetricsMetadataStorage[key]
		if !ok {
			s.MetricsMetadataStorage[key] = make([]Row, 0, 1)
			s.MetricsMetadataStorage[key] = append(s.MetricsMetadataStorage[key], mr)
			s.itemsIngestedTotal++
			s.itemsCurrentTotal++
			continue
		}

		found := false
		for _, v := range metadataRows {
			if v.Type == mr.Type && bytes.Equal(mr.Unit, v.Unit) && bytes.Equal(mr.Help, v.Help) {
				found = true
				break
			}
		}

		if found {
			s.itemsDeduplicated++
			continue
		}
		s.MetricsMetadataStorage[key] = append(metadataRows, mr)
		s.itemsIngestedTotal++
		s.itemsCurrentTotal++
	}

	return nil
}

func (s *Store) updateMetricTimingLocked(key metadataKey, now uint64) {
	timing, exists := s.MetricTimingInfo[key]
	if !exists {
		s.MetricTimingInfo[key] = metricTimingInfo{
			LastIngestionTime:    now,
			AvgIngestionInterval: 0,
			IngestionCount:       1,
		}
		return
	}

	timeSinceLastIngestion := now - timing.LastIngestionTime

	// Update running average using exponential moving average
	if timing.IngestionCount == 1 {
		timing.AvgIngestionInterval = timeSinceLastIngestion
	} else {
		alpha := 0.2
		timing.AvgIngestionInterval = uint64(float64(timing.AvgIngestionInterval)*(1-alpha) + float64(timeSinceLastIngestion)*alpha)
	}

	timing.LastIngestionTime = now
	timing.IngestionCount++

	s.MetricTimingInfo[key] = timing
}

func (s *Store) runCleanupScheduler() {
	defer s.cleanupWG.Done()

	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.cleanupStopCh:
			return
		}
	}
}

func (s *Store) cleanup() {
	s.metricMetadataStorageLock.Lock()
	defer s.metricMetadataStorageLock.Unlock()

	now := fasttime.UnixTimestamp()
	var keysToDelete []metadataKey

	for key, timing := range s.MetricTimingInfo {
		if timing.IngestionCount < 2 {
			continue
		}

		// Check if it's been more than 10x the average interval since last ingestion
		// Prometheus keeps metadata for 10 scrapes after the last ingestion
		// https://github.com/prometheus/prometheus/blob/5a5424cbc1422ddfd94651122845fdc4a2e8b5c7/scrape/scrape.go#L1041
		timeSinceLastIngestion := now - timing.LastIngestionTime
		threshold := timing.AvgIngestionInterval * 10

		if timeSinceLastIngestion > threshold {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		if rows, ok := s.MetricsMetadataStorage[key]; ok {
			s.itemsDeleted += uint64(len(rows))
			s.itemsCurrentTotal -= uint64(len(rows))
		}
		delete(s.MetricsMetadataStorage, key)
		delete(s.MetricTimingInfo, key)
	}
}

func (s *Store) get(limit, limitPerMetric int64, metric string, keepFilter func(k metadataKey) bool) []Row {
	if limit <= 0 {
		limit = 0
	}
	if limitPerMetric <= 0 {
		limitPerMetric = 0
	}

	s.metricMetadataStorageLock.RLock()
	defer s.metricMetadataStorageLock.RUnlock()
	prealloc := int(limit)
	if limit == 0 {
		// assume that we will return all entries
		// Using itemsCurrentTotal counter for better performance
		prealloc = int(s.itemsCurrentTotal)
		if limitPerMetric > 0 {
			prealloc = len(s.MetricsMetadataStorage) * int(limitPerMetric)
		}
	}
	res := make([]Row, 0, prealloc)
	for k, m := range s.MetricsMetadataStorage {
		if len(metric) > 0 && k.metricFamilyName != metric {
			continue
		}

		perMetric := int64(0)
		for _, r := range m {
			if keepFilter != nil && !keepFilter(k) {
				continue
			}

			res = append(res, r)
			perMetric++

			if limitPerMetric > 0 && perMetric >= limitPerMetric {
				break
			}

			if limit > 0 && len(res) >= int(limit) {
				return res
			}
		}
	}

	return res
}

func (s *Store) GetForTenant(accountID, projectID uint32, limit, limitPerMetric int64, metric string) []Row {
	keepFilter := func(k metadataKey) bool {
		return k.accountID == accountID && k.projectID == projectID
	}
	return s.get(limit, limitPerMetric, metric, keepFilter)
}

func (s *Store) Get(limit, limitPerMetric int64, metric string) []Row {
	return s.get(limit, limitPerMetric, metric, nil)
}

func (s *Store) UpdateMetrics(dst *MetadataStoreMetrics) {
	s.metricMetadataStorageLock.RLock()
	defer s.metricMetadataStorageLock.RUnlock()
	totalSize := 0
	perRowOverhead := int(unsafe.Sizeof(metadataKey{})) + int(unsafe.Sizeof(Row{})) + 24 // 24 bytes for map overhead
	for _, rows := range s.MetricsMetadataStorage {
		for _, row := range rows {
			totalSize += len(row.MetricFamilyName) + len(row.Help) + len(row.Unit) + perRowOverhead
		}
	}
	dst.ItemsTotal = s.itemsCurrentTotal
	dst.ItemsIngestedTotal = s.itemsIngestedTotal
	dst.ItemsDeduplicated = s.itemsDeduplicated
	dst.ItemsDeleted = s.itemsDeleted
	dst.ItemsSizeBytes = uint64(totalSize)
}
func (s *Store) mustSaveLocked() {
	if s.storagePath == "" {
		return
	}

	// Create cachePath dir if it doesn't exist in the same manner as other caches doing
	dir := filepath.Dir(s.storagePath)
	if !fs.IsPathExist(dir) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Fatalf("cannot create dir %q: %s", dir, err)
		}
	}

	var bb bytes.Buffer
	zw := gzip.NewWriter(&bb)
	je := json.NewEncoder(zw)

	var r recordForStore
	for key, rows := range s.MetricsMetadataStorage {
		r.AccountID = key.accountID
		r.ProjectID = key.projectID
		r.MetricFamilyName = key.metricFamilyName
		r.Rows = rows
		if timingInfo, ok := s.MetricTimingInfo[key]; ok {
			r.TimingInfo = timingInfo
		} else {
			r.TimingInfo = metricTimingInfo{}
		}
		if err := je.Encode(r); err != nil {
			logger.Fatalf("cannot save encoded record for %q: %s", key.metricFamilyName, err)
		}
	}
	if err := zw.Close(); err != nil {
		logger.Fatalf("cannot close gzip writer: %s", err)
	}

	// Atomically store the data
	data := bb.Bytes()
	fs.MustWriteAtomic(s.storagePath, data, true)

}

func (s *Store) LoadFrom(path string) error {
	s.storagePath = path

	// fast path
	if !fs.IsPathExist(s.storagePath) {
		return nil
	}

	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return fmt.Errorf("cannot read metrics metadata from %q: %w", s.storagePath, err)
	}

	bb := bytes.NewBuffer(data)
	zr, err := gzip.NewReader(bb)
	if err != nil {
		return fmt.Errorf("cannot create new gzip reader: %w", err)
	}
	defer func() {
		if err := zr.Close(); err != nil {
			logger.Panicf("FATAL: cannot close gzip reader: %s", err)
		}
	}()
	jd := json.NewDecoder(zr)

	var r recordForStore
	for {
		if err := jd.Decode(&r); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("cannot parse record: %w", err)
		}
		key := metadataKey{
			accountID:        r.AccountID,
			projectID:        r.ProjectID,
			metricFamilyName: r.MetricFamilyName,
		}
		s.MetricsMetadataStorage[key] = r.Rows
		s.MetricTimingInfo[key] = r.TimingInfo
		s.itemsCurrentTotal += uint64(len(r.Rows))
	}
	s.itemsIngestedTotal = s.itemsCurrentTotal
	return nil
}
