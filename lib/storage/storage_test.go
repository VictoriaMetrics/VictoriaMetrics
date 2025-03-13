package storage

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

func TestReplaceAlternateRegexpsWithGraphiteWildcards(t *testing.T) {
	f := func(q, resultExpected string) {
		t.Helper()
		result := replaceAlternateRegexpsWithGraphiteWildcards([]byte(q))
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for %s\ngot\n%s\nwant\n%s", q, result, resultExpected)
		}
	}
	f("", "")
	f("foo", "foo")
	f("foo(bar", "foo(bar")
	f("foo.(bar|baz", "foo.(bar|baz")
	f("foo.(bar).x", "foo.{bar}.x")
	f("foo.(bar|baz).*.{x,y}", "foo.{bar,baz}.*.{x,y}")
	f("foo.(bar|baz).*.{x,y}(z|aa)", "foo.{bar,baz}.*.{x,y}{z,aa}")
	f("foo(.*)", "foo*")
}

func TestGetRegexpForGraphiteNodeQuery(t *testing.T) {
	f := func(q, expectedRegexp string) {
		t.Helper()
		re, err := getRegexpForGraphiteQuery(q)
		if err != nil {
			t.Fatalf("unexpected error for query=%q: %s", q, err)
		}
		reStr := re.String()
		if reStr != expectedRegexp {
			t.Fatalf("unexpected regexp for query %q; got %q want %q", q, reStr, expectedRegexp)
		}
	}
	f(``, `^$`)
	f(`*`, `^[^.]*$`)
	f(`foo.`, `^foo\.$`)
	f(`foo.bar`, `^foo\.bar$`)
	f(`{foo,b*ar,b[a-z]}`, `^(?:foo|b[^.]*ar|b[a-z])$`)
	f(`[-a-zx.]`, `^[-a-zx.]$`)
	f(`**`, `^[^.]*[^.]*$`)
	f(`a*[de]{x,y}z`, `^a[^.]*[de](?:x|y)z$`)
	f(`foo{bar`, `^foo\{bar$`)
	f(`foo{ba,r`, `^foo\{ba,r$`)
	f(`foo[bar`, `^foo\[bar$`)
	f(`foo{bar}`, `^foobar$`)
	f(`foo{bar,,b{{a,b*},z},[x-y]*z}a`, `^foo(?:bar||b(?:(?:a|b[^.]*)|z)|[x-y][^.]*z)a$`)
}

func TestDateMetricIDCacheSerial(t *testing.T) {
	c := newDateMetricIDCache()
	if err := testDateMetricIDCache(c, false); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestDateMetricIDCacheConcurrent(t *testing.T) {
	c := newDateMetricIDCache()
	ch := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			ch <- testDateMetricIDCache(c, true)
		}()
	}
	for i := 0; i < 5; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(time.Second * 5):
			t.Fatalf("timeout")
		}
	}
}

func testDateMetricIDCache(c *dateMetricIDCache, concurrent bool) error {
	type dmk struct {
		generation uint64
		date       uint64
		metricID   uint64
	}
	m := make(map[dmk]bool)
	for i := 0; i < 1e5; i++ {
		generation := uint64(i) % 2
		date := uint64(i) % 2
		metricID := uint64(i) % 1237
		if !concurrent && c.Has(generation, date, metricID) {
			if !m[dmk{generation, date, metricID}] {
				return fmt.Errorf("c.Has(%d, %d, %d) must return false, but returned true", generation, date, metricID)
			}
			continue
		}
		c.Set(generation, date, metricID)
		m[dmk{generation, date, metricID}] = true
		if !concurrent && !c.Has(generation, date, metricID) {
			return fmt.Errorf("c.Has(%d, %d, %d) must return true, but returned false", generation, date, metricID)
		}
		if i%11234 == 0 {
			c.mu.Lock()
			c.syncLocked()
			c.mu.Unlock()
		}
		if i%34323 == 0 {
			c.mu.Lock()
			c.resetLocked()
			c.mu.Unlock()
			m = make(map[dmk]bool)
		}
	}

	// Verify fast path after sync.
	for i := 0; i < 1e5; i++ {
		generation := uint64(i) % 2
		date := uint64(i) % 2
		metricID := uint64(i) % 123
		c.Set(generation, date, metricID)
	}
	c.mu.Lock()
	c.syncLocked()
	c.mu.Unlock()
	for i := 0; i < 1e5; i++ {
		generation := uint64(i) % 2
		date := uint64(i) % 2
		metricID := uint64(i) % 123
		if !concurrent && !c.Has(generation, date, metricID) {
			return fmt.Errorf("c.Has(%d, %d, %d) must return true after sync", generation, date, metricID)
		}
	}

	// Verify c.Reset
	if n := c.EntriesCount(); !concurrent && n < 123 {
		return fmt.Errorf("c.EntriesCount must return at least 123; returned %d", n)
	}
	c.mu.Lock()
	c.resetLocked()
	c.mu.Unlock()
	if n := c.EntriesCount(); !concurrent && n > 0 {
		return fmt.Errorf("c.EntriesCount must return 0 after reset; returned %d", n)
	}
	return nil
}

func TestDateMetricIDCacheIsConsistent(_ *testing.T) {
	const (
		generation  = 1
		date        = 1
		concurrency = 2
		numMetrics  = 100000
	)
	dmc := newDateMetricIDCache()
	var wg sync.WaitGroup
	for i := range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := uint64(i * numMetrics); id < uint64((i+1)*numMetrics); id++ {
				dmc.Set(generation, date, id)
				if !dmc.Has(generation, date, id) {
					panic(fmt.Errorf("dmc.Has(metricID=%d): unexpected cache miss after adding the entry to cache", id))
				}
			}
		}()
	}
	wg.Wait()
}

func TestUpdateCurrHourMetricIDs(t *testing.T) {
	newStorage := func() *Storage {
		var s Storage
		s.currHourMetricIDs.Store(&hourMetricIDs{})
		s.prevHourMetricIDs.Store(&hourMetricIDs{})
		s.pendingHourEntries = &uint64set.Set{}
		return &s
	}
	t.Run("empty_pending_metric_ids_stale_curr_hour", func(t *testing.T) {
		s := newStorage()
		hour := fasttime.UnixHour()
		if hour%24 == 0 {
			hour++
		}
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: hour - 1,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs(hour)
		hmCurr := s.currHourMetricIDs.Load()
		if hmCurr.hour != hour {
			t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
		}
		if hmCurr.m.Len() != 0 {
			t.Fatalf("unexpected length of hm.m; got %d; want %d", hmCurr.m.Len(), 0)
		}

		hmPrev := s.prevHourMetricIDs.Load()
		if !reflect.DeepEqual(hmPrev, hmOrig) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmOrig)
		}

		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
	t.Run("empty_pending_metric_ids_valid_curr_hour", func(t *testing.T) {
		s := newStorage()
		hour := fasttime.UnixHour()
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: hour,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs(hour)
		hmCurr := s.currHourMetricIDs.Load()
		if hmCurr.hour != hour {
			t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
		}
		if !reflect.DeepEqual(hmCurr, hmOrig) {
			t.Fatalf("unexpected hmCurr; got %v; want %v", hmCurr, hmOrig)
		}

		hmPrev := s.prevHourMetricIDs.Load()
		hmEmpty := &hourMetricIDs{}
		if !reflect.DeepEqual(hmPrev, hmEmpty) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmEmpty)
		}

		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_stale_curr_hour", func(t *testing.T) {
		s := newStorage()
		pendingHourEntries := &uint64set.Set{}
		pendingHourEntries.Add(343)
		pendingHourEntries.Add(32424)
		pendingHourEntries.Add(8293432)
		s.pendingHourEntries = pendingHourEntries

		hour := fasttime.UnixHour()
		if hour%24 == 0 {
			hour++
		}
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: hour - 1,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs(hour)
		hmCurr := s.currHourMetricIDs.Load()
		if hmCurr.hour != hour {
			t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
		}
		if !hmCurr.m.Equal(pendingHourEntries) {
			t.Fatalf("unexpected hmCurr.m; got %v; want %v", hmCurr.m, pendingHourEntries)
		}

		hmPrev := s.prevHourMetricIDs.Load()
		if !reflect.DeepEqual(hmPrev, hmOrig) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmOrig)
		}

		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_valid_curr_hour", func(t *testing.T) {
		s := newStorage()
		pendingHourEntries := &uint64set.Set{}
		pendingHourEntries.Add(343)
		pendingHourEntries.Add(32424)
		pendingHourEntries.Add(8293432)
		s.pendingHourEntries = pendingHourEntries

		hour := fasttime.UnixHour()
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: hour,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs(hour)
		hmCurr := s.currHourMetricIDs.Load()
		if hmCurr.hour != hour {
			t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
		}
		m := pendingHourEntries.Clone()
		hmOrig.m.ForEach(func(part []uint64) bool {
			for _, metricID := range part {
				m.Add(metricID)
			}
			return true
		})
		if !hmCurr.m.Equal(m) {
			t.Fatalf("unexpected hm.m; got %v; want %v", hmCurr.m, m)
		}

		hmPrev := s.prevHourMetricIDs.Load()
		hmEmpty := &hourMetricIDs{}
		if !reflect.DeepEqual(hmPrev, hmEmpty) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmEmpty)
		}

		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_valid_curr_hour_start_of_day", func(t *testing.T) {
		s := newStorage()
		pendingHourEntries := &uint64set.Set{}
		pendingHourEntries.Add(343)
		pendingHourEntries.Add(32424)
		pendingHourEntries.Add(8293432)
		s.pendingHourEntries = pendingHourEntries

		hour := fasttime.UnixHour()
		hour -= hour % 24
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: hour,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs(hour)
		hmCurr := s.currHourMetricIDs.Load()
		if hmCurr.hour != hour {
			t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
		}
		m := pendingHourEntries.Clone()
		hmOrig.m.ForEach(func(part []uint64) bool {
			for _, metricID := range part {
				m.Add(metricID)
			}
			return true
		})
		if !hmCurr.m.Equal(m) {
			t.Fatalf("unexpected hm.m; got %v; want %v", hmCurr.m, m)
		}

		hmPrev := s.prevHourMetricIDs.Load()
		hmEmpty := &hourMetricIDs{}
		if !reflect.DeepEqual(hmPrev, hmEmpty) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmEmpty)
		}

		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_from_previous_hour_new_day", func(t *testing.T) {
		s := newStorage()

		hour := fasttime.UnixHour()
		hour -= hour % 24

		pendingHourEntries := &uint64set.Set{}
		pendingHourEntries.Add(343)
		pendingHourEntries.Add(32424)
		pendingHourEntries.Add(8293432)
		s.pendingHourEntries = pendingHourEntries

		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: hour - 1,
		}
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs(hour)
		hmCurr := s.currHourMetricIDs.Load()
		if hmCurr.hour != hour {
			t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
		}
		if hmCurr.m.Len() != 0 {
			t.Fatalf("unexpected non-empty hmCurr.m; got %v", hmCurr.m.AppendTo(nil))
		}
		hmPrev := s.prevHourMetricIDs.Load()
		if !reflect.DeepEqual(hmPrev, hmOrig) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmOrig)
		}
		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
}

func TestMetricRowMarshalUnmarshal(t *testing.T) {
	var buf []byte
	typ := reflect.TypeOf(&MetricRow{})
	rng := rand.New(rand.NewSource(1))

	for i := 0; i < 1000; i++ {
		v, ok := quick.Value(typ, rng)
		if !ok {
			t.Fatalf("cannot create random MetricRow via quick.Value")
		}
		mr1 := v.Interface().(*MetricRow)
		if mr1 == nil {
			continue
		}

		buf = mr1.Marshal(buf[:0])
		var mr2 MetricRow
		tail, err := mr2.UnmarshalX(buf)
		if err != nil {
			t.Fatalf("cannot unmarshal mr1=%s: %s", mr1, err)
		}
		if len(tail) > 0 {
			t.Fatalf("non-empty tail returned after MetricRow.Unmarshal for mr1=%s", mr1)
		}
		if mr1.MetricNameRaw == nil {
			mr1.MetricNameRaw = []byte{}
		}
		if mr2.MetricNameRaw == nil {
			mr2.MetricNameRaw = []byte{}
		}
		if !reflect.DeepEqual(mr1, &mr2) {
			t.Fatalf("mr1 should match mr2; got\nmr1=%s\nmr2=%s", mr1, &mr2)
		}
	}
}

