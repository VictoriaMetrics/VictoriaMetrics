package metricsmetadata

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestStoreWrite(t *testing.T) {
	synctest.Run(func() {
		s := NewStore()
		defer s.MustClose()

		// Test adding empty rows
		err := s.Add(nil)
		if err != nil {
			t.Fatalf("unexpected error on empty add: %v", err)
		}

		// Test adding new metrics
		rows := []Row{
			{
				MetricFamilyName: []byte("metric1"),
				Type:             1,
				Unit:             []byte("seconds"),
				Help:             []byte("help1"),
				AccountID:        1,
				ProjectID:        1,
			},
			{
				MetricFamilyName: []byte("metric2"),
				Type:             2,
				Unit:             []byte("bytes"),
				Help:             []byte("help2"),
				AccountID:        1,
				ProjectID:        1,
			},
		}

		time.Sleep(1000 * time.Millisecond)

		err = s.Add(rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify metrics were added
		s.metricMetadataStorageLock.RLock()
		if len(s.metricsMetadataStorage) != 2 {
			t.Fatalf("expected 2 metrics, got %d", len(s.metricsMetadataStorage))
		}
		if len(s.metricTimingInfo) != 2 {
			t.Fatalf("expected 2 timing entries, got %d", len(s.metricTimingInfo))
		}
		s.metricMetadataStorageLock.RUnlock()

		time.Sleep(1000 * time.Millisecond)

		// Test adding duplicate metadata (should not add but update timing)
		err = s.Add(rows[:1])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		s.metricMetadataStorageLock.RLock()
		if len(s.metricsMetadataStorage["metric1"]) != 1 {
			t.Fatalf("duplicate metadata should not be added")
		}
		timing := s.metricTimingInfo["metric1"]
		if timing.ingestionCount != 2 {
			t.Fatalf("expected ingestion count 2, got %d", timing.ingestionCount)
		}
		s.metricMetadataStorageLock.RUnlock()
		synctest.Wait()
	})
}

func TestStoreAvgCalculation(t *testing.T) {
	synctest.Run(func() {
		s := NewStore()
		defer s.MustClose()

		metricName := "test_metric"
		row := Row{
			MetricFamilyName: []byte(metricName),
			Type:             1,
			Unit:             []byte("unit"),
			Help:             []byte("help"),
		}

		_ = s.Add([]Row{row})

		time.Sleep(100 * time.Second)
		_ = s.Add([]Row{row})

		time.Sleep(100 * time.Second)
		_ = s.Add([]Row{row})

		time.Sleep(100 * time.Second)
		_ = s.Add([]Row{row})

		s.metricMetadataStorageLock.RLock()
		timing := s.metricTimingInfo[metricName]
		s.metricMetadataStorageLock.RUnlock()

		expectedAvg := uint64(100)
		if timing.avgIngestionInterval != expectedAvg {
			t.Fatalf("expected avg interval %d, got %d", expectedAvg, timing.avgIngestionInterval)
		}
		if timing.ingestionCount != 4 {
			t.Fatalf("expected ingestion count 3, got %d", timing.ingestionCount)
		}
		synctest.Wait()
	})
}

func TestStoreCleanup(t *testing.T) {
	synctest.Run(func() {
		// Create store with short cleanup interval for testing
		s := &Store{
			metricsMetadataStorage: make(map[string][]Row),
			metricTimingInfo:       make(map[string]*metricTimingInfo),
			cleanupInterval:        100 * time.Millisecond,
			cleanupStopCh:          make(chan struct{}),
		}

		// Add test data
		row1 := Row{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
		}
		row2 := Row{
			MetricFamilyName: []byte("metric2"),
			Type:             1,
		}

		_ = s.Add([]Row{row1})
		_ = s.Add([]Row{row2})
		time.Sleep(1000 * time.Millisecond)

		_ = s.Add([]Row{row1})
		time.Sleep(1000 * time.Millisecond)

		_ = s.Add([]Row{row1})
		time.Sleep(1000 * time.Millisecond)

		// metric1 has interval of 1000ms, so it should be cleaned up after 10 seconds
		// metric2 has only 2 ingestions, so it should not be cleaned up
		time.Sleep(11000 * time.Millisecond)

		s.cleanup()

		if _, exists := s.metricsMetadataStorage["metric1"]; exists {
			t.Fatal("metric1 should have been cleaned up")
		}
		if _, exists := s.metricTimingInfo["metric1"]; exists {
			t.Fatal("metric1 timing should have been cleaned up")
		}

		if _, exists := s.metricsMetadataStorage["metric2"]; !exists {
			t.Fatal("metric2 should not have been cleaned up")
		}
		synctest.Wait()
	})
}

func TestStoreRead(t *testing.T) {
	s := NewStore()
	defer s.MustClose()

	// Add test data
	rows := []Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric1"),
			Type:             2,
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             1,
			AccountID:        2,
			ProjectID:        1,
		},
	}
	_ = s.Add(rows)

	// Test Get all
	result := s.Get(10, 10, "")
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}

	// Test Get with metric filter
	result = s.Get(10, 10, "metric1")
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}

	// Test Get with limit
	result = s.Get(1, 10, "")
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	// Test GetForTenant
	result = s.GetForTenant(1, 1, 10, 10, "")
	if len(result) != 2 {
		t.Fatalf("expected 2 results for tenant, got %d", len(result))
	}

	// Test limitPerMetric
	result = s.Get(10, 1, "")
	if len(result) != 2 { // 1 from metric1, 1 from metric2
		t.Fatalf("expected 2 results with limitPerMetric, got %d", len(result))
	}

	result = s.Get(0, 0, "nonexistent_metric")
	if len(result) != 0 {
		t.Fatalf("expected 0 results for nonexistent metric, got %d", len(result))
	}

	result = s.GetForTenant(3, 3, 0, 0, "")
	if len(result) != 0 {
		t.Fatalf("expected 0 results for nonexistent tenant, got %d", len(result))
	}
}
