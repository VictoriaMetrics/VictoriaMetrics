package storage

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
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
}

func TestStorageOpenClose(t *testing.T) {
	path := "TestStorageOpenClose"
	for i := 0; i < 10; i++ {
		s := MustOpenStorage(path, -1, 1e5, 1e6)
		s.MustClose()
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func TestStorageRandTimestamps(t *testing.T) {
	path := "TestStorageRandTimestamps"
	retention := 10 * retention31Days
	s := MustOpenStorage(path, retention, 0, 0)
	t.Run("serial", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if err := testStorageRandTimestamps(s); err != nil {
				t.Fatalf("error on iteration %d: %s", i, err)
			}
			s.MustClose()
			s = MustOpenStorage(path, retention, 0, 0)
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
		if err := s.AddRows(mrs, defaultPrecisionBits); err != nil {
			errStr := err.Error()
			if !strings.Contains(errStr, "too big timestamp") && !strings.Contains(errStr, "too small timestamp") {
				return fmt.Errorf("unexpected error when adding mrs: %w", err)
			}
		}
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
	s := MustOpenStorage(path, 0, 0, 0)

	// Verify no label names exist
	lns, err := s.SearchLabelNamesWithFiltersOnTimeRange(nil, nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("error in SearchLabelNamesWithFiltersOnTimeRange() at the start: %s", err)
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
			s = MustOpenStorage(path, 0, 0, 0)
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
	lns, err = s.SearchLabelNamesWithFiltersOnTimeRange(nil, nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("error in SearchLabelNamesWithFiltersOnTimeRange after the test: %s", err)
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
		if err := s.AddRows(mrs, defaultPrecisionBits); err != nil {
			return fmt.Errorf("unexpected error when adding mrs: %w", err)
		}
	}
	s.DebugFlush()

	// Verify tag values exist
	tvs, err := s.SearchLabelValuesWithFiltersOnTimeRange(nil, string(workerTag), nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelValuesWithFiltersOnTimeRange before metrics removal: %w", err)
	}
	if len(tvs) == 0 {
		return fmt.Errorf("unexpected empty number of tag values for workerTag")
	}

	// Verify tag keys exist
	lns, err := s.SearchLabelNamesWithFiltersOnTimeRange(nil, nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelNamesWithFiltersOnTimeRange before metrics removal: %w", err)
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
		deletedCount, err := s.DeleteSeries(nil, []*TagFilters{tfs})
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
		deletedCount, err = s.DeleteSeries(nil, nil)
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
	tvs, err = s.SearchLabelValuesWithFiltersOnTimeRange(nil, string(workerTag), nil, TimeRange{}, 1e5, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelValuesWithFiltersOnTimeRange after all the metrics are removed: %w", err)
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

func TestStorageRegisterMetricNamesSerial(t *testing.T) {
	path := "TestStorageRegisterMetricNamesSerial"
	s := MustOpenStorage(path, 0, 0, 0)
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
	s := MustOpenStorage(path, 0, 0, 0)
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

	// Verify that SearchLabelNamesWithFiltersOnTimeRange returns correct result.
	lnsExpected := []string{
		"__name__",
		"add_id",
		"instance",
		"job",
	}
	lns, err := s.SearchLabelNamesWithFiltersOnTimeRange(nil, nil, TimeRange{}, 100, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelNamesWithFiltersOnTimeRange: %w", err)
	}
	sort.Strings(lns)
	if !reflect.DeepEqual(lns, lnsExpected) {
		return fmt.Errorf("unexpected label names returned from SearchLabelNamesWithFiltersOnTimeRange;\ngot\n%q\nwant\n%q", lns, lnsExpected)
	}

	// Verify that SearchLabelNamesWithFiltersOnTimeRange with the specified time range returns correct result.
	now := timestampFromTime(time.Now())
	start := now - msecPerDay
	end := now + 60*1000
	tr := TimeRange{
		MinTimestamp: start,
		MaxTimestamp: end,
	}
	lns, err = s.SearchLabelNamesWithFiltersOnTimeRange(nil, nil, tr, 100, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelNamesWithFiltersOnTimeRange: %w", err)
	}
	sort.Strings(lns)
	if !reflect.DeepEqual(lns, lnsExpected) {
		return fmt.Errorf("unexpected label names returned from SearchLabelNamesWithFiltersOnTimeRange;\ngot\n%q\nwant\n%q", lns, lnsExpected)
	}

	// Verify that SearchLabelValuesWithFiltersOnTimeRange returns correct result.
	addIDs, err := s.SearchLabelValuesWithFiltersOnTimeRange(nil, "add_id", nil, TimeRange{}, addsCount+100, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelValuesWithFiltersOnTimeRange: %w", err)
	}
	sort.Strings(addIDs)
	if !reflect.DeepEqual(addIDs, addIDsExpected) {
		return fmt.Errorf("unexpected tag values returned from SearchLabelValuesWithFiltersOnTimeRange;\ngot\n%q\nwant\n%q", addIDs, addIDsExpected)
	}

	// Verify that SearchLabelValuesWithFiltersOnTimeRange with the specified time range returns correct result.
	addIDs, err = s.SearchLabelValuesWithFiltersOnTimeRange(nil, "add_id", nil, tr, addsCount+100, 1e9, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchLabelValuesWithFiltersOnTimeRange: %w", err)
	}
	sort.Strings(addIDs)
	if !reflect.DeepEqual(addIDs, addIDsExpected) {
		return fmt.Errorf("unexpected tag values returned from SearchLabelValuesWithFiltersOnTimeRange;\ngot\n%q\nwant\n%q", addIDs, addIDsExpected)
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
	retention := 10 * retention31Days
	s := MustOpenStorage(path, retention, 1e5, 1e5)
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
	retention := 10 * retention31Days
	s := MustOpenStorage(path, retention, 1e5, 1e5)
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
	var mrs []MetricRow
	var mn MetricName
	mn.Tags = []Tag{
		{[]byte("job"), []byte("webservice")},
		{[]byte("instance"), []byte("1.2.3.4")},
	}
	for i := 0; i < int(rows); i++ {
		mn.MetricGroup = []byte(fmt.Sprintf("metric_%d", i))
		metricNameRaw := mn.marshalRaw(nil)
		timestamp := rng.Int63n(timestampMax-timestampMin) + timestampMin
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
		if err := s.AddRows(mrs, defaultPrecisionBits); err != nil {
			return fmt.Errorf("unexpected error when adding mrs: %w", err)
		}
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
	s1 := MustOpenStorage(snapshotPath, 0, 0, 0)

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
	path := "TestStorageRotateIndexDB"
	s := MustOpenStorage(path, 0, 0, 0)

	// Start indexDB rotater in a separate goroutine
	stopCh := make(chan struct{})
	rotateDoneCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCh:
				close(rotateDoneCh)
				return
			default:
				time.Sleep(time.Millisecond)
				s.mustRotateIndexDB(time.Now())
			}
		}
	}()

	// Run concurrent workers that insert / select data from the storage.
	ch := make(chan error, 3)
	for i := 0; i < cap(ch); i++ {
		go func(workerNum int) {
			ch <- testStorageAddMetrics(s, workerNum)
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

	close(stopCh)
	<-rotateDoneCh

	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func testStorageAddMetrics(s *Storage, workerNum int) error {
	rng := rand.New(rand.NewSource(1))
	const rowsCount = 1e3

	var mn MetricName
	mn.Tags = []Tag{
		{[]byte("job"), []byte(fmt.Sprintf("webservice_%d", workerNum))},
		{[]byte("instance"), []byte("1.2.3.4")},
	}
	for i := 0; i < rowsCount; i++ {
		mn.MetricGroup = []byte(fmt.Sprintf("metric_%d_%d", workerNum, rng.Intn(10)))
		metricNameRaw := mn.marshalRaw(nil)
		timestamp := rng.Int63n(1e10)
		value := rng.NormFloat64() * 1e6

		mr := MetricRow{
			MetricNameRaw: metricNameRaw,
			Timestamp:     timestamp,
			Value:         value,
		}
		if err := s.AddRows([]MetricRow{mr}, defaultPrecisionBits); err != nil {
			return fmt.Errorf("unexpected error when adding mrs: %w", err)
		}
	}

	// Verify the storage contains rows.
	minRowsExpected := uint64(rowsCount)
	var m Metrics
	s.UpdateMetrics(&m)
	if rowsCount := m.TableMetrics.TotalRowsCount(); rowsCount < minRowsExpected {
		return fmt.Errorf("expecting at least %d rows in the table; got %d", minRowsExpected, rowsCount)
	}
	return nil
}

func TestStorageDeleteStaleSnapshots(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	path := "TestStorageDeleteStaleSnapshots"
	retention := 10 * retention31Days
	s := MustOpenStorage(path, retention, 1e5, 1e5)
	const rowsPerAdd = 1e3
	const addsCount = 10
	maxTimestamp := timestampFromTime(time.Now())
	minTimestamp := maxTimestamp - s.retentionMsecs
	for i := 0; i < addsCount; i++ {
		mrs := testGenerateMetricRows(rng, rowsPerAdd, minTimestamp, maxTimestamp)
		if err := s.AddRows(mrs, defaultPrecisionBits); err != nil {
			t.Fatalf("unexpected error when adding mrs: %s", err)
		}
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

func TestStorageSeriesAreNotCreatedOnStaleMarkers(t *testing.T) {
	path := "TestStorageSeriesAreNotCreatedOnStaleMarkers"
	s := MustOpenStorage(path, -1, 1e5, 1e6)

	tr := TimeRange{MinTimestamp: 0, MaxTimestamp: 2e10}
	tfsAll := NewTagFilters()
	if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %s", err)
	}

	findN := func(n int) {
		t.Helper()
		lns, err := s.SearchMetricNames(nil, []*TagFilters{tfsAll}, tr, 1e5, noDeadline)
		if err != nil {
			t.Fatalf("error in SearchLabelNamesWithFiltersOnTimeRange() at the start: %s", err)
		}
		if len(lns) != n {
			fmt.Println(lns)
			t.Fatalf("expected to find %d metric names, found %d instead", n, len(lns))
		}
	}

	// db is empty, so should be search results
	findN(0)

	rng := rand.New(rand.NewSource(1))
	mrs := testGenerateMetricRows(rng, 20, tr.MinTimestamp, tr.MaxTimestamp)
	// populate storage with some rows
	if err := s.AddRows(mrs[:10], defaultPrecisionBits); err != nil {
		t.Fatal("error when adding mrs: %w", err)
	}
	s.DebugFlush()

	// verify ingested rows are searchable
	findN(10)

	// clean up ingested data
	_, err := s.DeleteSeries(nil, []*TagFilters{tfsAll})
	if err != nil {
		t.Fatalf("DeleteSeries failed: %s", err)
	}

	// verify that data was actually deleted
	findN(0)

	// mark every 2nd row as stale, simulating a stale target
	for i := 0; i < len(mrs); i = i + 2 {
		mrs[i].Value = decimal.StaleNaN
	}
	if err := s.AddRows(mrs, defaultPrecisionBits); err != nil {
		t.Fatal("error when adding mrs: %w", err)
	}
	s.DebugFlush()

	// verify that rows marked as stale aren't searchable
	findN(10)

	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}