func TestNextRetentionDeadlineSeconds(t *testing.T) {
	f := func(currentTime string, retention, offset time.Duration, deadlineExpected string) {
		t.Helper()

		now, err := time.Parse(time.RFC3339, currentTime)
		if err != nil {
			t.Fatalf("cannot parse currentTime=%q: %s", currentTime, err)
		}

		d := nextRetentionDeadlineSeconds(now.Unix(), int64(retention.Seconds()), int64(offset.Seconds()))
		deadline := time.Unix(d, 0).UTC().Format(time.RFC3339)
		if deadline != deadlineExpected {
			t.Fatalf("unexpected deadline; got %s; want %s", deadline, deadlineExpected)
		}
	}

	f("2023-07-22T12:44:35Z", 24*time.Hour, 0, "2023-07-23T04:00:00Z")
	f("2023-07-22T03:44:35Z", 24*time.Hour, 0, "2023-07-22T04:00:00Z")
	f("2023-07-22T04:44:35Z", 24*time.Hour, 0, "2023-07-23T04:00:00Z")
	f("2023-07-22T23:44:35Z", 24*time.Hour, 0, "2023-07-23T04:00:00Z")
	f("2023-07-23T03:59:35Z", 24*time.Hour, 0, "2023-07-23T04:00:00Z")

	f("2023-07-22T12:44:35Z", 24*time.Hour, 2*time.Hour, "2023-07-23T02:00:00Z")
	f("2023-07-22T01:44:35Z", 24*time.Hour, 2*time.Hour, "2023-07-22T02:00:00Z")
	f("2023-07-22T02:44:35Z", 24*time.Hour, 2*time.Hour, "2023-07-23T02:00:00Z")
	f("2023-07-22T23:44:35Z", 24*time.Hour, 2*time.Hour, "2023-07-23T02:00:00Z")
	f("2023-07-23T01:59:35Z", 24*time.Hour, 2*time.Hour, "2023-07-23T02:00:00Z")

	f("2023-07-22T12:44:35Z", 24*time.Hour, -5*time.Hour, "2023-07-23T09:00:00Z")
	f("2023-07-22T08:44:35Z", 24*time.Hour, -5*time.Hour, "2023-07-22T09:00:00Z")
	f("2023-07-22T09:44:35Z", 24*time.Hour, -5*time.Hour, "2023-07-23T09:00:00Z")

	f("2023-07-22T12:44:35Z", 24*time.Hour, -12*time.Hour, "2023-07-22T16:00:00Z")
	f("2023-07-22T15:44:35Z", 24*time.Hour, -12*time.Hour, "2023-07-22T16:00:00Z")
	f("2023-07-22T16:44:35Z", 24*time.Hour, -12*time.Hour, "2023-07-23T16:00:00Z")

	f("2023-07-22T12:44:35Z", 24*time.Hour, -18*time.Hour, "2023-07-22T22:00:00Z")
	f("2023-07-22T21:44:35Z", 24*time.Hour, -18*time.Hour, "2023-07-22T22:00:00Z")
	f("2023-07-22T22:44:35Z", 24*time.Hour, -18*time.Hour, "2023-07-23T22:00:00Z")

	f("2023-07-22T12:44:35Z", 24*time.Hour, 18*time.Hour, "2023-07-23T10:00:00Z")
	f("2023-07-22T09:44:35Z", 24*time.Hour, 18*time.Hour, "2023-07-22T10:00:00Z")
	f("2023-07-22T10:44:35Z", 24*time.Hour, 18*time.Hour, "2023-07-23T10:00:00Z")

	f("2023-07-22T12:44:35Z", 24*time.Hour, 37*time.Hour, "2023-07-22T15:00:00Z")
	f("2023-07-22T14:44:35Z", 24*time.Hour, 37*time.Hour, "2023-07-22T15:00:00Z")
	f("2023-07-22T15:44:35Z", 24*time.Hour, 37*time.Hour, "2023-07-23T15:00:00Z")

	// The test cases below confirm that it is possible to pick a retention
	// period such that the previous IndexDB may be removed earlier than it should be.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7609

	// Cluster is configured with 12 month retentionPeriod on 2023-01-01.
	f("2023-01-01T00:00:00Z", 365*24*time.Hour, 0, "2023-12-19T04:00:00Z")

	// Restarts during that period do not change the retention deadline:
	f("2023-03-01T00:00:00Z", 365*24*time.Hour, 0, "2023-12-19T04:00:00Z")
	f("2023-06-01T00:00:00Z", 365*24*time.Hour, 0, "2023-12-19T04:00:00Z")
	f("2023-09-01T00:00:00Z", 365*24*time.Hour, 0, "2023-12-19T04:00:00Z")
	f("2023-12-01T00:00:00Z", 365*24*time.Hour, 0, "2023-12-19T04:00:00Z")
	f("2023-12-19T03:59:59Z", 365*24*time.Hour, 0, "2023-12-19T04:00:00Z")

	// At 2023-12-19T04:00:00Z the rotation occurs. New deadline is
	// 2024-12-18T04:00:00Z. Restarts during that period do not change the
	// new deadline:
	f("2023-12-19T04:00:01Z", 365*24*time.Hour, 0, "2024-12-18T04:00:00Z")
	f("2024-01-01T00:00:00Z", 365*24*time.Hour, 0, "2024-12-18T04:00:00Z")
	f("2024-03-01T00:00:00Z", 365*24*time.Hour, 0, "2024-12-18T04:00:00Z")
	f("2024-04-29T00:00:00Z", 365*24*time.Hour, 0, "2024-12-18T04:00:00Z")

	// Now restart again but with the new retention period of 451d and the
	// rotation time becomes 2024-05-01T04:00:00Z.
	//
	// At 2024-05-01T04:00:00Z, a new IndexDB is created and the current
	// IndexDB (currently applicable to only ~4 months of data) becomes the
	// previous IndexDB.  The preceding IndexDB is deleted despite possibly
	// being related to ~8 months of data that is still within retention.
	f("2024-04-29T00:00:00Z", 451*24*time.Hour, 0, "2024-05-01T04:00:00Z")
}

func TestStorageOpenClose(t *testing.T) {
	path := "TestStorageOpenClose"
	opts := OpenOptions{
		Retention:       -1,
		MaxHourlySeries: 1e5,
		MaxDailySeries:  1e6,
	}
	for i := 0; i < 10; i++ {
		s := MustOpenStorage(path, opts)
		s.MustClose()
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func TestStorageRandTimestamps(t *testing.T) {
	path := "TestStorageRandTimestamps"
	opts := OpenOptions{
		Retention: 10 * retention31Days,
	}
	s := MustOpenStorage(path, opts)
	t.Run("serial", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if err := testStorageRandTimestamps(s); err != nil {
				t.Fatalf("error on iteration %d: %s", i, err)
			}
			s.MustClose()
			s = MustOpenStorage(path, opts)
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		ch := make(chan error, 3)
		for i := 0; i < cap(ch); i++ {
			go func() {
				var err error
				for i := 0; i < 2; i++ {
					err = testStorageRandTimestamps(s)
				}
				ch <- err
			}()
		}
		tt := time.NewTimer(time.Second * 10)
		for i := 0; i < cap(ch); i++ {
			select {
			case err := <-ch:
				if err != nil {
					t.Fatalf("error on iteration %d: %s", i, err)
				}
			case <-tt.C:
				t.Fatalf("timeout on iteration %d", i)
			}
		}
	})
	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func testStorageRandTimestamps(s *Storage) error {
	currentTime := timestampFromTime(time.Now())
	const rowsPerAdd = 5e3
	const addsCount = 3
	rng := rand.New(rand.NewSource(1))

	for i := 0; i < addsCount; i++ {
		var mrs []MetricRow
		var mn MetricName
		mn.Tags = []Tag{
			{[]byte("job"), []byte("webservice")},
			{[]byte("instance"), []byte("1.2.3.4")},
		}
		for j := 0; j < rowsPerAdd; j++ {
			mn.MetricGroup = []byte(fmt.Sprintf("metric_%d", rng.Intn(100)))
			metricNameRaw := mn.marshalRaw(nil)
			timestamp := currentTime - int64((rng.Float64()-0.2)*float64(2*s.retentionMsecs))
			value := rng.NormFloat64() * 1e11

			mr := MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     timestamp,
				Value:         value,
			}
			mrs = append(mrs, mr)
		}
		s.AddRows(mrs, defaultPrecisionBits)
	}

	// Verify the storage contains rows.
	var m Metrics
	s.UpdateMetrics(&m)
	if rowsCount := m.TableMetrics.TotalRowsCount(); rowsCount == 0 {
		return fmt.Errorf("expecting at least one row in storage")
	}
	return nil
}

func TestStorageDeleteSeries(t *testing.T) {
	path := "TestStorageDeleteSeries"
	s := MustOpenStorage(path, OpenOptions{})

	// Verify no label names exist
	lns, err := s.SearchLabelNames(nil, nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("error in SearchLabelNames() at the start: %s", err)
	}
	if len(lns) != 0 {
		t.Fatalf("found non-empty tag keys at the start: %q", lns)
	}

	t.Run("serial", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if err = testStorageDeleteSeries(s, 0); err != nil {
				t.Fatalf("unexpected error on iteration %d: %s", i, err)
			}

			// Re-open the storage in order to check how deleted metricIDs
			// are persisted.
			s.MustClose()
			s = MustOpenStorage(path, OpenOptions{})
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		ch := make(chan error, 3)
		for i := 0; i < cap(ch); i++ {
			go func(workerNum int) {
				var err error
				for j := 0; j < 2; j++ {
					err = testStorageDeleteSeries(s, workerNum)
					if err != nil {
						break
					}
				}
				ch <- err
			}(i)
		}
		tt := time.NewTimer(30 * time.Second)
		for i := 0; i < cap(ch); i++ {
			select {
			case err := <-ch:
				if err != nil {
					t.Fatalf("unexpected error on iteration %d: %s", i, err)
				}
			case <-tt.C:
				t.Fatalf("timeout on iteration %d", i)
			}
		}
	})

	// Verify no more tag keys exist
	lns, err = s.SearchLabelNames(nil, nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("error in SearchLabelNames after the test: %s", err)
	}
	if len(lns) != 0 {
		t.Fatalf("found non-empty tag keys after the test: %q", lns)
	}

	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func testStorageDeleteSeries(s *Storage, workerNum int) error {
	rng := rand.New(rand.NewSource(1))
	const rowsPerMetric = 100
	const metricsCount = 30

	workerTag := []byte(fmt.Sprintf("workerTag_%d", workerNum))

	lnsAll := make(map[string]bool)
	lnsAll["__name__"] = true
	for i := 0; i < metricsCount; i++ {
		var mrs []MetricRow
		var mn MetricName
		job := fmt.Sprintf("job_%d_%d", i, workerNum)
		instance := fmt.Sprintf("instance_%d_%d", i, workerNum)
		mn.Tags = []Tag{
			{[]byte("job"), []byte(job)},
			{[]byte("instance"), []byte(instance)},
			{workerTag, []byte("foobar")},
		}
		for i := range mn.Tags {
			lnsAll[string(mn.Tags[i].Key)] = true
		}
		mn.MetricGroup = []byte(fmt.Sprintf("metric_%d_%d", i, workerNum))
		metricNameRaw := mn.marshalRaw(nil)

		for j := 0; j < rowsPerMetric; j++ {
			timestamp := rng.Int63n(1e10)
			value := rng.NormFloat64() * 1e6

			mr := MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     timestamp,
				Value:         value,
			}
			mrs = append(mrs, mr)
		}
		s.AddRows(mrs, defaultPrecisionBits)
	}
	s.DebugFlush()

	// Verify tag values exist
	tvs, err := s.SearchLabelValues(nil, string(workerTag), nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelValues before metrics removal: %w", err)
	}
	if len(tvs) == 0 {
		return fmt.Errorf("unexpected empty number of tag values for workerTag")
	}

	// Verify tag keys exist
	lns, err := s.SearchLabelNames(nil, nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelNames before metrics removal: %w", err)
	}
	if err := checkLabelNames(lns, lnsAll); err != nil {
		return fmt.Errorf("unexpected label names before metrics removal: %w", err)
	}

	var sr Search
	tr := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: 2e10,
	}
	metricBlocksCount := func(tfs *TagFilters) int {
		// Verify the number of blocks
		n := 0
		sr.Init(nil, s, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		for sr.NextMetricBlock() {
			n++
		}
		sr.MustClose()
		return n
	}
	for i := 0; i < metricsCount; i++ {
		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte("metric_.+"), false, true); err != nil {
			return fmt.Errorf("cannot add regexp tag filter: %w", err)
		}
		job := fmt.Sprintf("job_%d_%d", i, workerNum)
		if err := tfs.Add([]byte("job"), []byte(job), false, false); err != nil {
			return fmt.Errorf("cannot add job tag filter: %w", err)
		}
		if n := metricBlocksCount(tfs); n == 0 {
			return fmt.Errorf("expecting non-zero number of metric blocks for tfs=%s", tfs)
		}
		deletedCount, err := s.DeleteSeries(nil, []*TagFilters{tfs}, 1e9)
		if err != nil {
			return fmt.Errorf("cannot delete metrics: %w", err)
		}
		if deletedCount == 0 {
			return fmt.Errorf("expecting non-zero number of deleted metrics on iteration %d", i)
		}
		if n := metricBlocksCount(tfs); n != 0 {
			return fmt.Errorf("expecting zero metric blocks after DeleteSeries call for tfs=%s; got %d blocks", tfs, n)
		}

		// Try deleting empty tfss
		deletedCount, err = s.DeleteSeries(nil, nil, 1e9)
		if err != nil {
			return fmt.Errorf("cannot delete empty tfss: %w", err)
		}
		if deletedCount != 0 {
			return fmt.Errorf("expecting zero deleted metrics for empty tfss; got %d", deletedCount)
		}
	}

	// Make sure no more metrics left for the given workerNum
	tfs := NewTagFilters()
	if err := tfs.Add(nil, []byte(fmt.Sprintf("metric_.+_%d", workerNum)), false, true); err != nil {
		return fmt.Errorf("cannot add regexp tag filter for worker metrics: %w", err)
	}
	if n := metricBlocksCount(tfs); n != 0 {
		return fmt.Errorf("expecting zero metric blocks after deleting all the metrics; got %d blocks", n)
	}
	tvs, err = s.SearchLabelValues(nil, string(workerTag), nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelValues after all the metrics are removed: %w", err)
	}
	if len(tvs) != 0 {
		return fmt.Errorf("found non-empty tag values for %q after metrics removal: %q", workerTag, tvs)
	}

	return nil
}

func checkLabelNames(lns []string, lnsExpected map[string]bool) error {
	if len(lns) < len(lnsExpected) {
		return fmt.Errorf("unexpected number of label names found; got %d; want at least %d; lns=%q, lnsExpected=%v", len(lns), len(lnsExpected), lns, lnsExpected)
	}
	hasItem := func(s string, lns []string) bool {
		for _, labelName := range lns {
			if s == labelName {
				return true
			}
		}
		return false
	}
	for labelName := range lnsExpected {
		if !hasItem(labelName, lns) {
			return fmt.Errorf("cannot find %q in label names %q", labelName, lns)
		}
	}
	return nil
}

