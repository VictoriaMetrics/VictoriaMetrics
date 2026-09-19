package metricsmetadata

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

func TestStoragePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "metadata")
	s := NewStorage(path, 1<<20)
	rows := []Row{
		{MetricFamilyName: []byte("requests"), Help: []byte("default help"), Unit: []byte("requests"), Type: 1},
		{AccountID: 7, ProjectID: 3, MetricFamilyName: []byte("requests"), Help: []byte("tenant help"), Unit: []byte("seconds"), Type: 2},
		{AccountID: 7, ProjectID: 4, MetricFamilyName: []byte("requests"), Help: []byte("other project"), Type: 3},
	}
	s.Add(rows)
	// A final partial update immediately before shutdown must be included.
	s.Add([]Row{{AccountID: 7, ProjectID: 3, MetricFamilyName: []byte("requests"), Help: []byte("updated help")}})
	rows[1].Help = []byte("updated help")
	lastWriteTimes := make([]uint64, len(rows))
	for i, r := range rows {
		lastWriteTimes[i] = s.GetForTenant(r.AccountID, r.ProjectID, 0, "requests")[0].lastWriteTime
	}
	for range 3 {
		s.MustClose()
		s = NewStorage(path, 1<<20)
		for i, want := range rows {
			got := s.GetForTenant(want.AccountID, want.ProjectID, 0, "requests")
			if len(got) != 1 || !got[0].matchesNonEmptyRow(&want) || got[0].lastWriteTime != lastWriteTimes[i] {
				t.Fatalf("metadata did not survive restart for tenant %d:%d", want.AccountID, want.ProjectID)
			}
		}
		if got := s.GetForTenant(8, 3, 0, ""); len(got) != 0 {
			t.Fatalf("metadata leaked to another tenant")
		}
	}
	defer s.MustClose()
	// A partial update after restoration must merge with the restored row.
	s.Add([]Row{{AccountID: 7, ProjectID: 3, MetricFamilyName: []byte("requests"), Unit: []byte("bytes")}})
	got := s.GetForTenant(7, 3, 0, "requests")[0]
	if string(got.Help) != "updated help" || string(got.Unit) != "bytes" || got.Type != 2 {
		t.Fatalf("partial update lost restored fields")
	}
	if got := s.GetForTenant(0, 0, 0, "requests")[0]; string(got.Unit) != "requests" {
		t.Fatalf("partial update changed another tenant")
	}
}

func TestStoragePersistenceExpiration(t *testing.T) {
	const now = uint64(10000)
	const ttl = uint64(metadataExpireDuration / time.Second)
	s := NewStorage("", 1<<20)
	defer s.MustClose()
	add := func(name string, ts uint64) {
		r := Row{MetricFamilyName: []byte(name), Help: []byte("help")}
		s.buckets[xxhash.Sum64(r.MetricFamilyName)%bucketsCount].add(&r, ts)
	}
	add("expired", now-ttl)
	add("live", now-ttl+1)
	add("recent", now-10)
	// Save just before expiration, then spend one second offline.
	data := s.marshalCache(now - 1)
	restored := NewStorage("", 1<<20)
	defer restored.MustClose()
	if err := restored.unmarshalCache(data, now); err != nil {
		t.Fatal(err)
	}
	if len(restored.Get(0, "expired")) != 0 || len(restored.Get(0, "live")) != 1 {
		t.Fatal("incorrect expiration boundary on load")
	}
	if got := restored.Get(0, "live")[0].lastWriteTime; got != now-ttl+1 {
		t.Fatalf("load refreshed ingestion time: %d", got)
	}
	// Repeated shutdown/restart must not extend the lifetime.
	next := NewStorage("", 1<<20)
	defer next.MustClose()
	if err := next.unmarshalCache(restored.marshalCache(now), now+1); err != nil {
		t.Fatal(err)
	}
	if len(next.Get(0, "live")) != 0 || len(next.Get(0, "recent")) != 1 {
		t.Fatal("restart extended the expiration deadline")
	}
	// Expired rows must also be pruned from the saved file, even without a
	// cleaner tick between the last ingestion and shutdown.
	empty := NewStorage("", 1<<20)
	defer empty.MustClose()
	if err := empty.unmarshalCache(s.marshalCache(now+ttl), now+ttl); err != nil {
		t.Fatal(err)
	}
	if empty.totalItems() != 0 {
		t.Fatal("saved expired rows")
	}
}

