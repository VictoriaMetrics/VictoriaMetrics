package metricsmetadata

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

func TestRepairOversizedRowIsolation(t *testing.T) {
	f := func(t *testing.T, update bool, help, unit []byte) {
		t.Helper()
		b := &bucket{maxSizeBytes: 1024, perTenantStorage: make(map[uint64]map[string]*Row)}
		for i := uint32(1); i <= 3; i++ {
			b.add(&Row{AccountID: i, MetricFamilyName: []byte("same"), Help: []byte("keep"), Unit: []byte("seconds"), Type: 1}, 100+uint64(i))
		}
		before := b.itemsTotalSize.Load()
		rowsLen, namesLen := len(b.rowsBuff), len(b.metricNamesBuf)
		old := b.perTenantStorage[encodeTenantID(1, 0)]["same"]
		id := uint32(4)
		if update {
			id = 1
		}
		b.add(&Row{AccountID: id, MetricFamilyName: []byte("same"), Help: help, Unit: unit, Type: 2}, 110)
		for i := uint32(1); i <= 3; i++ {
			r := b.perTenantStorage[encodeTenantID(i, 0)]["same"]
			if r == nil || string(r.Help) != "keep" || string(r.Unit) != "seconds" || r.Type != 1 || r.lastWriteTime != 100+uint64(i) {
				t.Errorf("rejected row changed tenant %d: %v", i, r)
			}
		}
		if b.perTenantStorage[encodeTenantID(1, 0)]["same"] != old {
			t.Error("oversized update replaced the previous valid row")
		}
		if len(b.perTenantStorage) != 3 || b.itemsCurrent.Load() != 3 || len(b.lwh) != 3 {
			t.Error("rejected row changed tenant maps, item count or heap")
		}
		if b.itemsTotalSize.Load() != before || before > b.maxSizeBytes {
			t.Error("rejected row changed accounting or exceeded budget")
		}
		if len(b.rowsBuff) != rowsLen || len(b.metricNamesBuf) != namesLen {
			t.Error("rejected row was cloned")
		}
	}
	t.Run("insert", func(t *testing.T) { f(t, false, bytes.Repeat([]byte("x"), 2048), nil) })
	t.Run("update", func(t *testing.T) { f(t, true, bytes.Repeat([]byte("x"), 2048), nil) })
	// The incoming row alone fits exactly; the retained non-empty field
	// makes the merged row too large. Exercise both merge directions.
	t.Run("merge-retained-unit", func(t *testing.T) {
		f(t, true, bytes.Repeat([]byte("x"), int(1024-perItemOverhead)-len("same")), nil)
	})
	t.Run("merge-retained-help", func(t *testing.T) {
		f(t, true, nil, bytes.Repeat([]byte("x"), int(1024-perItemOverhead)-len("same")))
	})
}

func TestRepairExactShardBudget(t *testing.T) {
	b := &bucket{maxSizeBytes: 1024, perTenantStorage: make(map[uint64]map[string]*Row)}
	r := Row{MetricFamilyName: []byte("same"), Help: bytes.Repeat([]byte("x"), int(1024-perItemOverhead)-len("same"))}
	b.add(&r, 100)
	b.add(&Row{MetricFamilyName: []byte("same"), Type: 2}, 101)
	got := b.perTenantStorage[0]["same"]
	if got == nil || !bytes.Equal(got.Help, r.Help) || got.Type != 2 || got.lastWriteTime != 101 || b.itemsTotalSize.Load() != b.maxSizeBytes || b.itemsCurrent.Load() != 1 {
		t.Fatal("row fitting the exact shard budget or its partial update was rejected")
	}
}

func TestRepairRejectedInsertDoesNotCreateTenant(t *testing.T) {
	b := &bucket{maxSizeBytes: 1, perTenantStorage: make(map[uint64]map[string]*Row)}
	for i := uint32(0); i < 100; i++ {
		b.add(&Row{AccountID: i, MetricFamilyName: []byte("too_large")}, 100)
	}
	if len(b.perTenantStorage) != 0 || len(b.lwh) != 0 || len(b.rowsBuff) != 0 || len(b.metricNamesBuf) != 0 || b.itemsTotalSize.Load() != 0 || b.itemsCurrent.Load() != 0 {
		t.Fatal("rejected inserts retained maps, rows or backing buffers")
	}
}

func TestRepairRestoreReducedBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata")
	s := NewStorage(path, 1<<20)
	now := fasttime.UnixTimestamp()
	b := s.buckets[xxhash.Sum64([]byte("same"))%bucketsCount]
	for i := uint32(1); i <= 3; i++ {
		b.add(&Row{AccountID: i, ProjectID: 7, MetricFamilyName: []byte("same"), Help: []byte("keep"), Unit: []byte("seconds"), Type: 1}, now-10+uint64(i))
	}
	b.add(&Row{AccountID: 4, ProjectID: 7, MetricFamilyName: []byte("same"), Help: bytes.Repeat([]byte("x"), 2048)}, now)
	if len(s.marshalCache(now)) > 8192 {
		t.Fatal("control: cache must fit the reduced global read limit")
	}
	s.MustClose()
	s = NewStorage(path, 8192)
	defer s.MustClose()
	for i := uint32(1); i <= 3; i++ {
		rows := s.GetForTenant(i, 7, 0, "same")
		if len(rows) != 1 || string(rows[0].Help) != "keep" || string(rows[0].Unit) != "seconds" || rows[0].Type != 1 || rows[0].lastWriteTime != now-10+uint64(i) {
			t.Errorf("restore lost or changed fitting tenant %d", i)
		}
	}
	if len(s.GetForTenant(4, 7, 0, "same")) != 0 || s.totalItems() != 3 {
		t.Error("restore retained oversized row or wrong row count")
	}
	var m MetadataStorageMetrics
	s.UpdateMetrics(&m)
	if m.CurrentSizeBytes > m.MaxSizeBytes {
		t.Error("restore exceeded global budget")
	}
	for _, b := range s.buckets {
		if b.itemsTotalSize.Load() > b.maxSizeBytes {
			t.Error("restore exceeded shard budget")
		}
	}
}

func TestRepairClockRollback(t *testing.T) {
	f := func(t *testing.T, written, saveAt, loadAt uint64) {
		t.Helper()
		s := NewStorage("", 1<<20)
		defer s.MustClose()
		r := Row{AccountID: 7, ProjectID: 3, MetricFamilyName: []byte("rollback"), Help: []byte("keep"), Unit: []byte("seconds"), Type: 1}
		s.buckets[xxhash.Sum64(r.MetricFamilyName)%bucketsCount].add(&r, written)
		const ttl = uint64(metadataExpireDuration / time.Second)
		for _, at := range []uint64{loadAt, loadAt, written, written + ttl - 1} {
			next := NewStorage("", 1<<20)
			defer next.MustClose()
			if err := next.unmarshalCache(s.marshalCache(saveAt), at); err != nil {
				t.Fatal(err)
			}
			got := next.GetForTenant(7, 3, 0, "rollback")
			if len(got) != 1 || !got[0].matchesNonEmptyRow(&r) || got[0].lastWriteTime != written {
				t.Fatalf("restart at %d lost live row or renewed its timestamp", at)
			}
			s, saveAt = next, at
		}
		// Check expiration at age == TTL independently at load and save.
		for _, at := range []uint64{written + ttl, written + ttl + 1} {
			for _, save := range []uint64{saveAt, at} {
				next := NewStorage("", 1<<20)
				defer next.MustClose()
				// Loading at the previous live time proves save itself pruned.
				load := at
				if save == at {
					load = saveAt
				}
				if err := next.unmarshalCache(s.marshalCache(save), load); err != nil {
					t.Fatal(err)
				}
				if next.totalItems() != 0 {
					t.Fatalf("row survived TTL boundary: save=%d load=%d", save, load)
				}
			}
		}
	}
	t.Run("save", func(t *testing.T) { f(t, 10000, 9998, 10000) })
	t.Run("load", func(t *testing.T) { f(t, 10000, 10000, 9998) })
	t.Run("save-and-load", func(t *testing.T) { f(t, 10000, 9998, 9998) })
	t.Run("near-epoch", func(t *testing.T) { f(t, 2, 0, 0) })
}