func TestStorageDeleteSeries_TooManyTimeseries(t *testing.T) {
	defer testRemoveAll(t)

	const numSeries = 1000
	rng := rand.New(rand.NewSource(1))
	mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
		MinTimestamp: time.Now().Add(-100 * 24 * time.Hour).UnixMilli(),
		MaxTimestamp: time.Now().UnixMilli(),
	})

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	tfs := NewTagFilters()
	if err := tfs.Add(nil, []byte("metric.*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	maxSeries := numSeries - 1
	count, err := s.DeleteSeries(nil, []*TagFilters{tfs}, maxSeries)
	if err == nil {
		t.Errorf("expected an error but there hasn't been one")
	}
	if count != 0 {
		t.Errorf("unexpected deleted series count: got %d, want 0", count)
	}
}

func TestStorageDeleteSeries_CachesAreUpdatedOrReset(t *testing.T) {
	defer testRemoveAll(t)

	// For this test, the time range must be < 40 days. Otherwise the global
	// index will be searched and the actual time range used in tag filters
	// cache will be the globalTimeRange.
	tr := TimeRange{
		MinTimestamp: time.Now().Add(-30 * 24 * time.Hour).UnixMilli(),
		MaxTimestamp: time.Now().UnixMilli(),
	}
	mn := MetricName{MetricGroup: []byte("metric")}
	mr := MetricRow{
		MetricNameRaw: mn.marshalRaw(nil),
		Timestamp:     tr.MaxTimestamp,
		Value:         123,
	}
	var (
		genTSID generationTSID
		tfssKey []byte
	)
	tfs := NewTagFilters()
	if err := tfs.Add(nil, []byte("metric.*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	tfss := []*TagFilters{tfs}
	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	idb, putIndexDB := s.getCurrIndexDB()
	defer putIndexDB()

	// Ensure caches are empty.
	if s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
		t.Fatalf("tsidCache unexpected contents: got %v, want empty", genTSID)
	}
	tfssKey = marshalTagFiltersKey(nil, tfss, tr, true)
	if got, ok := idb.getMetricIDsFromTagFiltersCache(nil, tfssKey); ok {
		t.Fatalf("tagFiltersToMetricIDsCache unexpected contents: got %v, want empty", got)
	}
	if got := s.getDeletedMetricIDs().Len(); got != 0 {
		t.Fatalf("deletedMetricIDs cache: unexpected size: got %d, want empty", got)
	}

	// Add one row, search it, and ensure that the tsidCache and
	// tagFiltersToMetricIDsCache are not empty but the deletedMetricIDs
	// cache is still empty.
	s.AddRows([]MetricRow{mr}, defaultPrecisionBits)
	s.DebugFlush()
	gotMetrics, err := s.SearchMetricNames(nil, tfss, tr, 1, noDeadline)
	if err != nil {
		t.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
	}
	wantMetrics := []string{string(mr.MetricNameRaw)}
	if reflect.DeepEqual(gotMetrics, wantMetrics) {
		t.Fatalf("SearchMetricNames() unexpected search result: got %v, want %v", gotMetrics, wantMetrics)
	}

	if !s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
		t.Fatalf("tsidCache was expected to contain a record but it did not")
	}
	metricID := genTSID.TSID.MetricID
	tfssKey = marshalTagFiltersKey(nil, tfss, tr, true)
	if _, ok := idb.getMetricIDsFromTagFiltersCache(nil, tfssKey); !ok {
		t.Fatalf("tagFiltersToMetricIDsCache was expected to contain a record but it did not")
	}
	if got := s.getDeletedMetricIDs().Len(); got != 0 {
		t.Fatalf("deletedMetricIDs cache unexpected size: got %d, want empty", got)
	}

	// Delete the metric added earlier and ensure that the tsidCache and
	// tagFiltersToMetricIDsCache have been reset and the deletedMetricIDs
	// cache is now contains ID of the deleted metric.
	numDeletedSeries, err := s.DeleteSeries(nil, tfss, 1)
	if err != nil {
		t.Fatalf("DeleteSeries() failed unexpectedly: %v", err)
	}
	if got, want := numDeletedSeries, 1; got != want {
		t.Fatalf("unexpected number of deleted series, got %d, want %d", got, want)
	}
	if s.getTSIDFromCache(&genTSID, mr.MetricNameRaw) {
		t.Fatalf("tsidCache unexpected contents: got %v, want empty", genTSID)
	}
	tfssKey = marshalTagFiltersKey(nil, tfss, tr, true)
	if got, ok := idb.getMetricIDsFromTagFiltersCache(nil, tfssKey); ok {
		t.Fatalf("tagFiltersToMetricIDsCache unexpected contents: got %v, want empty", got)
	}
	if got, want := s.getDeletedMetricIDs().AppendTo(nil), []uint64{metricID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("deletedMetricIDs cache: unexpected contents: got %v, want %v", got, want)
	}
}

func TestStorageRegisterMetricNamesSerial(t *testing.T) {
	path := "TestStorageRegisterMetricNamesSerial"
	s := MustOpenStorage(path, OpenOptions{})
	if err := testStorageRegisterMetricNames(s); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func TestStorageRegisterMetricNamesConcurrent(t *testing.T) {
	path := "TestStorageRegisterMetricNamesConcurrent"
	s := MustOpenStorage(path, OpenOptions{})
	ch := make(chan error, 3)
	for i := 0; i < cap(ch); i++ {
		go func() {
			ch <- testStorageRegisterMetricNames(s)
		}()
	}
	for i := 0; i < cap(ch); i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout")
		}
	}
	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func testStorageRegisterMetricNames(s *Storage) error {
	const metricsPerAdd = 1e3
	const addsCount = 10

	addIDsMap := make(map[string]struct{})
	for i := 0; i < addsCount; i++ {
		var mrs []MetricRow
		var mn MetricName
		addID := fmt.Sprintf("%d", i)
		addIDsMap[addID] = struct{}{}
		mn.Tags = []Tag{
			{[]byte("job"), []byte("webservice")},
			{[]byte("instance"), []byte("1.2.3.4")},
			{[]byte("add_id"), []byte(addID)},
		}
		now := timestampFromTime(time.Now())
		for j := 0; j < metricsPerAdd; j++ {
			mn.MetricGroup = []byte(fmt.Sprintf("metric_%d", j))
			metricNameRaw := mn.marshalRaw(nil)

			mr := MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     now,
			}
			mrs = append(mrs, mr)
		}
		s.RegisterMetricNames(nil, mrs)
	}
	var addIDsExpected []string
	for k := range addIDsMap {
		addIDsExpected = append(addIDsExpected, k)
	}
	sort.Strings(addIDsExpected)

	// Verify the storage contains the added metric names.
	s.DebugFlush()

	// Verify that SearchLabelNames returns correct result.
	lnsExpected := []string{
		"__name__",
		"add_id",
		"instance",
		"job",
	}
	lns, err := s.SearchLabelNames(nil, nil, TimeRange{}, 100, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelNames: %w", err)
	}
	sort.Strings(lns)
	if !reflect.DeepEqual(lns, lnsExpected) {
		return fmt.Errorf("unexpected label names returned from SearchLabelNames;\ngot\n%q\nwant\n%q", lns, lnsExpected)
	}

	// Verify that SearchLabelNames with the specified time range returns correct result.
	now := timestampFromTime(time.Now())
	start := now - msecPerDay
	end := now + 60*1000
	tr := TimeRange{
		MinTimestamp: start,
		MaxTimestamp: end,
	}
	lns, err = s.SearchLabelNames(nil, nil, tr, 100, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelNames: %w", err)
	}
	sort.Strings(lns)
	if !reflect.DeepEqual(lns, lnsExpected) {
		return fmt.Errorf("unexpected label names returned from SearchLabelNames;\ngot\n%q\nwant\n%q", lns, lnsExpected)
	}

	// Verify that SearchLabelValues returns correct result.
	addIDs, err := s.SearchLabelValues(nil, "add_id", nil, TimeRange{}, addsCount+100, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelValues: %w", err)
	}
	sort.Strings(addIDs)
	if !reflect.DeepEqual(addIDs, addIDsExpected) {
		return fmt.Errorf("unexpected tag values returned from SearchLabelValues;\ngot\n%q\nwant\n%q", addIDs, addIDsExpected)
	}

	// Verify that SearchLabelValues with the specified time range returns correct result.
	addIDs, err = s.SearchLabelValues(nil, "add_id", nil, tr, addsCount+100, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelValues: %w", err)
	}
	sort.Strings(addIDs)
	if !reflect.DeepEqual(addIDs, addIDsExpected) {
		return fmt.Errorf("unexpected tag values returned from SearchLabelValues;\ngot\n%q\nwant\n%q", addIDs, addIDsExpected)
	}

	// Verify that SearchMetricNames returns correct result.
	tfs := NewTagFilters()
	if err := tfs.Add([]byte("add_id"), []byte("0"), false, false); err != nil {
		return fmt.Errorf("unexpected error in TagFilters.Add: %w", err)
	}
	metricNames, err := s.SearchMetricNames(nil, []*TagFilters{tfs}, tr, metricsPerAdd*addsCount*100+100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchMetricNames: %w", err)
	}
	if len(metricNames) < metricsPerAdd {
		return fmt.Errorf("unexpected number of metricNames returned from SearchMetricNames; got %d; want at least %d", len(metricNames), int(metricsPerAdd))
	}
	var mn MetricName
	for i, metricName := range metricNames {
		if err := mn.UnmarshalString(metricName); err != nil {
			return fmt.Errorf("cannot unmarshal metricName=%q: %w", metricName, err)
		}
		addID := mn.GetTagValue("add_id")
		if string(addID) != "0" {
			return fmt.Errorf("unexpected addID for metricName #%d; got %q; want %q", i, addID, "0")
		}
		job := mn.GetTagValue("job")
		if string(job) != "webservice" {
			return fmt.Errorf("unexpected job for metricName #%d; got %q; want %q", i, job, "webservice")
		}
	}

	return nil
}

func TestStorageAddRowsSerial(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	path := "TestStorageAddRowsSerial"
	opts := OpenOptions{
		Retention:       10 * retention31Days,
		MaxHourlySeries: 1e5,
		MaxDailySeries:  1e5,
	}
	s := MustOpenStorage(path, opts)
	if err := testStorageAddRows(rng, s); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func TestStorageAddRowsConcurrent(t *testing.T) {
	path := "TestStorageAddRowsConcurrent"
	opts := OpenOptions{
		Retention:       10 * retention31Days,
		MaxHourlySeries: 1e5,
		MaxDailySeries:  1e5,
	}
	s := MustOpenStorage(path, opts)
	ch := make(chan error, 3)
	for i := 0; i < cap(ch); i++ {
		go func(n int) {
			rLocal := rand.New(rand.NewSource(int64(n)))
			ch <- testStorageAddRows(rLocal, s)
		}(i)
	}
	for i := 0; i < cap(ch); i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout")
		}
	}
	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func testGenerateMetricRows(rng *rand.Rand, rows uint64, timestampMin, timestampMax int64) []MetricRow {
	return testGenerateMetricRowsWithPrefix(rng, rows, "metric", TimeRange{timestampMin, timestampMax})
}

func testGenerateMetricRowsWithPrefix(rng *rand.Rand, rows uint64, prefix string, tr TimeRange) []MetricRow {
	var mrs []MetricRow
	var mn MetricName
	mn.Tags = []Tag{
		{[]byte("job"), []byte("webservice")},
		{[]byte("instance"), []byte("1.2.3.4")},
	}
	for i := 0; i < int(rows); i++ {
		mn.MetricGroup = []byte(fmt.Sprintf("%s_%d", prefix, i))
		metricNameRaw := mn.marshalRaw(nil)
		timestamp := rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp
		value := rng.NormFloat64() * 1e6

		mr := MetricRow{
			MetricNameRaw: metricNameRaw,
			Timestamp:     timestamp,
			Value:         value,
		}
		mrs = append(mrs, mr)
	}
	return mrs
}

func testStorageAddRows(rng *rand.Rand, s *Storage) error {
	const rowsPerAdd = 1e3
	const addsCount = 10

	maxTimestamp := timestampFromTime(time.Now())
	minTimestamp := maxTimestamp - s.retentionMsecs + 3600*1000
	for i := 0; i < addsCount; i++ {
		mrs := testGenerateMetricRows(rng, rowsPerAdd, minTimestamp, maxTimestamp)
		s.AddRows(mrs, defaultPrecisionBits)
	}

	// Verify the storage contains rows.
	minRowsExpected := uint64(rowsPerAdd * addsCount)
	var m Metrics
	s.UpdateMetrics(&m)
	if rowsCount := m.TableMetrics.TotalRowsCount(); rowsCount < minRowsExpected {
		return fmt.Errorf("expecting at least %d rows in the table; got %d", minRowsExpected, rowsCount)
	}

	// Try creating a snapshot from the storage.
	snapshotName, err := s.CreateSnapshot()
	if err != nil {
		return fmt.Errorf("cannot create snapshot from the storage: %w", err)
	}

	// Verify the snapshot is visible
	snapshots, err := s.ListSnapshots()
	if err != nil {
		return fmt.Errorf("cannot list snapshots: %w", err)
	}
	if !containsString(snapshots, snapshotName) {
		return fmt.Errorf("cannot find snapshot %q in %q", snapshotName, snapshots)
	}

	// Try opening the storage from snapshot.
	snapshotPath := filepath.Join(s.path, snapshotsDirname, snapshotName)
	s1 := MustOpenStorage(snapshotPath, OpenOptions{})

	// Verify the snapshot contains rows
	var m1 Metrics
	s1.UpdateMetrics(&m1)
	if rowsCount := m1.TableMetrics.TotalRowsCount(); rowsCount < minRowsExpected {
		return fmt.Errorf("snapshot %q must contain at least %d rows; got %d", snapshotPath, minRowsExpected, rowsCount)
	}

	// Verify that force merge for the snapshot leaves at most a single part per partition.
	// Zero parts are possible if the snapshot is created just after the partition has been created
	// by concurrent goroutine, but it didn't put the data into it yet.
	if err := s1.ForceMergePartitions(""); err != nil {
		return fmt.Errorf("error when force merging partitions: %w", err)
	}
	ptws := s1.tb.GetPartitions(nil)
	for _, ptw := range ptws {
		pws := ptw.pt.GetParts(nil, true)
		numParts := len(pws)
		ptw.pt.PutParts(pws)
		if numParts > 1 {
			s1.tb.PutPartitions(ptws)
			return fmt.Errorf("unexpected number of parts for partition %q after force merge; got %d; want at most 1", ptw.pt.name, numParts)
		}
	}
	s1.tb.PutPartitions(ptws)

	s1.MustClose()

	// Delete the snapshot and make sure it is no longer visible.
	if err := s.DeleteSnapshot(snapshotName); err != nil {
		return fmt.Errorf("cannot delete snapshot %q: %w", snapshotName, err)
	}
	snapshots, err = s.ListSnapshots()
	if err != nil {
		return fmt.Errorf("cannot list snapshots: %w", err)
	}
	if containsString(snapshots, snapshotName) {
		return fmt.Errorf("snapshot %q must be deleted, but is still visible in %q", snapshotName, snapshots)
	}

	return nil
}

