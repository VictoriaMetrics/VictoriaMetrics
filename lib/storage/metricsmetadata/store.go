package metricsmetadata

import (
	"bytes"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

type metricTimingInfo struct {
	lastIngestionTime    uint64
	avgIngestionInterval uint64
	ingestionCount       int64
}

type Store struct {
	metricsMetadataStorage    map[string][]Row
	metricTimingInfo          map[string]*metricTimingInfo
	metricMetadataStorageLock sync.RWMutex

	cleanupInterval time.Duration
	cleanupStopCh   chan struct{}
	cleanupWG       sync.WaitGroup
}

func NewStore() *Store {
	s := &Store{
		metricsMetadataStorage: make(map[string][]Row),
		metricTimingInfo:       make(map[string]*metricTimingInfo),
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
		mf := bytesutil.ToUnsafeString(mr.MetricFamilyName)
		s.updateMetricTimingLocked(mf, now)

		metadataRows, ok := s.metricsMetadataStorage[mf]
		if !ok {
			s.metricsMetadataStorage[mf] = make([]Row, 0, 1)
			s.metricsMetadataStorage[mf] = append(s.metricsMetadataStorage[mf], mr)
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
			continue
		}
		s.metricsMetadataStorage[mf] = append(metadataRows, mr)
	}

	return nil
}

func (s *Store) updateMetricTimingLocked(metricName string, now uint64) {
	timing, exists := s.metricTimingInfo[metricName]
	if !exists {
		s.metricTimingInfo[metricName] = &metricTimingInfo{
			lastIngestionTime:    now,
			avgIngestionInterval: 0,
			ingestionCount:       1,
		}
		return
	}

	timeSinceLastIngestion := now - timing.lastIngestionTime

	// Update running average using exponential moving average
	if timing.ingestionCount == 1 {
		timing.avgIngestionInterval = timeSinceLastIngestion
	} else {
		alpha := 0.2
		timing.avgIngestionInterval = uint64(float64(timing.avgIngestionInterval)*(1-alpha) + float64(timeSinceLastIngestion)*alpha)
	}

	timing.lastIngestionTime = now
	timing.ingestionCount++
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
	var metricsToDelete []string

	for metricName, timing := range s.metricTimingInfo {
		if timing.ingestionCount < 2 {
			continue
		}

		// Check if it's been more than 10x the average interval since last ingestion
		// Prometheus keeps metadata for 10 scrapes after the last ingestion
		// https://github.com/prometheus/prometheus/blob/5a5424cbc1422ddfd94651122845fdc4a2e8b5c7/scrape/scrape.go#L1041
		timeSinceLastIngestion := now - timing.lastIngestionTime
		threshold := timing.avgIngestionInterval * 10

		if timeSinceLastIngestion > threshold {
			metricsToDelete = append(metricsToDelete, metricName)
		}
	}

	for _, metricName := range metricsToDelete {
		delete(s.metricsMetadataStorage, metricName)
		delete(s.metricTimingInfo, metricName)
	}
}

func (s *Store) get(limit, limitPerMetric int64, metric string, keepFilter func(row Row) bool) []Row {
	if limit <= 0 {
		limit = 0
	}
	if limitPerMetric <= 0 {
		limitPerMetric = 0
	}

	s.metricMetadataStorageLock.RLock()
	defer s.metricMetadataStorageLock.RUnlock()
	res := make([]Row, 0, limit)
	for k, m := range s.metricsMetadataStorage {
		if len(metric) > 0 && k != metric {
			continue
		}

		perMetric := int64(0)
		for _, r := range m {
			if keepFilter != nil && !keepFilter(r) {
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
	keepFilter := func(row Row) bool {
		return row.AccountID == accountID && row.ProjectID == projectID
	}
	return s.get(limit, limitPerMetric, metric, keepFilter)
}

func (s *Store) Get(limit, limitPerMetric int64, metric string) []Row {
	return s.get(limit, limitPerMetric, metric, nil)
}
