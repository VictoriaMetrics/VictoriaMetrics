package metricsmetadata

import (
	"bytes"
	"sync"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

type metadataKey struct {
	accountID        uint32
	projectID        uint32
	metricFamilyName string
}

type metricTimingInfo struct {
	LastIngestionTime    uint64
	AvgIngestionInterval uint64
	IngestionCount       int64
}

type MetadataStoreMetrics struct {
	ItemsTotal         uint64
	ItemsIngestedTotal uint64
	ItemsDeduplicated  uint64
	ItemsDeleted       uint64
	ItemsSizeBytes     uint64
}

type Store struct {
	metricMetadataStorageLock sync.RWMutex
	metricsMetadataStorage    map[metadataKey][]Row
	metricTimingInfo          map[metadataKey]metricTimingInfo

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
		metricsMetadataStorage: make(map[metadataKey][]Row),
		metricTimingInfo:       make(map[metadataKey]metricTimingInfo),
		cleanupInterval:        5 * time.Minute,
		cleanupStopCh:          make(chan struct{}),
	}

	s.cleanupWG.Add(1)
	go s.runCleanupScheduler()

	return s
}

func (s *Store) MustClose() {
	close(s.cleanupStopCh)
	s.cleanupWG.Wait()
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
		s.updateMetricTimingLocked(&key, now)

		metadataRows, ok := s.metricsMetadataStorage[key]
		if !ok {
			s.metricsMetadataStorage[key] = make([]Row, 0, 1)
			s.metricsMetadataStorage[key] = append(s.metricsMetadataStorage[key], mr)
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
		s.metricsMetadataStorage[key] = append(metadataRows, mr)
		s.itemsIngestedTotal++
		s.itemsCurrentTotal++
	}

	return nil
}

func (s *Store) updateMetricTimingLocked(key *metadataKey, now uint64) {
	timing, exists := s.metricTimingInfo[*key]
	if !exists {
		s.metricTimingInfo[*key] = metricTimingInfo{
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

	s.metricTimingInfo[*key] = timing
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

	for key, timing := range s.metricTimingInfo {
		if timing.IngestionCount < 2 {
			continue
		}

		// Check if it's been more than 10x the average interval since last ingestion
		// Prometheus keeps metadata for 10 scrapes after the last ingestion
		// https://github.com/prometheus/prometheus/blob/5a5424cbc1422ddfd94651122845fdc4a2e8b5c7/scrape/scrape.go#L1041
		timeSinceLastIngestion := now - timing.LastIngestionTime
		threshold := timing.AvgIngestionInterval * 10

		if timeSinceLastIngestion > threshold {
			if rows, ok := s.metricsMetadataStorage[key]; ok {
				s.itemsDeleted += uint64(len(rows))
				s.itemsCurrentTotal -= uint64(len(rows))
			}
			delete(s.metricsMetadataStorage, key)
			delete(s.metricTimingInfo, key)
		}
	}
}

func (s *Store) get(limit, limitPerMetric int64, metric string, keepFilter func(k metadataKey) bool) []Row {
	if limit < 0 {
		limit = 0
	}
	if limitPerMetric < 0 {
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
			prealloc = len(s.metricsMetadataStorage) * int(limitPerMetric)
		}
	}
	res := make([]Row, 0, prealloc)
	metricLen := len(metric)
	for k, m := range s.metricsMetadataStorage {
		if metricLen > 0 && k.metricFamilyName != metric {
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
	for _, rows := range s.metricsMetadataStorage {
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