func TestStorageRotateIndexDB(t *testing.T) {
	defer testRemoveAll(t)

	const (
		numRotations = 4
		numWorkers   = 10
		numRows      = 10000
	)
	tr := TimeRange{
		MinTimestamp: time.Now().UTC().Add(-numRows * time.Hour).UnixMilli(),
		MaxTimestamp: time.Now().UTC().UnixMilli(),
	}
	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()

	insertAndRotateConcurrently := func(i int) (int, int) {
		var wg sync.WaitGroup
		for workerNum := range numWorkers {
			wg.Add(1)
			go func() {
				time.Sleep(1 * time.Millisecond)
				rng := rand.New(rand.NewSource(1))
				prefix := fmt.Sprintf("metric_%d_%d", i, workerNum)
				mrs := testGenerateMetricRowsWithPrefix(rng, numRows, prefix, tr)
				s.AddRows(mrs, defaultPrecisionBits)
				wg.Done()
			}()
		}
		s.mustRotateIndexDB(time.Now())
		wg.Wait()
		s.DebugFlush()

		idbCurr, putIndexDB := s.getCurrIndexDB()
		defer putIndexDB()
		idbPrev := idbCurr.extDB
		isCurr := idbCurr.getIndexSearch(noDeadline)
		defer idbCurr.putIndexSearch(isCurr)
		isPrev := idbPrev.getIndexSearch(noDeadline)
		defer idbPrev.putIndexSearch(isPrev)

		return testCountAllMetricNamesNoExtDB(isPrev, tr), testCountAllMetricNamesNoExtDB(isCurr, tr)
	}

	var oldCurr int
	for i := range numRotations {
		newPrev, newCurr := insertAndRotateConcurrently(i)

		var m Metrics
		s.UpdateMetrics(&m)
		if got, want := m.TableMetrics.TotalRowsCount(), uint64(numWorkers*numRows*(i+1)); got != want {
			t.Errorf("[rotation %d] unexpected row count: got %d, want %d", i, got, want)
		}

		if got, want := newPrev-oldCurr+newCurr, numWorkers*numRows; got != want {
			t.Errorf("[rotation %d] unexpected metric count count: got (%d - %d) + %d = %d, want %d", i, newPrev, oldCurr, newCurr, got, want)
		}
		oldCurr = newCurr
	}
}

func testCountAllMetricNamesNoExtDB(is *indexSearch, tr TimeRange) int {
	tfss := NewTagFilters()
	if err := tfss.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		panic(fmt.Sprintf("unexpected error in TagFilters.Add: %v", err))
	}
	metricIDs, err := is.searchMetricIDs(nil, []*TagFilters{tfss}, tr, 1e9)
	if err != nil {
		panic(fmt.Sprintf("searchMetricIDs failed unexpectedly: %v", err))
	}
	metricNames := map[string]bool{}
	var metricName []byte
	for _, metricID := range metricIDs {
		metricName, _ = is.searchMetricName(metricName[:0], metricID)
		metricNames[string(metricName)] = true
	}
	return len(metricNames)
}

func TestStorageRotateIndexDB_AddRows(t *testing.T) {
	op := func(s *Storage) {
		rng := rand.New(rand.NewSource(1))
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		}
		mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
	}

	testRotateIndexDB(t, []MetricRow{}, op)
}

func TestStorageRotateIndexDB_RegisterMetricNames(t *testing.T) {
	op := func(s *Storage) {
		rng := rand.New(rand.NewSource(1))
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		}
		mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
		s.RegisterMetricNames(nil, mrs)
		s.DebugFlush()
	}

	testRotateIndexDB(t, []MetricRow{}, op)
}

func TestStorageRotateIndexDB_DeleteSeries(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	tfs := NewTagFilters()
	if err := tfs.Add(nil, []byte("metric.*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	op := func(s *Storage) {
		_, err := s.DeleteSeries(nil, []*TagFilters{tfs}, 1e9)
		if err != nil {
			panic(fmt.Sprintf("DeleteSeries() failed unexpectedly: %v", err))
		}
	}

	testRotateIndexDB(t, mrs, op)
}

func TestStorageRotateIndexDB_CreateSnapshot(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	op := func(s *Storage) {
		_, err := s.CreateSnapshot()
		if err != nil {
			panic(fmt.Sprintf("CreateSnapshot() failed unexpectedly: %v", err))
		}
	}

	testRotateIndexDB(t, mrs, op)
}

func TestStorageRotateIndexDB_SearchMetricNames(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	tfs := NewTagFilters()
	if err := tfs.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	tfss := []*TagFilters{tfs}
	op := func(s *Storage) {
		_, err := s.SearchMetricNames(nil, tfss, tr, 1e9, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchMetricNames() failed unexpectedly: %v", err))
		}
	}

	testRotateIndexDB(t, mrs, op)
}

func TestStorageRotateIndexDB_SearchLabelNames(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)

	testRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.SearchLabelNames(nil, []*TagFilters{}, tr, 1e6, 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchLabelNames() failed unexpectedly: %v", err))
		}
	})
}

func TestStorageRotateIndexDB_SearchLabelValues(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)

	testRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.SearchLabelValues(nil, "__name__", []*TagFilters{}, tr, 1e6, 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchLabelValues() failed unexpectedly: %v", err))
		}
	})
}

func TestStorageRotateIndexDB_SearchTagValueSuffixes(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric.", tr)

	testRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.SearchTagValueSuffixes(nil, tr, "", "metric.", '.', 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchTagValueSuffixes() failed unexpectedly: %v", err))
		}
	})
}

func TestStorageRotateIndexDB_SearchGraphitePaths(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric.", tr)

	testRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.SearchGraphitePaths(nil, tr, []byte("*.*"), 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchGraphitePaths() failed unexpectedly: %v", err))
		}
	})
}

func TestStorageRotateIndexDB_GetSeriesCount(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)

	testRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.GetSeriesCount(noDeadline)
		if err != nil {
			panic(fmt.Sprintf("GetSeriesCount() failed unexpectedly: %v", err))
		}
	})
}

func TestStorageRotateIndexDB_GetTSDBStatus(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	date := uint64(tr.MinTimestamp) / msecPerDay

	testRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.GetTSDBStatus(nil, nil, date, "", 10, 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("GetTSDBStatus failed unexpectedly: %v", err))
		}
	})
}

func TestStorageRotateIndexDB_NotifyReadWriteMode(t *testing.T) {
	op := func(s *Storage) {
		// Set readonly so that the background workers started by
		// notifyReadWriteMode exit early.
		s.isReadOnly.Store(true)
		s.notifyReadWriteMode()
	}

	testRotateIndexDB(t, []MetricRow{}, op)
}

func TestStorageRotateIndexDB_UpdateMetrics(t *testing.T) {
	op := func(s *Storage) {
		s.UpdateMetrics(&Metrics{})
	}

	testRotateIndexDB(t, []MetricRow{}, op)
}

func TestStorageRotateIndexDB_Search(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	tfs := NewTagFilters()
	if err := tfs.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	tfss := []*TagFilters{tfs}

	testRotateIndexDB(t, mrs, func(s *Storage) {
		var search Search
		search.Init(nil, s, tfss, tr, 1e5, noDeadline)
		for search.NextMetricBlock() {
			var b Block
			search.MetricBlockRef.BlockRef.MustReadBlock(&b)
		}
		if err := search.Error(); err != nil {
			panic(fmt.Sprintf("search error: %v", err))
		}
		search.MustClose()
	})
}

// testRotateIndexDB checks that storage handles gracefully indexDB rotation
// that happens concurrently with some operation (ingestion or search). The
// operation is expected to finish successfully and there must be no panics.
func testRotateIndexDB(t *testing.T, mrs []MetricRow, op func(s *Storage)) {
	defer testRemoveAll(t)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for range 100 {
		wg.Add(1)
		go func() {
			for {
				select {
				case <-stop:
					wg.Done()
					return
				default:
				}
				op(s)
			}
		}()
	}

	for range 10 {
		s.mustRotateIndexDB(time.Now())
	}

	close(stop)
	wg.Wait()
}

func TestStorageDeleteStaleSnapshots(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	path := "TestStorageDeleteStaleSnapshots"
	opts := OpenOptions{
		Retention:       10 * retention31Days,
		MaxHourlySeries: 1e5,
		MaxDailySeries:  1e5,
	}
	s := MustOpenStorage(path, opts)
	const rowsPerAdd = 1e3
	const addsCount = 10
	maxTimestamp := timestampFromTime(time.Now())
	minTimestamp := maxTimestamp - s.retentionMsecs
	for i := 0; i < addsCount; i++ {
		mrs := testGenerateMetricRows(rng, rowsPerAdd, minTimestamp, maxTimestamp)
		s.AddRows(mrs, defaultPrecisionBits)
	}
	// Try creating a snapshot from the storage.
	snapshotName, err := s.CreateSnapshot()
	if err != nil {
		t.Fatalf("cannot create snapshot from the storage: %s", err)
	}
	// Delete snapshots older than 1 month
	if err := s.DeleteStaleSnapshots(30 * 24 * time.Hour); err != nil {
		t.Fatalf("error in DeleteStaleSnapshots(1 month): %s", err)
	}
	snapshots, err := s.ListSnapshots()
	if err != nil {
		t.Fatalf("cannot list snapshots: %s", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expecting one snapshot; got %q", snapshots)
	}
	if snapshots[0] != snapshotName {
		t.Fatalf("snapshot %q is missing in %q", snapshotName, snapshots)
	}

	// Delete the snapshot which is older than 1 nanoseconds
	time.Sleep(2 * time.Nanosecond)
	if err := s.DeleteStaleSnapshots(time.Nanosecond); err != nil {
		t.Fatalf("cannot delete snapshot %q: %s", snapshotName, err)
	}
	snapshots, err = s.ListSnapshots()
	if err != nil {
		t.Fatalf("cannot list snapshots: %s", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expecting zero snapshots; got %q", snapshots)
	}
	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

// testRemoveAll removes all storage data produced by a test if the test hasn't
// failed. For this to work, the storage must use t.Name() as the base dir in
// its data path.
//
// In case of failure, the data is kept for further debugging.
func testRemoveAll(t *testing.T) {
	defer func() {
		if !t.Failed() {
			fs.MustRemoveAll(t.Name())
		}
	}()
}

func TestStorageRowsNotAdded(t *testing.T) {
	defer testRemoveAll(t)

	type options struct {
		name        string
		retention   time.Duration
		mrs         []MetricRow
		tr          TimeRange
		wantMetrics *Metrics
	}
	f := func(opts *options) {
		t.Helper()

		var gotMetrics Metrics
		path := fmt.Sprintf("%s/%s", t.Name(), opts.name)
		s := MustOpenStorage(path, OpenOptions{Retention: opts.retention})
		defer s.MustClose()
		s.AddRows(opts.mrs, defaultPrecisionBits)
		s.DebugFlush()
		s.UpdateMetrics(&gotMetrics)

		got := testCountAllMetricNames(s, opts.tr)
		if got != 0 {
			t.Fatalf("unexpected metric name count: got %d, want 0", got)
		}

		if got, want := gotMetrics.RowsReceivedTotal, opts.wantMetrics.RowsReceivedTotal; got != want {
			t.Fatalf("unexpected Metrics.RowsReceivedTotal: got %d, want %d", got, want)
		}
		if got, want := gotMetrics.RowsAddedTotal, opts.wantMetrics.RowsAddedTotal; got != want {
			t.Fatalf("unexpected Metrics.RowsAddedTotal: got %d, want %d", got, want)
		}
		if got, want := gotMetrics.InvalidRawMetricNames, opts.wantMetrics.InvalidRawMetricNames; got != want {
			t.Fatalf("unexpected Metrics.InvalidRawMetricNames: got %d, want %d", got, want)
		}
	}

	const numRows = 1000
	var (
		rng          = rand.New(rand.NewSource(1))
		retention    time.Duration
		minTimestamp int64
		maxTimestamp int64
		mrs          []MetricRow
	)

	minTimestamp = -1000
	maxTimestamp = -1
	f(&options{
		name:      "NegativeTimestamps",
		retention: retentionMax,
		mrs:       testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp),
		tr:        TimeRange{minTimestamp, maxTimestamp},
		wantMetrics: &Metrics{
			RowsReceivedTotal:     numRows,
			TooSmallTimestampRows: numRows,
		},
	})

	retention = 48 * time.Hour
	minTimestamp = time.Now().Add(-retention - time.Hour).UnixMilli()
	maxTimestamp = minTimestamp + 1000
	f(&options{
		name:      "TooSmallTimestamps",
		retention: retention,
		mrs:       testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp),
		tr:        TimeRange{minTimestamp, maxTimestamp},
		wantMetrics: &Metrics{
			RowsReceivedTotal:     numRows,
			TooSmallTimestampRows: numRows,
		},
	})

	retention = 48 * time.Hour
	minTimestamp = time.Now().Add(7 * 24 * time.Hour).UnixMilli()
	maxTimestamp = minTimestamp + 1000
	f(&options{
		name:      "TooBigTimestamps",
		retention: retention,
		mrs:       testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp),
		tr:        TimeRange{minTimestamp, maxTimestamp},
		wantMetrics: &Metrics{
			RowsReceivedTotal:   numRows,
			TooBigTimestampRows: numRows,
		},
	})

	minTimestamp = time.Now().UnixMilli()
	maxTimestamp = minTimestamp + 1000
	mrs = testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp)
	for i := range numRows {
		mrs[i].MetricNameRaw = []byte("garbage")
	}
	f(&options{
		name: "InvalidMetricNameRaw",
		mrs:  mrs,
		tr:   TimeRange{minTimestamp, maxTimestamp},
		wantMetrics: &Metrics{
			RowsReceivedTotal:     numRows,
			InvalidRawMetricNames: numRows,
		},
	})
}