func TestStoragePersistenceInvalidCache(t *testing.T) {
	s := NewStorage("", 1<<20)
	defer s.MustClose()
	s.Add([]Row{{MetricFamilyName: []byte("first"), Help: []byte("help")}, {MetricFamilyName: []byte("second")}})
	data := s.marshalCache(fasttime.UnixTimestamp())
	f := func(data []byte) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "metadata")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		restored := NewStorage(path, 1<<20)
		defer restored.MustClose()
		if restored.totalItems() != 0 {
			t.Fatal("invalid cache left partially restored rows")
		}
	}
	// Includes truncation exactly between records.
	for n := range len(data) {
		f(data[:n])
	}
	bad := bytes.Clone(data)
	bad[7]++
	f(bad)
	bad = bytes.Clone(data)
	bad[len(bad)-1]++
	f(bad)
	// Well-formed envelope with malicious lengths in the second record.
	bad = bytes.Clone(data)
	_, tail, err := unmarshalCacheRow(bad[cacheHeaderSize:])
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(tail[20:], ^uint32(0))
	binary.BigEndian.PutUint32(bad[len(cacheMagic)+8:], crc32.ChecksumIEEE(bad[cacheHeaderSize:]))
	f(bad)
}

func TestStoragePersistenceBudget(t *testing.T) {
	f := func(limit int) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "metadata")
		s := NewStorage(path, 1<<20)
		// Same name puts all tenants into the same bucket.
		for i := range 20 {
			s.Add([]Row{{AccountID: uint32(i), MetricFamilyName: []byte("requests"), Help: []byte("help")}})
		}
		s.MustClose()
		s = NewStorage(path, limit)
		defer s.MustClose()
		var m MetadataStorageMetrics
		s.UpdateMetrics(&m)
		if m.CurrentSizeBytes > m.MaxSizeBytes {
			t.Fatalf("restore exceeded memory budget: %+v", m)
		}
		for _, b := range s.buckets {
			if b.itemsTotalSize.Load() > b.maxSizeBytes {
				t.Fatal("restore exceeded shard budget")
			}
		}
		if limit <= 1 && s.totalItems() != 0 {
			t.Fatal("disabled or too-small cache restored rows")
		}
		if limit == 4096 && (s.totalItems() == 0 || s.totalItems() >= 20) {
			t.Fatal("lower shard budget should retain only some rows")
		}
	}
	f(0)
	f(-1)
	f(1)
	f(512)  // File exceeds new read budget: empty fallback.
	f(4096) // File fits; restore must evict to respect the smaller shard budget.
}

func TestStoragePersistenceMissingAndDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	s := NewStorage(path, 0)
	s.Add([]Row{{MetricFamilyName: []byte("ignored")}})
	if s.totalItems() != 0 {
		t.Fatal("zero budget accepted rows")
	}
	s.MustClose()
	s = NewStorage(path, 1<<20)
	defer s.MustClose()
	if s.totalItems() != 0 {
		t.Fatal("disabled cache saved rows")
	}
}

func TestStorageBudgetAfterUpdate(t *testing.T) {
	s := NewStorage("", 1024*bucketsCount)
	defer s.MustClose()
	for i := range 5 {
		s.Add([]Row{{AccountID: uint32(i), MetricFamilyName: []byte("same_bucket")}})
	}
	s.Add([]Row{{MetricFamilyName: []byte("same_bucket"), Help: bytes.Repeat([]byte("x"), 800)}})
	b := s.buckets[xxhash.Sum64([]byte("same_bucket"))%bucketsCount]
	if b.itemsTotalSize.Load() > b.maxSizeBytes {
		t.Fatal("growing row exceeded budget")
	}
	s.Add([]Row{{MetricFamilyName: []byte("same_bucket"), Help: bytes.Repeat([]byte("x"), 2048)}})
	if b.itemsTotalSize.Load() > b.maxSizeBytes {
		t.Fatal("oversized row exceeded budget")
	}
}

func TestStoragePersistenceConcurrentSnapshot(t *testing.T) {
	s := NewStorage("", 1<<20)
	defer s.MustClose()
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 100 {
			s.Add([]Row{{MetricFamilyName: []byte("requests"), Help: []byte("help")}})
		}
	})
	for range 100 {
		data := s.marshalCache(fasttime.UnixTimestamp())
		restored := NewStorage("", 1<<20)
		if err := restored.unmarshalCache(data, fasttime.UnixTimestamp()); err != nil {
			t.Fatal(err)
		}
		restored.MustClose()
	}
	wg.Wait()
}
