package metricsmetadata

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
)

func TestStoreWrite(t *testing.T) {
	synctest.Run(func() {
		s := NewStore(memory.Allowed())
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

		err = s.Add(rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify metrics were added
		r := s.Get(0, 0, "")
		if len(r) != 2 {
			t.Fatalf("expected 2 metrics, got %d", len(r))
		}

		// Test adding duplicate metadata (should not add but update timing)
		err = s.Add(rows[:1])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r = s.Get(0, 0, "")
		if len(r) != 2 {
			t.Fatalf("expected 2 metrics, got %d", len(r))
		}
	})
}

func TestStoreCleanup(t *testing.T) {
	synctest.Run(func() {
		s := NewStore(memory.Allowed())
		defer s.MustClose()

		// Add test data
		row1 := Row{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			AccountID:        1,
			ProjectID:        1,
		}
		row2 := Row{
			MetricFamilyName: []byte("metric2"),
			Type:             1,
			AccountID:        2,
			ProjectID:        1,
		}

		_ = s.Add([]Row{row1, row2})

		time.Sleep(storeMetadataTTL / 2)
		_ = s.Add([]Row{row1})

		time.Sleep(storeMetadataTTL * 2)

		r := s.Get(-1, -1, "metric1")
		if len(r) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(r))
		}

		r = s.Get(0, 0, "metric2")
		if len(r) != 0 {
			t.Fatalf("expected nothing to be found, got len: %d", len(r))
		}
	})
}

func TestStoreRead(t *testing.T) {
	s := NewStore(memory.Allowed())
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

func TestMetricsGathering(t *testing.T) {
	synctest.Run(func() {
		s := NewStore(memory.Allowed())
		defer s.MustClose()

		totalItems := int(10e3)

		rows := getRows(totalItems)
		_ = s.Add(rows)

		m := MetadataStoreMetrics{}

		s.UpdateMetrics(&m)
		if m.ItemsCurrent != int64(totalItems) {
			t.Fatalf("expected %d items, got %d", totalItems, m.ItemsCurrent)
		}

		_ = s.Add(rows)
		s.UpdateMetrics(&m)
		if m.ItemsCurrent != int64(totalItems) {
			t.Fatalf("expected %d items, got %d", totalItems, m.ItemsCurrent)
		}

		time.Sleep(2 * max(storeRotationInterval, storeMetadataTTL))

		s.UpdateMetrics(&m)
		if m.ItemsCurrent != 0 {
			t.Fatalf("expected %d items, got %d", 0, m.ItemsCurrent)
		}
	})
}