func TestStorageRowsNotAdded_SeriesLimitExceeded(t *testing.T) {
	defer testRemoveAll(t)

	f := func(t *testing.T, numRows uint64, maxHourlySeries, maxDailySeries int) {
		t.Helper()

		rng := rand.New(rand.NewSource(1))
		minTimestamp := time.Now().UnixMilli()
		maxTimestamp := minTimestamp + 1000
		mrs := testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp)

		// Insert metrics into the empty storage. The insertion will take the slow path.
		opts := OpenOptions{
			MaxHourlySeries: maxHourlySeries,
			MaxDailySeries:  maxDailySeries,
		}
		s := MustOpenStorage(t.Name(), opts)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		assertCounts := func(pathName string) {
			var gotMetrics Metrics
			s.UpdateMetrics(&gotMetrics)

			if got, want := gotMetrics.RowsReceivedTotal, numRows; got != want {
				t.Fatalf("[%s] unexpected Metrics.RowsReceivedTotal: got %d, want %d", pathName, got, want)
			}
			if got := gotMetrics.HourlySeriesLimitRowsDropped; maxHourlySeries > 0 && got <= 0 {
				t.Fatalf("[%s] unexpected Metrics.HourlySeriesLimitRowsDropped: got %d, want > 0", pathName, got)
			}
			if got := gotMetrics.DailySeriesLimitRowsDropped; maxDailySeries > 0 && got <= 0 {
				t.Fatalf("[%s] unexpected Metrics.DailySeriesLimitRowsDropped: got %d, want > 0", pathName, got)
			}

			want := numRows - (gotMetrics.HourlySeriesLimitRowsDropped + gotMetrics.DailySeriesLimitRowsDropped)
			if got := testCountAllMetricNames(s, TimeRange{minTimestamp, maxTimestamp}); uint64(got) != want {
				t.Fatalf("[%s] unexpected metric name count: %d, want %d", pathName, got, want)
			}

			if got := gotMetrics.RowsAddedTotal; got != want {
				t.Fatalf("[%s] unexpected Metrics.RowsAddedTotal: got %d, want %d", pathName, got, want)
			}
		}

		assertCounts("Slow Path")
		s.MustClose()

		// Open the storage again and insert the same metrics again.
		// This time tsidCache should have the metric names and the fast path
		// branch will be executed.
		s = MustOpenStorage(t.Name(), opts)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		assertCounts("Fast Path")
		s.MustClose()

		// Open the storage again, drop tsidCache, and insert the same metrics
		// again. This time tsidCache should not have the metric names so they
		// will be searched in index. Thus, the insertion takes the slower path.
		s = MustOpenStorage(t.Name(), opts)
		s.resetAndSaveTSIDCache()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		assertCounts("Slower Path")
		s.MustClose()
	}

	const (
		numRows         = 1000
		maxHourlySeries = 500
		maxDailySeries  = 500
	)

	t.Run("HourlyLimitExceeded", func(t *testing.T) {
		f(t, numRows, maxHourlySeries, 0)
	})

	t.Run("DailyLimitExceeded", func(t *testing.T) {
		f(t, numRows, 0, maxDailySeries)
	})
}

// testCountAllMetricNames is a test helper function that counts the names of
// all time series within the given time range.
func testCountAllMetricNames(s *Storage, tr TimeRange) int {
	tfsAll := NewTagFilters()
	if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		panic(fmt.Sprintf("unexpected error in TagFilters.Add: %v", err))
	}
	names, err := s.SearchMetricNames(nil, []*TagFilters{tfsAll}, tr, 1e9, noDeadline)
	if err != nil {
		panic(fmt.Sprintf("SearchMetricNames() failed unexpectedly: %v", err))
	}
	return len(names)
}

// testCountAllMetricIDs is a test helper function that counts the IDs of
// all time series within the given time range.
func testCountAllMetricIDs(s *Storage, tr TimeRange) int {
	tfsAll := NewTagFilters()
	if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		panic(fmt.Sprintf("unexpected error in TagFilters.Add: %v", err))
	}
	if s.disablePerDayIndex {
		tr = globalIndexTimeRange
	}
	idb, putIndexDB := s.getCurrIndexDB()
	defer putIndexDB()
	ids, err := idb.searchMetricIDs(nil, []*TagFilters{tfsAll}, tr, 1e9, noDeadline)
	if err != nil {
		panic(fmt.Sprintf("seachMetricIDs() failed unexpectedly: %s", err))
	}
	return len(ids)
}

func TestStorageSearchMetricNames_TooManyTimeseries(t *testing.T) {
	defer testRemoveAll(t)

	const (
		numDays = 100
		numRows = 10
	)
	rng := rand.New(rand.NewSource(1))
	var (
		days []TimeRange
		mrs  []MetricRow
	)
	for i := range numDays {
		day := TimeRange{
			MinTimestamp: time.Date(2000, 1, i+1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, i+1, 23, 59, 59, 999, time.UTC).UnixMilli(),
		}
		days = append(days, day)
		prefix1 := fmt.Sprintf("metric1_%d", i)
		mrs = append(mrs, testGenerateMetricRowsWithPrefix(rng, numRows, prefix1, day)...)
		prefix2 := fmt.Sprintf("metric2_%d", i)
		mrs = append(mrs, testGenerateMetricRowsWithPrefix(rng, numRows, prefix2, day)...)
	}

	type options struct {
		path       string
		filters    []string
		tr         TimeRange
		maxMetrics int
		wantErr    bool
		wantCount  int
	}
	f := func(opts *options) {
		t.Helper()

		s := MustOpenStorage(t.Name()+"/"+opts.path, OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		var tfss []*TagFilters
		for _, filter := range opts.filters {
			filter := fmt.Sprintf("%s.*", filter)
			tfs := NewTagFilters()
			if err := tfs.Add(nil, []byte(filter), false, true); err != nil {
				t.Fatalf("unexpected error in TagFilters.Add: %v", err)
			}
			tfss = append(tfss, tfs)
		}

		names, err := s.SearchMetricNames(nil, tfss, opts.tr, opts.maxMetrics, noDeadline)
		gotErr := err != nil
		if gotErr != opts.wantErr {
			t.Errorf("SeachMetricNames(%v, %v, %d): unexpected error: got %v, want error to happen %v", []any{
				tfss, &opts.tr, opts.maxMetrics, err, opts.wantErr}...)
		}
		if got := len(names); got != opts.wantCount {
			t.Errorf("SeachMetricNames(%v, %v, %d): unexpected metric name count: got %d, want %d", []any{
				tfss, &opts.tr, opts.maxMetrics, got, opts.wantCount}...)
		}
	}

	// Using one filter to search metric names within one day. The maxMetrics
	// param is set to match exactly the number of time series that match the
	// filter within that time range. Search operation must complete
	// successfully.
	f(&options{
		path:       "OneDay/OneTagFilter/MaxMetricsNotExeeded",
		filters:    []string{"metric1"},
		tr:         days[0],
		maxMetrics: numRows,
		wantCount:  numRows,
	})

	// Using one filter to search metric names within one day. The maxMetrics
	// param is less than the number of time series that match the filter
	// within that time range. Search operation must fail.
	f(&options{
		path:       "OneDay/OneTagFilter/MaxMetricsExeeded",
		filters:    []string{"metric1"},
		tr:         days[0],
		maxMetrics: numRows - 1,
		wantErr:    true,
	})

	// Using two filters to search metric names within one day. The maxMetrics
	// param is set to match exactly the number of time series that match the
	// two filters within that time range. Search operation must complete
	// successfully.
	f(&options{
		path:       "OneDay/TwoTagFilters/MaxMetricsNotExeeded",
		filters:    []string{"metric1", "metric2"},
		tr:         days[0],
		maxMetrics: numRows * 2,
		wantCount:  numRows * 2,
	})

	// Using two filters to search metric names within one day. The maxMetrics
	// param is less than the number of time series that match the two filters
	// within that time range. Search operation must fail.
	f(&options{
		path:       "OneDay/TwoTagFilters/MaxMetricsExeeded",
		filters:    []string{"metric1", "metric2"},
		tr:         days[0],
		maxMetrics: numRows*2 - 1,
		wantErr:    true,
	})

	// Using one filter to search metric names within two days. The maxMetrics
	// param is set to match exactly the number of time series that match the
	// filter within that time range. Search operation must complete
	// successfully.
	f(&options{
		path:    "TwoDays/OneTagFilter/MaxMetricsNotExeeded",
		filters: []string{"metric1"},
		tr: TimeRange{
			MinTimestamp: days[0].MinTimestamp,
			MaxTimestamp: days[1].MaxTimestamp,
		},
		maxMetrics: numRows * 2,
		wantCount:  numRows * 2,
	})

	// Using one filter to search metric names within two days. The maxMetrics
	// param is less than the number of time series that match the filter
	// within that time range. Search operation must fail.
	f(&options{
		path:    "TwoDays/OneTagFilter/MaxMetricsExeeded",
		filters: []string{"metric1"},
		tr: TimeRange{
			MinTimestamp: days[0].MinTimestamp,
			MaxTimestamp: days[1].MaxTimestamp,
		},
		maxMetrics: numRows*2 - 1,
		wantErr:    true,
	})

	// Using two filters to search metric names within two days. The maxMetrics
	// param is set to match exactly the number of time series that match the
	// two filters within that time range. Search operation must complete
	// successfully.
	f(&options{
		path:    "TwoDays/TwoTagFilters/MaxMetricsNotExeeded",
		filters: []string{"metric1", "metric2"},
		tr: TimeRange{
			MinTimestamp: days[0].MinTimestamp,
			MaxTimestamp: days[1].MaxTimestamp,
		},
		maxMetrics: numRows * 4,
		wantCount:  numRows * 4,
	})

	// Using two filters to search metric names within two days. The maxMetrics
	// param is less than the number of time series that match the two filters
	// within that time range. Search operation must fail.
	f(&options{
		path:    "TwoDays/TwoTagFilters/MaxMetricsExeeded",
		filters: []string{"metric1", "metric2"},
		tr: TimeRange{
			MinTimestamp: days[0].MinTimestamp,
			MaxTimestamp: days[1].MaxTimestamp,
		},
		maxMetrics: numRows*4 - 1,
		wantErr:    true,
	})

	// Using one filter to search metric names within the time range of 41 days.
	// This time range corresponds to the day difference of 40 days, which is
	// the max day difference when the per-day index is still used for
	// searching. The maxMetrics param is set to match exactly the number of
	// time series that match the filter within that time range. Search
	// operation must complete successfully.
	f(&options{
		path:    "40Days/OneTagFilter/MaxMetricsNotExeeded",
		filters: []string{"metric1"},
		tr: TimeRange{
			MinTimestamp: days[0].MinTimestamp,
			MaxTimestamp: days[40].MaxTimestamp,
		},
		maxMetrics: numRows * 41,
		wantCount:  numRows * 41,
	})

	// Using one filter to search metric names within the time range of 42 days.
	// This time range corresponds to the day difference of 41 days, which is
	// longer than than 40 days. In this case, the search is performed using
	// global index instead of per-day index and the metric names will be
	// searched within the entire retention period. The maxMetrics parameter,
	// however, is set to the number of time series within the 42 days. The
	// search must fail because the number of metrics will be much larger.
	f(&options{
		path:    "MoreThan40Days/OneTagFilter/MaxMetricsExeeded",
		filters: []string{"metric1"},
		tr: TimeRange{
			MinTimestamp: days[0].MinTimestamp,
			MaxTimestamp: days[41].MaxTimestamp,
		},
		maxMetrics: numRows * 42,
		wantErr:    true,
	})

	// To fix the above case, the maxMetrics must be adjusted to be not less
	// than the number of time series within the entire retention period.
	f(&options{
		path:    "MoreThan40Days/OneTagFilter/MaxMetricsNotExeeded",
		filters: []string{"metric1"},
		tr: TimeRange{
			MinTimestamp: days[0].MinTimestamp,
			MaxTimestamp: days[41].MaxTimestamp,
		},
		maxMetrics: numRows * numDays,
		wantCount:  numRows * numDays,
	})
}

func TestStorageDate(t *testing.T) {
	defer testRemoveAll(t)

	f := func(disablePerDayIndex bool, millis int64, want uint64) {
		t.Helper()
		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: disablePerDayIndex,
		})
		defer s.MustClose()
		if got := s.date(millis); got != want {
			t.Errorf("unexpected date: got %d, want %d", got, want)
		}
	}

	// Zero millis are converted to zero date regardless whether
	// -disablePerDayIndex flag is set or not.
	f(false, 0, 0)
	f(true, 0, 0)

	// When per-day index is enabled, positive millis are converted to the
	// corresponding date.
	f(false, 10*msecPerDay, 10)

	// When per-day index is disabled, positive millis are converted to
	// globalIndexDate.
	f(true, 10*msecPerDay, globalIndexDate)
}

func TestStorageDate_negativeMillis(t *testing.T) {
	defer testRemoveAll(t)

	// Negative millis won't be converted to a negative date because the date is
	// always positive. As a result, dates earlier than 1970-01-01 are not
	// supported. However, when the -disablePerDayIndex flag is set, negative
	// millis must be converted to globalIndexDate.
	s := MustOpenStorage(t.Name(), OpenOptions{
		DisablePerDayIndex: true,
	})
	millis := int64(-10 * msecPerDay)
	want := globalIndexDate
	if got := s.date(millis); got != want {
		t.Errorf("unexpected date: got %d, want %d", got, want)
	}
	s.MustClose()
}

func TestStorageAdjustTimeRange(t *testing.T) {
	defer testRemoveAll(t)

	f := func(disablePerDayIndex bool, tr TimeRange, want TimeRange) {
		t.Helper()

		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: disablePerDayIndex,
		})
		defer s.MustClose()
		if got := s.adjustTimeRange(tr); got != want {
			t.Errorf("unexpected time range: got %v, want %v", &got, &want)
		}
	}

	var tr TimeRange

	// Zero time range is adjusted to globalIndexTimeRange regardless whether
	// the -disablePerDayIndex flag is set or not.
	tr = TimeRange{}
	f(false, tr, globalIndexTimeRange)
	f(true, tr, globalIndexTimeRange)

	// Time range is smaller than 40 days. When the -disablePerDayIndex flag is
	// unset, the time range will not be adjusted. When the flag is set, the
	// adjusted time range will be globalIndexTimeRange.
	tr = TimeRange{10 * msecPerDay, 50 * msecPerDay}
	f(false, tr, tr)
	f(true, tr, globalIndexTimeRange)

	// Time range is exactly 40 days. In this case, the TimeRange.MaxTimestamp
	// is set to the minimum possible value when the time range becomes 40 days.
	// When the -disablePerDayIndex flag is unset, the time range will not be
	// adjusted. When the flag is set, the adjusted time range will be
	// globalIndexTimeRange.
	tr = TimeRange{10 * msecPerDay, 50*msecPerDay + 1}
	f(false, tr, tr)
	f(true, tr, globalIndexTimeRange)

	// Another case when time range is exactly 40 days. This time the the
	// TimeRange.MaxTimestamp is set to its max value when the time range is
	// still 40 days.
	// When the -disablePerDayIndex flag is unset, the time range will not be
	// adjusted. When the flag is set, the adjusted time range will be
	// globalIndexTimeRange.
	tr = TimeRange{10 * msecPerDay, 51 * msecPerDay}
	f(false, tr, tr)
	f(true, tr, globalIndexTimeRange)

	// Time range is more than 40 days. The time range is adjusted to
	// globalIndexTimeRange regardless whether the -disablePerDayIndex flag is
	// set or not.
	tr = TimeRange{10 * msecPerDay, 51*msecPerDay + 1}
	f(false, tr, globalIndexTimeRange)
	f(true, tr, globalIndexTimeRange)

}

type testStorageSearchWithoutPerDayIndexOptions struct {
	mrs                []MetricRow
	assertSearchResult func(t *testing.T, s *Storage, tr TimeRange, want any)
	alwaysPerTimeRange bool // If true, use wantPerTimeRange instead of wantAll
	wantPerTimeRange   map[TimeRange]any
	wantAll            any
	wantEmpty          any
}

// testStorageSearchWithoutPerDayIndex tests how the search behaves when the
// per-day index is disabled. This function is expected to be called by
// functions that test a particular search operation, such as GetTSDBStatus(),
// SearchMetricNames(), etc.
func testStorageSearchWithoutPerDayIndex(t *testing.T, opts *testStorageSearchWithoutPerDayIndexOptions) {
	defer testRemoveAll(t)

	// The data is inserted and the search is performed when the per-day index
	// is enabled.
	t.Run("InsertAndSearchWithPerDayIndex", func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: false,
		})
		s.AddRows(opts.mrs, defaultPrecisionBits)
		s.DebugFlush()
		for tr, want := range opts.wantPerTimeRange {
			opts.assertSearchResult(t, s, tr, want)
		}
		s.MustClose()
	})

	//  The data is inserted and the search is performed when the per-day index
	//  is disabled.
	t.Run("InsertAndSearchWithoutPerDayIndex", func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: true,
		})
		s.AddRows(opts.mrs, defaultPrecisionBits)
		s.DebugFlush()
		for tr, want := range opts.wantPerTimeRange {
			if !opts.alwaysPerTimeRange {
				want = opts.wantAll
			}
			opts.assertSearchResult(t, s, tr, want)
		}
		s.MustClose()
	})

	// The data is inserted when the per-day index is enabled but the search is
	// performed when the per-day index is disabled.
	t.Run("InsertWithPerDayIndexSearchWithout", func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: false,
		})
		s.AddRows(opts.mrs, defaultPrecisionBits)
		s.DebugFlush()
		s.MustClose()

		s = MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: true,
		})
		for tr, want := range opts.wantPerTimeRange {
			if !opts.alwaysPerTimeRange {
				want = opts.wantAll
			}
			opts.assertSearchResult(t, s, tr, want)
		}
		s.MustClose()
	})

	// The data is inserted when the per-day index is disabled but the search is
	// performed when the per-day index is enabled. This case also shows that
	// registering metric names recovers the per-day index.
	t.Run("InsertWithoutPerDayIndexSearchWith", func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: true,
		})
		s.AddRows(opts.mrs, defaultPrecisionBits)
		s.DebugFlush()
		s.MustClose()

		s = MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: false,
		})
		for tr := range opts.wantPerTimeRange {
			opts.assertSearchResult(t, s, tr, opts.wantEmpty)
		}

		// Verify that search result contains correct label values after populating
		// per-day index by registering metric names.
		s.RegisterMetricNames(nil, opts.mrs)
		s.DebugFlush()
		for tr, want := range opts.wantPerTimeRange {
			opts.assertSearchResult(t, s, tr, want)
		}
		s.MustClose()
	})
}

func TestStorageGetTSDBStatusWithoutPerDayIndex(t *testing.T) {
	const (
		days = 4
		rows = 10
	)
	rng := rand.New(rand.NewSource(1))
	opts := testStorageSearchWithoutPerDayIndexOptions{
		wantEmpty:        &TSDBStatus{},
		wantPerTimeRange: make(map[TimeRange]any),
		wantAll:          &TSDBStatus{TotalSeries: days * rows},
	}
	for day := 1; day <= days; day++ {
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, day, 23, 59, 59, 999, time.UTC).UnixMilli(),
		}
		for row := 0; row < rows; row++ {
			name := fmt.Sprintf("metric_%d", rows*day+row)
			mn := &MetricName{
				MetricGroup: []byte(name),
			}
			metricNameRaw := mn.marshalRaw(nil)
			opts.mrs = append(opts.mrs, MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp,
				Value:         rng.NormFloat64() * 1e6,
			})
		}
		opts.wantPerTimeRange[tr] = &TSDBStatus{TotalSeries: rows}
	}

	opts.assertSearchResult = func(t *testing.T, s *Storage, tr TimeRange, want any) {
		t.Helper()

		date := uint64(tr.MinTimestamp) / msecPerDay
		gotStatus, err := s.GetTSDBStatus(nil, nil, date, "", 10, 1e6, noDeadline)
		if err != nil {
			t.Fatalf("GetTSDBStatus(%v) failed unexpectedly", &tr)
		}

		wantStatus := want.(*TSDBStatus)
		if got, want := gotStatus.TotalSeries, wantStatus.TotalSeries; got != want {
			t.Errorf("[%v] unexpected TSDBStatus.TotalSeries: got %d, want %d", &tr, got, want)
		}
	}

	testStorageSearchWithoutPerDayIndex(t, &opts)
}

func TestStorageSearchMetricNamesWithoutPerDayIndex(t *testing.T) {
	const (
		days = 4
		rows = 10
	)
	rng := rand.New(rand.NewSource(1))
	opts := testStorageSearchWithoutPerDayIndexOptions{
		wantEmpty:        []string(nil),
		wantPerTimeRange: make(map[TimeRange]any),
		wantAll:          []string{},
	}
	for day := 1; day <= days; day++ {
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, day, 23, 59, 59, 999, time.UTC).UnixMilli(),
		}
		var want []string
		for row := 0; row < rows; row++ {
			name := fmt.Sprintf("metric_%d", rows*day+row)
			mn := &MetricName{
				MetricGroup: []byte(name),
			}
			metricNameRaw := mn.marshalRaw(nil)
			want = append(want, string(name))
			opts.mrs = append(opts.mrs, MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp,
				Value:         rng.NormFloat64() * 1e6,
			})
		}
		opts.wantPerTimeRange[tr] = want
		opts.wantAll = append(opts.wantAll.([]string), want...)
	}

	opts.assertSearchResult = func(t *testing.T, s *Storage, tr TimeRange, want any) {
		t.Helper()

		tfsAll := NewTagFilters()
		if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			panic(fmt.Sprintf("unexpected error in TagFilters.Add: %v", err))
		}
		got, err := s.SearchMetricNames(nil, []*TagFilters{tfsAll}, tr, 1e6, noDeadline)
		if err != nil {
			t.Fatalf("SearchMetricNames(%v) failed unexpectedly: %v", &tr, err)
		}
		for i, name := range got {
			var mn MetricName
			if err := mn.Unmarshal([]byte(name)); err != nil {
				t.Fatalf("mn.Unmarshal(%q) failed unexpectedly: %v", name, err)
			}
			got[i] = string(mn.MetricGroup)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("[%v] unexpected metric names: got %v, want %v", &tr, got, want)
		}
	}

	testStorageSearchWithoutPerDayIndex(t, &opts)
}

func TestStorageSearchLabelNamesWithoutPerDayIndex(t *testing.T) {
	const (
		days = 4
		rows = 10
	)
	rng := rand.New(rand.NewSource(1))
	opts := testStorageSearchWithoutPerDayIndexOptions{
		wantEmpty:        []string{},
		wantPerTimeRange: make(map[TimeRange]any),
		wantAll:          []string{},
	}
	for day := 1; day <= days; day++ {
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, day, 23, 59, 59, 999, time.UTC).UnixMilli(),
		}
		var want []string
		for row := 0; row < rows; row++ {
			labelName := fmt.Sprintf("job_%d", rows*day+row)
			mn := &MetricName{
				MetricGroup: []byte("metric"),
				Tags: []Tag{
					{[]byte(labelName), []byte("webservice")},
				},
			}
			metricNameRaw := mn.marshalRaw(nil)
			want = append(want, labelName)
			opts.mrs = append(opts.mrs, MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp,
				Value:         rng.NormFloat64() * 1e6,
			})
		}
		opts.wantAll = append(opts.wantAll.([]string), want...)
		opts.wantPerTimeRange[tr] = append(want, "__name__")
	}
	opts.wantAll = append(opts.wantAll.([]string), "__name__")

	opts.assertSearchResult = func(t *testing.T, s *Storage, tr TimeRange, want any) {
		t.Helper()
		got, err := s.SearchLabelNames(nil, []*TagFilters{}, tr, 1e6, 1e6, noDeadline)
		if err != nil {
			t.Fatalf("SearchLabelNames(%v) failed unexpectedly: %v", &tr, err)
		}
		slices.Sort(got)
		slices.Sort(want.([]string))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("[%v] unexpected label names: got %v, want %v", &tr, got, want)
		}
	}

	testStorageSearchWithoutPerDayIndex(t, &opts)
}

func TestStorageSearchLabelValuesWithoutPerDayIndex(t *testing.T) {
	const (
		days      = 4
		rows      = 10
		labelName = "job"
	)
	rng := rand.New(rand.NewSource(1))
	opts := testStorageSearchWithoutPerDayIndexOptions{
		wantEmpty:        []string{},
		wantPerTimeRange: make(map[TimeRange]any),
		wantAll:          []string{},
	}
	for day := 1; day <= days; day++ {
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, day, 23, 59, 59, 999, time.UTC).UnixMilli(),
		}
		var want []string
		for row := 0; row < rows; row++ {
			labelValue := fmt.Sprintf("webservice_%d", rows*day+row)
			mn := &MetricName{
				MetricGroup: []byte("metric"),
				Tags: []Tag{
					{[]byte(labelName), []byte(labelValue)},
				},
			}
			metricNameRaw := mn.marshalRaw(nil)
			want = append(want, labelValue)
			opts.mrs = append(opts.mrs, MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp,
				Value:         rng.NormFloat64() * 1e6,
			})
		}
		opts.wantPerTimeRange[tr] = want
		opts.wantAll = append(opts.wantAll.([]string), want...)
	}

	opts.assertSearchResult = func(t *testing.T, s *Storage, tr TimeRange, want any) {
		t.Helper()
		got, err := s.SearchLabelValues(nil, labelName, []*TagFilters{}, tr, 1e6, 1e6, noDeadline)
		if err != nil {
			t.Fatalf("SearchLabelValues(%v) failed unexpectedly: %v", &tr, err)
		}
		slices.Sort(got)
		slices.Sort(want.([]string))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("[%v] unexpected label values: got %v, want %v", &tr, got, want)
		}
	}

	testStorageSearchWithoutPerDayIndex(t, &opts)
}

func TestStorageSearchTagValueSuffixesWithoutPerDayIndex(t *testing.T) {
	const (
		days           = 4
		rows           = 10
		tagValuePrefix = "metric."
	)
	rng := rand.New(rand.NewSource(1))
	opts := testStorageSearchWithoutPerDayIndexOptions{
		wantEmpty:        []string{},
		wantPerTimeRange: make(map[TimeRange]any),
		wantAll:          []string{},
	}
	for day := 1; day <= days; day++ {
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, day, 23, 59, 59, 999, time.UTC).UnixMilli(),
		}
		for row := 0; row < rows; row++ {
			metricName := fmt.Sprintf("%sday%d.row%d", tagValuePrefix, day, row)
			mn := &MetricName{
				MetricGroup: []byte(metricName),
			}
			metricNameRaw := mn.marshalRaw(nil)
			opts.mrs = append(opts.mrs, MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp,
				Value:         rng.NormFloat64() * 1e6,
			})
		}
		want := fmt.Sprintf("day%d.", day)
		opts.wantPerTimeRange[tr] = []string{want}
		opts.wantAll = append(opts.wantAll.([]string), want)
	}

	opts.assertSearchResult = func(t *testing.T, s *Storage, tr TimeRange, want any) {
		t.Helper()
		got, err := s.SearchTagValueSuffixes(nil, tr, "", tagValuePrefix, '.', 1e6, noDeadline)
		if err != nil {
			t.Fatalf("SearchTagValueSuffixes(%v) failed unexpectedly: %v", &tr, err)
		}
		slices.Sort(got)
		slices.Sort(want.([]string))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("[%v] unexpected tag value suffixes: got %v, want %v", &tr, got, want)
		}
	}

	testStorageSearchWithoutPerDayIndex(t, &opts)
}

func TestStorageSearchGraphitePathsWithoutPerDayIndex(t *testing.T) {
	const (
		days = 4
		rows = 10
	)
	rng := rand.New(rand.NewSource(1))
	opts := testStorageSearchWithoutPerDayIndexOptions{
		wantEmpty:        []string(nil),
		wantPerTimeRange: make(map[TimeRange]any),
		wantAll:          []string{},
	}
	for day := 1; day <= days; day++ {
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, day, 23, 59, 59, 999, time.UTC).UnixMilli(),
		}
		want := make([]string, rows)
		for row := 0; row < rows; row++ {
			metricName := fmt.Sprintf("day%d.row%d", day, row)
			mn := &MetricName{
				MetricGroup: []byte(metricName),
			}
			metricNameRaw := mn.marshalRaw(nil)
			opts.mrs = append(opts.mrs, MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp,
				Value:         rng.NormFloat64() * 1e6,
			})
			want[row] = metricName
		}
		opts.wantPerTimeRange[tr] = want
		opts.wantAll = append(opts.wantAll.([]string), want...)
	}

	opts.assertSearchResult = func(t *testing.T, s *Storage, tr TimeRange, want any) {
		t.Helper()
		got, err := s.SearchGraphitePaths(nil, tr, []byte("*.*"), 1e6, noDeadline)
		if err != nil {
			t.Fatalf("SearchGraphitePaths(%v) failed unexpectedly: %v", &tr, err)
		}
		slices.Sort(got)
		slices.Sort(want.([]string))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("[%v] unexpected graphite paths: got %v, want %v", &tr, got, want)
		}
	}

	testStorageSearchWithoutPerDayIndex(t, &opts)
}

func TestStorageQueryWithoutPerDayIndex(t *testing.T) {
	const (
		days = 4
		rows = 10
	)
	rng := rand.New(rand.NewSource(1))
	opts := testStorageSearchWithoutPerDayIndexOptions{
		wantEmpty:          []MetricRow(nil),
		wantPerTimeRange:   make(map[TimeRange]any),
		alwaysPerTimeRange: true,
	}
	for day := 1; day <= days; day++ {
		tr := TimeRange{
			MinTimestamp: time.Date(2024, 1, day, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, 1, day, 23, 59, 59, 999, time.UTC).UnixMilli(),
		}
		var want []MetricRow
		for row := 0; row < rows; row++ {
			seqNumber := rows*day + row
			name := fmt.Sprintf("metric_%d", seqNumber)
			mn := &MetricName{
				MetricGroup: []byte(name),
			}
			metricNameRaw := mn.marshalRaw(nil)
			mr := MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp,
				Value:         float64(seqNumber),
			}
			opts.mrs = append(opts.mrs, mr)
			want = append(want, mr)
		}
		opts.wantPerTimeRange[tr] = want
	}

	opts.assertSearchResult = func(t *testing.T, s *Storage, tr TimeRange, want any) {
		t.Helper()

		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte(`metric_\d*`), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		if err := testAssertSearchResult(s, tr, tfs, want.([]MetricRow)); err != nil {
			t.Errorf("%v: %v", &tr, err)
		}
	}

	testStorageSearchWithoutPerDayIndex(t, &opts)
}

func TestStorageAddRows_SamplesWithZeroDate(t *testing.T) {
	defer testRemoveAll(t)

	f := func(t *testing.T, disablePerDayIndex bool) {
		t.Helper()

		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: disablePerDayIndex,
		})
		defer s.MustClose()

		mn := MetricName{MetricGroup: []byte("metric")}
		mr := MetricRow{MetricNameRaw: mn.marshalRaw(nil)}
		for range 10 {
			mr.Timestamp = rand.Int63n(msecPerDay)
			mr.Value = float64(rand.Intn(1000))
			s.AddRows([]MetricRow{mr}, defaultPrecisionBits)
			s.DebugFlush()
			// Reset TSID cache so that insertion takes the path that involves
			// checking whether the index contains metricName->TSID mapping.
			s.resetAndSaveTSIDCache()
		}

		want := 1
		firstUnixDay := TimeRange{
			MinTimestamp: 0,
			MaxTimestamp: msecPerDay - 1,
		}
		if got := s.newTimeseriesCreated.Load(); got != uint64(want) {
			t.Errorf("unexpected new timeseries count: got %d, want %d", got, want)
		}
		if got := testCountAllMetricNames(s, firstUnixDay); got != want {
			t.Errorf("unexpected metric name count: got %d, want %d", got, want)
		}
		if got := testCountAllMetricIDs(s, firstUnixDay); got != want {
			t.Errorf("unexpected metric id count: got %d, want %d", got, want)
		}
	}

	t.Run("disablePerDayIndex=false", func(t *testing.T) {
		f(t, false)
	})
	t.Run("disablePerDayIndex=true", func(t *testing.T) {
		f(t, true)
	})
}

func TestStorageAddRows_currHourMetricIDs(t *testing.T) {
	defer testRemoveAll(t)

	f := func(t *testing.T, disablePerDayIndex bool) {
		t.Helper()

		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: disablePerDayIndex,
		})
		defer s.MustClose()

		now := time.Now().UTC()
		currHourTR := TimeRange{
			MinTimestamp: time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 59, 59, 999_999_999, time.UTC).UnixMilli(),
		}
		currHour := uint64(currHourTR.MinTimestamp / 1000 / 3600)
		prevHourTR := TimeRange{
			MinTimestamp: currHourTR.MinTimestamp - 3600*1000,
			MaxTimestamp: currHourTR.MaxTimestamp - 3600*1000,
		}
		rng := rand.New(rand.NewSource(1))

		// Test current hour metricIDs population when data ingestion takes the
		// slow path. The database is empty, therefore the index and the
		// tsidCache contain no metricIDs, therefore the data ingestion will
		// take slow path.

		mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "slow_path", currHourTR)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		s.updateCurrHourMetricIDs(currHour)
		if got, want := s.currHourMetricIDs.Load().m.Len(), 1000; got != want {
			t.Errorf("[slow path] unexpected current hour metric ID count: got %d, want %d", got, want)
		}

		// Test current hour metricIDs population when data ingestion takes the
		// fast path (when the metricIDs are found in the tsidCache)

		// First insert samples to populate the tsidCache. The samples belong to
		// the previous hour, therefore the metricIDs won't be added to
		// currHourMetricIDs.
		mrs = testGenerateMetricRowsWithPrefix(rng, 1000, "fast_path", prevHourTR)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		s.updateCurrHourMetricIDs(currHour)
		if got, want := s.currHourMetricIDs.Load().m.Len(), 1000; got != want {
			t.Errorf("[fast path] unexpected current hour metric ID count after ingesting samples for previous hour: got %d, want %d", got, want)
		}

		// Now ingest the same metrics. This time the metricIDs will be found in
		// tsidCache so the ingestion will take the fast path.
		mrs = testGenerateMetricRowsWithPrefix(rng, 1000, "fast_path", currHourTR)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		s.updateCurrHourMetricIDs(currHour)
		if got, want := s.currHourMetricIDs.Load().m.Len(), 2000; got != want {
			t.Errorf("[fast path] unexpected current hour metric ID count: got %d, want %d", got, want)
		}

		// Test current hour metricIDs population when data ingestion takes the
		// slower path (when the metricIDs are not found in the tsidCache but
		// found in the index)

		// First insert samples to populate the index. The samples belong to
		// the previous hour, therefore the metricIDs won't be added to
		// currHourMetricIDs.
		mrs = testGenerateMetricRowsWithPrefix(rng, 1000, "slower_path", prevHourTR)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		s.updateCurrHourMetricIDs(currHour)
		if got, want := s.currHourMetricIDs.Load().m.Len(), 2000; got != want {
			t.Errorf("[slower path] unexpected current hour metric ID count after ingesting samples for previous hour: got %d, want %d", got, want)
		}
		// Inserted samples were also added to the tsidCache. Drop it to
		// enforce the fallback to index search.
		s.resetAndSaveTSIDCache()

		// Now ingest the same metrics. This time the metricIDs will be searched
		// and found in index so the ingestion will take the slower path.
		mrs = testGenerateMetricRowsWithPrefix(rng, 1000, "slower_path", currHourTR)
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		s.updateCurrHourMetricIDs(currHour)
		if got, want := s.currHourMetricIDs.Load().m.Len(), 3000; got != want {
			t.Errorf("[slower path] unexpected current hour metric ID count: got %d, want %d", got, want)
		}
	}

	t.Run("disablePerDayIndex=false", func(t *testing.T) {
		f(t, false)
	})
	t.Run("disablePerDayIndex=true", func(t *testing.T) {
		f(t, true)
	})
}

func TestStorageRegisterMetricNamesForVariousDataPatternsConcurrently(t *testing.T) {
	testStorageVariousDataPatternsConcurrently(t, true, func(s *Storage, mrs []MetricRow) {
		s.RegisterMetricNames(nil, mrs)
	})
}

func TestStorageAddRowsForVariousDataPatternsConcurrently(t *testing.T) {
	testStorageVariousDataPatternsConcurrently(t, false, func(s *Storage, mrs []MetricRow) {
		s.AddRows(mrs, defaultPrecisionBits)
	})
}

// testStorageVariousDataPatternsConcurrently tests different concurrency use
// cases when ingesting data of different patterns. Each concurrency use case
// considered with and without the per-day index.
//
// The function is intended to be used by other tests that define which
// operation (AddRows or RegisterMetricNames) is tested.
func testStorageVariousDataPatternsConcurrently(t *testing.T, registerOnly bool, op func(s *Storage, mrs []MetricRow)) {
	defer testRemoveAll(t)

	const concurrency = 4

	disablePerDayIndex := false
	t.Run("perDayIndexes/serial", func(t *testing.T) {
		testStorageVariousDataPatterns(t, disablePerDayIndex, registerOnly, op, 1, false)
	})
	t.Run("perDayIndexes/concurrentRows", func(t *testing.T) {
		testStorageVariousDataPatterns(t, disablePerDayIndex, registerOnly, op, concurrency, true)
	})
	t.Run("perDayIndexes/concurrentBatches", func(t *testing.T) {
		testStorageVariousDataPatterns(t, disablePerDayIndex, registerOnly, op, concurrency, false)
	})

	disablePerDayIndex = true
	t.Run("noPerDayIndexes/serial", func(t *testing.T) {
		testStorageVariousDataPatterns(t, disablePerDayIndex, registerOnly, op, 1, false)
	})
	t.Run("noPerDayIndexes/concurrentRows", func(t *testing.T) {
		testStorageVariousDataPatterns(t, disablePerDayIndex, registerOnly, op, concurrency, true)
	})
	t.Run("noPerDayIndexes/concurrentBatches", func(t *testing.T) {
		testStorageVariousDataPatterns(t, disablePerDayIndex, registerOnly, op, concurrency, false)
	})
}

// testStorageVariousDataPatterns tests the ingestion of different combinations
// of metric names and dates.
//
// The function is intended to be used by other tests that define the
// concurrency, the per-day index setting, and the operation (AddRows or
// RegisterMetricNames) under test.
func testStorageVariousDataPatterns(t *testing.T, disablePerDayIndex, registerOnly bool, op func(s *Storage, mrs []MetricRow), concurrency int, splitBatches bool) {
	f := func(t *testing.T, sameBatchMetricNames, sameRowMetricNames, sameBatchDates, sameRowDates bool) {
		batches, wantCounts := testGenerateMetricRowBatches(&batchOptions{
			numBatches:           3,
			numRowsPerBatch:      30,
			disablePerDayIndex:   disablePerDayIndex,
			registerOnly:         registerOnly,
			sameBatchMetricNames: sameBatchMetricNames,
			sameRowMetricNames:   sameRowMetricNames,
			sameBatchDates:       sameBatchDates,
			sameRowDates:         sameRowDates,
		})
		strict := concurrency == 1
		rowsAddedTotal := wantCounts.metrics.RowsAddedTotal

		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: disablePerDayIndex,
		})

		testDoConcurrently(s, op, concurrency, splitBatches, batches)
		s.DebugFlush()
		assertCounts(t, s, wantCounts, strict)

		// Rotate indexDB to test the case when TSIDs from tsidCache have the
		// generation that is older than the generation of the current indexDB.
		s.mustRotateIndexDB(time.Now())
		testDoConcurrently(s, op, concurrency, splitBatches, batches)
		s.DebugFlush()
		wantCounts.metrics.RowsAddedTotal += rowsAddedTotal
		assertCounts(t, s, wantCounts, strict)

		// Empty the tsidCache to test the case when tsid is retrived from the
		// index that belongs to the current generation indexDB.
		s.resetAndSaveTSIDCache()
		testDoConcurrently(s, op, concurrency, splitBatches, batches)
		s.DebugFlush()
		wantCounts.metrics.RowsAddedTotal += rowsAddedTotal
		assertCounts(t, s, wantCounts, strict)

		// Empty the tsidCache and rotate indexDB to test the case when tsid is
		// retrived from the index that belongs to the previous generation
		// indexDB.
		s.resetAndSaveTSIDCache()
		s.mustRotateIndexDB(time.Now())
		testDoConcurrently(s, op, concurrency, splitBatches, batches)
		s.DebugFlush()
		wantCounts.metrics.RowsAddedTotal += rowsAddedTotal
		assertCounts(t, s, wantCounts, strict)

		s.MustClose()
	}

	t.Run("sameBatchMetrics/sameRowMetrics/sameBatchDates/sameRowDates", func(t *testing.T) {
		// Batch1: metric 1971-01-01, metric 1971-01-01
		// Batch2: metric 1971-01-01, metric 1971-01-01
		t.Parallel()
		f(t, true, true, true, true)
	})

	t.Run("sameBatchMetrics/sameRowMetrics/sameBatchDates/diffRowDates", func(t *testing.T) {
		// Batch1: metric 1971-01-01, metric 1971-01-02
		// Batch2: metric 1971-01-01, metric 1971-01-02
		t.Parallel()
		f(t, true, true, true, false)
	})

	t.Run("sameBatchMetrics/sameRowMetrics/diffBatchDates/sameRowDates", func(t *testing.T) {
		// Batch1: metric 1971-01-01, metric 1971-01-01
		// Batch2: metric 1971-01-02, metric 1971-01-02
		t.Parallel()
		f(t, true, true, false, true)
	})

	t.Run("sameBatchMetrics/sameRowMetrics/diffBatchDates/diffRowDates", func(t *testing.T) {
		// Batch1: metric 1971-01-01, metric 1971-01-02
		// Batch2: metric 1971-01-03, metric 1971-01-04
		t.Parallel()
		f(t, true, true, false, false)
	})

	t.Run("sameBatchMetrics/diffRowMetrics/sameBatchDates/sameRowDates", func(t *testing.T) {
		// Batch1: metric_row0 1971-01-01, metric_row1 1971-01-01
		// Batch2: metric_row0 1971-01-01, metric_row1 1971-01-01
		t.Parallel()
		f(t, true, false, true, true)
	})

	t.Run("sameBatchMetrics/diffRowMetrics/sameBatchDates/diffRowDates", func(t *testing.T) {
		// Batch1: metric_row0 1971-01-01, metric_row1 1971-01-02
		// Batch2: metric_row0 1971-01-01, metric_row1 1971-01-02
		t.Parallel()
		f(t, true, false, true, false)
	})

	t.Run("sameBatchMetrics/diffRowMetrics/diffBatchDates/sameRowDates", func(t *testing.T) {
		// Batch1: metric_row0 1971-01-01, metric_row1 1971-01-01
		// Batch2: metric_row0 1971-01-02, metric_row1 1971-01-02
		t.Parallel()
		f(t, true, false, false, true)
	})

	t.Run("sameBatchMetrics/diffRowMetrics/diffBatchDates/diffRowDates", func(t *testing.T) {
		// Batch1: metric_row0 1971-01-01, metric_row1 1971-01-02
		// Batch2: metric_row0 1971-01-03, metric_row1 1971-01-04
		t.Parallel()
		f(t, true, false, false, false)
	})

	t.Run("diffBatchMetrics/sameRowMetrics/sameBatchDates/sameRowDates", func(t *testing.T) {
		// Batch1: metric_batch0 1971-01-01, metric_batch0 1971-01-01
		// Batch2: metric_batch1 1971-01-01, metric_batch1 1971-01-01
		t.Parallel()
		f(t, false, true, true, true)
	})

	t.Run("diffBatchMetrics/sameRowMetrics/sameBatchDates/diffRowDates", func(t *testing.T) {
		// Batch1: metric_batch0 1971-01-01, metric_batch0 1971-01-02
		// Batch2: metric_batch1 1971-01-01, metric_batch1 1971-01-02
		t.Parallel()
		f(t, false, true, true, false)
	})

	t.Run("diffBatchMetrics/sameRowMetrics/diffBatchDates/sameRowDates", func(t *testing.T) {
		// Batch1: metric_batch0 1971-01-01, metric_batch0 1971-01-01
		// Batch2: metric_batch1 1971-01-02, metric_batch1 1971-01-02
		t.Parallel()
		f(t, false, true, false, true)
	})

	t.Run("diffBatchMetrics/sameRowMetrics/diffBatchDates/diffRowDates", func(t *testing.T) {
		// Batch1: metric_batch0 1971-01-01, metric_batch0 1971-01-02
		// Batch2: metric_batch1 1971-01-03, metric_batch1 1971-01-04
		t.Parallel()
		f(t, false, true, false, false)
	})

	t.Run("diffBatchMetrics/diffRowMetrics/sameBatchDates/sameRowDates", func(t *testing.T) {
		// Batch1: metric_batch0_row0 1971-01-01, metric_batch0_row1 1971-01-01
		// Batch2: metric_batch1_row0 1971-01-01, metric_batch1_row1 1971-01-01
		t.Parallel()
		f(t, false, false, true, true)
	})

	t.Run("diffBatchMetrics/diffRowMetrics/sameBatchDates/diffRowDates", func(t *testing.T) {
		// Batch1: metric_batch0_row0 1971-01-01, metric_batch0_row1 1971-01-02
		// Batch2: metric_batch1_row0 1971-01-01, metric_batch1_row1 1971-01-02
		t.Parallel()
		f(t, false, false, true, false)
	})

	t.Run("diffBatchMetrics/diffRowMetrics/diffBatchDates/sameRowDates", func(t *testing.T) {
		// Batch1: metric_batch0_row0 1971-01-01, metric_batch0_row1 1971-01-01
		// Batch2: metric_batch1_row0 1971-01-02, metric_batch1_row1 1971-01-02
		t.Parallel()
		f(t, false, false, false, true)
	})

	t.Run("diffBatchMetrics/diffRowMetrics/diffBatchDates/diffRowDates", func(t *testing.T) {
		// Batch1: metric_batch0_row0 1971-01-01, metric_batch0_row1 1971-01-02
		// Batch2: metric_batch1_row0 1971-01-03, metric_batch1_row1 1971-01-04
		t.Parallel()
		f(t, false, false, false, false)
	})
}

// testDoConcurrently performs some storage operation on metric rows
// concurrently.
//
// The function accepts metric rows organized in batches. The number of
// goroutines is specified with concurrency arg. If splitBatches is false, then
// each batch is processed in a separate goroutine. Otherwise, rows from a
// single batch are spread across multiple goroutines and next batch won't be
// processed until all records of the current batch are processed.
func testDoConcurrently(s *Storage, op func(s *Storage, mrs []MetricRow), concurrency int, splitBatches bool, mrsBatches [][]MetricRow) {
	if concurrency < 1 {
		panic(fmt.Sprintf("Unexpected concurrency: got %d, want >= 1", concurrency))
	}

	var wg sync.WaitGroup
	mrsCh := make(chan []MetricRow)
	for range concurrency {
		wg.Add(1)
		go func() {
			for mrs := range mrsCh {
				op(s, mrs)
			}
			wg.Done()
		}()
	}

	n := 1
	if splitBatches {
		n = concurrency
	}
	for _, batch := range mrsBatches {
		step := len(batch) / n
		if step == 0 {
			step = 1
		}
		for begin := 0; begin < len(batch); begin += step {
			limit := begin + step
			if limit > len(batch) {
				limit = len(batch)
			}
			mrsCh <- batch[begin:limit]
		}
	}
	close(mrsCh)
	wg.Wait()
}

type counts struct {
	metrics          *Metrics
	timeRangeCounts  map[TimeRange]int
	dateTSDBStatuses map[uint64]*TSDBStatus
}

// assertCounts retrieves various counts from storage and compares them with
// the wanted ones.
//
// Some counts can be greater than wanted values because duplicate metric IDs
// can be created when rows are inserted concurrently. In this case `strict`
// arg can be set to false in order to replace strict equality comparison with
// `greater or equal`.
func assertCounts(t *testing.T, s *Storage, want *counts, strict bool) {
	t.Helper()

	var gotMetrics Metrics
	s.UpdateMetrics(&gotMetrics)
	if got, want := gotMetrics.RowsAddedTotal, want.metrics.RowsAddedTotal; got != want {
		t.Errorf("unexpected Metrics.RowsAddedTotal: got %d, want %d", got, want)
	}

	gotCnt, wantCnt := gotMetrics.NewTimeseriesCreated, want.metrics.NewTimeseriesCreated
	if strict {
		if gotCnt != wantCnt {
			t.Errorf("unexpected Metrics.NewTimeseriesCreated: got %d, want %d", gotCnt, wantCnt)
		}
	} else {
		if gotCnt < wantCnt {
			t.Errorf("unexpected Metrics.NewTimeseriesCreated: got %d, want >= %d", gotCnt, wantCnt)
		}
	}

	for tr, want := range want.timeRangeCounts {
		if got := testCountAllMetricNames(s, tr); got != want {
			t.Errorf("%v: unexpected metric name count: got %d, want %d", &tr, got, want)
		}
		got := testCountAllMetricIDs(s, tr)
		if strict {
			if got != want {
				t.Errorf("%v: unexpected metric ID count: got %d, want %d", &tr, got, want)
			}
		} else {
			if got < want {
				t.Errorf("%v: unexpected metric ID count: got %d, want >= %d", &tr, got, want)
			}
		}

	}

	for date, wantStatus := range want.dateTSDBStatuses {
		dt := time.UnixMilli(int64(date) * msecPerDay).UTC()
		gotStatus, err := s.GetTSDBStatus(nil, nil, date, "", 10, 1e6, noDeadline)
		if err != nil {
			t.Fatalf("GetTSDBStatus(%v) failed unexpectedly: %v", dt, err)
		}
		got, want := gotStatus.TotalSeries, wantStatus.TotalSeries
		if strict {
			if got != want {
				t.Errorf("%v: unexpected TSDBStatus.TotalSeries: got %d, want %d", dt, got, want)
			}
		} else {
			if got < want {
				t.Errorf("%v: unexpected TSDBStatus.TotalSeries: got %d, want >= %d", dt, got, want)
			}
		}
	}
}

type batchOptions struct {
	numBatches           int
	numRowsPerBatch      int
	disablePerDayIndex   bool
	registerOnly         bool
	sameBatchMetricNames bool
	sameRowMetricNames   bool
	sameBatchDates       bool
	sameRowDates         bool
}

// testGenerateMetricRowBatches generates metric rows batches of various
// combinations of metric names and dates. The function also returns the counts
// that the storage is expected to report once the generated batch is ingested
// into the storage.
func testGenerateMetricRowBatches(opts *batchOptions) ([][]MetricRow, *counts) {
	if opts.numBatches <= 0 {
		panic(fmt.Sprintf("unexpected number of batches: got %d, want > 0", opts.numBatches))
	}
	if opts.numRowsPerBatch <= 0 {
		panic(fmt.Sprintf("unexpected number of rows per batch: got %d, want > 0", opts.numRowsPerBatch))
	}

	rng := rand.New(rand.NewSource(1))

	batches := make([][]MetricRow, opts.numBatches)
	metricName := "metric"
	startTime := time.Date(1971, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(1971, 1, 1, 23, 59, 59, 999, time.UTC)
	days := time.Duration(0)
	trNames := make(map[TimeRange]map[string]bool)
	names := make(map[string]bool)
	for batch := range opts.numBatches {
		batchMetricName := metricName
		if !opts.sameBatchMetricNames {
			batchMetricName += fmt.Sprintf("_batch%d", batch)
		}
		var rows []MetricRow
		for row := range opts.numRowsPerBatch {
			rowMetricName := batchMetricName
			if !opts.sameRowMetricNames {
				rowMetricName += fmt.Sprintf("_row%d", row)
			}
			mn := MetricName{
				MetricGroup: []byte(rowMetricName),
			}
			tr := TimeRange{
				MinTimestamp: startTime.Add(days * 24 * time.Hour).UnixMilli(),
				MaxTimestamp: endTime.Add(days * 24 * time.Hour).UnixMilli(),
			}
			rows = append(rows, MetricRow{
				MetricNameRaw: mn.marshalRaw(nil),
				Timestamp:     rng.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp,
				Value:         rng.NormFloat64() * 1e6,
			})
			if !opts.sameRowDates {
				days++
			}

			if trNames[tr] == nil {
				trNames[tr] = make(map[string]bool)
			}
			names[rowMetricName] = true
			trNames[tr][rowMetricName] = true
		}
		batches[batch] = rows
		if opts.sameBatchDates {
			days = 0
		} else if opts.sameRowDates {
			days++
		}
	}

	allTimeseries := len(names)
	rowsAddedTotal := uint64(opts.numBatches * opts.numRowsPerBatch)

	// When RegisterMetricNames() is called it only restisters the time series
	// in IndexDB but no samples is written to the storage.
	if opts.registerOnly {
		rowsAddedTotal = 0
	}
	want := counts{
		metrics: &Metrics{
			RowsAddedTotal:       rowsAddedTotal,
			NewTimeseriesCreated: uint64(allTimeseries),
		},
		timeRangeCounts:  make(map[TimeRange]int),
		dateTSDBStatuses: make(map[uint64]*TSDBStatus),
	}
	for tr, names := range trNames {
		var count int
		if opts.disablePerDayIndex {
			count = allTimeseries
		} else {
			count = len(names)
		}
		date := uint64(tr.MinTimestamp / msecPerDay)
		want.timeRangeCounts[tr] = count
		want.dateTSDBStatuses[date] = &TSDBStatus{
			TotalSeries: uint64(count),
		}
	}
	return batches, &want
}

func TestStorageMetricTracker(t *testing.T) {
	defer testRemoveAll(t)
	rng := rand.New(rand.NewSource(1))
	numRows := uint64(1000)
	minTimestamp := time.Now().UnixMilli()
	maxTimestamp := minTimestamp + 1000
	mrs := testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp)

	var gotMetrics Metrics
	s := MustOpenStorage(t.Name(), OpenOptions{TrackMetricNamesStats: true})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()
	s.UpdateMetrics(&gotMetrics)

	var sr Search
	tr := TimeRange{
		MinTimestamp: minTimestamp,
		MaxTimestamp: maxTimestamp,
	}

	// check stats for metrics with 0 requests count
	mus := s.GetMetricNamesStats(nil, 10_000, 0, "")
	if len(mus.Records) != int(numRows) {
		t.Fatalf("unexpected Stats records count=%d, want %d records", len(mus.Records), numRows)
	}

	// search query for all ingested metrics
	tfs := NewTagFilters()
	if err := tfs.Add(nil, []byte("metric_.+"), false, true); err != nil {
		t.Fatalf("unexpected error at tfs add: %s", err)
	}

	sr.Init(nil, s, []*TagFilters{tfs}, tr, 1e5, noDeadline)
	for sr.NextMetricBlock() {
	}
	sr.MustClose()

	mus = s.GetMetricNamesStats(nil, 10_000, 0, "")
	if len(mus.Records) != 0 {
		t.Fatalf("unexpected Stats records count=%d; want 0 records", len(mus.Records))
	}
	mus = s.GetMetricNamesStats(nil, 10_000, 1, "")
	if len(mus.Records) != int(numRows) {
		t.Fatalf("unexpected Stats records count=%d, want %d records", len(mus.Records), numRows)
	}
}
