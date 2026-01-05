package storage

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/google/go-cmp/cmp"
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

func TestUpdateCurrHourMetricIDs(t *testing.T) {
	defer testRemoveAll(t)

	t.Run("empty_pending_metric_ids_stale_curr_hour", func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

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
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

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
		if hmPrev.m.Len() > 0 {
			t.Fatalf("hmPrev is not empty: %v", hmPrev)
		}

		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_stale_curr_hour", func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

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
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

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
		if hmPrev.m.Len() > 0 {
			t.Fatalf("hmPrev is not empty: %v", hmPrev)
		}

		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_valid_curr_hour_start_of_day", func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

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
		if hmPrev.m.Len() > 0 {
			t.Fatalf("hmPrev is not empty: %v", hmPrev)
		}

		if s.pendingHourEntries.Len() != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", s.pendingHourEntries.Len(), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_from_previous_hour_new_day", func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

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
	fs.MustRemoveDir(path)
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
	fs.MustRemoveDir(path)
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

func TestStorageDeletePendingSeries(t *testing.T) {
	defer testRemoveAll(t)

	const numMonths = 10
	s := MustOpenStorage(t.Name(), OpenOptions{})

	var metricGroupName = []byte("metric")
	tfs := NewTagFilters()
	if err := tfs.Add(nil, metricGroupName, false, false); err != nil {
		t.Fatalf("cannot add tag filter: %s", err)
	}

	addRows := func(from, to time.Time, reverse bool) {
		t.Helper()

		var mn MetricName
		mn.Tags = []Tag{
			{[]byte("job"), []byte("job")},
		}
		mn.MetricGroup = metricGroupName
		metricNameRaw := mn.marshalRaw(nil)

		ts := from
		inc := 1
		if reverse {
			inc = -1
			ts = to
		}
		for {
			mr := MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     ts.UnixMilli(),
				Value:         1,
			}
			s.AddRows([]MetricRow{mr}, defaultPrecisionBits)
			ts = ts.AddDate(0, inc, 0)
			if ts.After(to) || ts.Before(from) {
				break
			}
		}
	}

	assertDeleteSeries := func(want int) {
		t.Helper()

		n, err := s.DeleteSeries(nil, []*TagFilters{tfs}, 1e5)
		if err != nil {
			t.Fatalf("error in DeleteSeries: %s", err)
		}

		if n != want {
			t.Fatalf("unexpected number of deleted series; got %d; want %d", n, want)
		}
	}

	assertCountMonthsWithLabels := func(count int) {
		t.Helper()

		ts := time.Unix(0, 0)
		n := 0
		for range numMonths {
			lns, err := s.SearchLabelNames(nil, nil, TimeRange{ts.UnixMilli(), ts.UnixMilli()}, 1e5, 1e9, noDeadline)
			if err != nil {
				t.Fatalf("error in SearchLabelNames: %s", err)
				return
			}
			if len(lns) != 0 {
				n++
			}
			ts = ts.AddDate(0, 1, 0)
		}

		if n != count {
			t.Fatalf("unexpected labels count; got %d; want %d", n, count)
		}
	}

	assertCountRows := func(count int) {
		t.Helper()

		var search Search
		defer search.MustClose()

		search.Init(nil, s, []*TagFilters{tfs}, TimeRange{0, math.MaxInt64}, 1e5, noDeadline)
		n := 0
		for search.NextMetricBlock() {
			var b Block
			search.MetricBlockRef.BlockRef.MustReadBlock(&b)
			n += b.RowsCount()
		}
		if err := search.Error(); err != nil {
			t.Fatalf("error in Search: %s", err)
		}
		if n != count {
			t.Fatalf("unexpected rows count; got %d; want %d", n, count)
		}
	}
	// Verify no metrics exist
	assertCountRows(0)

	start := time.Unix(0, 0)
	middle := start.AddDate(0, (numMonths-1)/2, 0)
	end := start.AddDate(0, numMonths-1, 0)

	// Add some rows and flush, so next DeleteSeries() can delete them
	addRows(start, middle, false)
	s.DebugFlush()

	// Add the rest of the rows – DeleteSeries() won't see them since they are not flushed yet
	addRows(middle.AddDate(0, 1, 0), end, false)

	assertDeleteSeries(1)

	// Verify metrics are partially deleted
	s.DebugFlush()
	assertCountRows(numMonths / 2)

	// Verify all deleted TSIDs are recreated. TSIDs should be deleted only for some subset of months in the beginning.
	// Add rows in reverse order to ensure that cache is not leaking between partitions.
	addRows(start, end, true)
	s.DebugFlush()
	assertCountMonthsWithLabels(numMonths)

	// Verify all metrics are present
	assertCountRows(numMonths/2 + numMonths)

	s.MustClose()
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
	fs.MustRemoveDir(path)
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

func TestStorageDeleteSeries_EmptyFilters(t *testing.T) {
	defer testRemoveAll(t)

	const numMetrics = 10
	mrs := make([]MetricRow, numMetrics)
	allMetricNames := make([]string, numMetrics)
	tr := TimeRange{
		MinTimestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2020, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	step := (tr.MaxTimestamp - tr.MinTimestamp) / numMetrics
	for i := range numMetrics {
		name := fmt.Sprintf("metric_%04d", i)
		mn := MetricName{
			MetricGroup: []byte(name),
		}
		mrs[i].MetricNameRaw = mn.marshalRaw(nil)
		mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
		mrs[i].Value = float64(i)
		allMetricNames[i] = name
	}

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	assertAllMetricNames := func(want []string) {
		tfs := NewTagFilters()
		if err := tfs.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		got, err := s.SearchMetricNames(nil, []*TagFilters{tfs}, tr, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
		}
		for i, name := range got {
			var mn MetricName
			if err := mn.UnmarshalString(name); err != nil {
				t.Fatalf("Could not unmarshal metric name %q: %v", name, err)
			}
			got[i] = string(mn.MetricGroup)
		}
		slices.Sort(got)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected metric names (-want, +got):\n%s", diff)
		}
	}

	// Confirm that metric names have been written to the index.
	assertAllMetricNames(allMetricNames)

	got, err := s.DeleteSeries(nil, []*TagFilters{}, 1e9)
	if err != nil {
		t.Fatalf("DeleteSeries() failed unexpectedly: %v", err)
	}
	if got != 0 {
		t.Fatalf("unexpected deleted series count: got %d, want 0", got)
	}

	// Ensure that metric names haven't been deleted.
	assertAllMetricNames(allMetricNames)
}

func TestStorageDeleteSeries_TooManyTimeseries(t *testing.T) {
	defer testRemoveAll(t)

	type options struct {
		tr         TimeRange
		numMetrics int
		maxMetrics int
		wantErr    bool
		wantCount  int
	}

	f := func(t *testing.T, opts *options) {
		t.Helper()

		rng := rand.New(rand.NewSource(1))
		mrs := testGenerateMetricRowsWithPrefix(rng, uint64(opts.numMetrics), "metric", opts.tr)
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte("metric.*"), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		got, err := s.DeleteSeries(nil, []*TagFilters{tfs}, opts.maxMetrics)
		if got := err != nil; got != opts.wantErr {
			t.Errorf("unmet error expectation: got %t, want %t", got, opts.wantErr)
		}
		if got != opts.wantCount {
			t.Errorf("unexpected deleted series count: got %d, want %d", got, opts.wantCount)
		}
	}

	// All ingested samples belong to a single month. In this case,
	// DeleteSeries() is expected to return an error because the number of
	// metrics registered within a single partition index is 1000 while the
	// number of metrics to delete at once is 999.
	t.Run("1m", func(t *testing.T) {
		f(t, &options{
			tr: TimeRange{
				MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
				MaxTimestamp: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC).UnixMilli(),
			},
			numMetrics: 1000,
			maxMetrics: 999,
			wantErr:    true,
		})
	})

	// All ingested samples belong to two months. In this case,
	// DeleteSeries() must delete the requested metrics because the 1000 metrics
	// is spread across two months and each month has roughly 500 metrics. Since
	// the number of metrics to delete at once (999) is applied per partition
	// index, the DeleteSeries() must succeed.
	t.Run("2m", func(t *testing.T) {
		f(t, &options{
			tr: TimeRange{
				MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
				MaxTimestamp: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC).UnixMilli(),
			},
			numMetrics: 1000,
			maxMetrics: 999,
			wantCount:  1000,
		})
	})
}

func TestStorageDeleteSeries_CachesAreUpdatedOrReset(t *testing.T) {
	defer testRemoveAll(t)

	month1 := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli(),
	}
	month2 := TimeRange{
		MinTimestamp: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC).UnixMilli(),
	}
	var mn MetricName
	mn.MetricGroup = []byte("metric1")
	mr1Month1 := MetricRow{
		MetricNameRaw: mn.marshalRaw(nil),
		Timestamp:     month1.MinTimestamp,
		Value:         123,
	}
	mn.MetricGroup = []byte("metric2")
	mr2Month2 := MetricRow{
		MetricNameRaw: mn.marshalRaw(nil),
		Timestamp:     month2.MinTimestamp,
		Value:         456,
	}
	mn.MetricGroup = []byte("metric3")
	mr3Month1 := MetricRow{
		MetricNameRaw: mn.marshalRaw(nil),
		Timestamp:     month1.MinTimestamp,
		Value:         789,
	}
	mr3Month2 := MetricRow{
		MetricNameRaw: mn.marshalRaw(nil),
		Timestamp:     month2.MinTimestamp,
		Value:         987,
	}

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	s.AddRows([]MetricRow{mr1Month1, mr2Month2, mr3Month1, mr3Month2}, defaultPrecisionBits)
	s.DebugFlush()

	tfss := func(metricNameRE string) []*TagFilters {
		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte(metricNameRE), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		return []*TagFilters{tfs}
	}
	tfssMetric1 := tfss("metric1")
	tfssMetric2 := tfss("metric2")
	tfssMetric3 := tfss("metric3")
	tfssMetric12 := tfss("metric(1|2)")
	tfssMetric123 := tfss("metric(1|2|3)")

	assertMetricNameCached := func(metricNameRaw []byte, want bool) {
		t.Helper()
		var v legacyTSID
		if got := s.getTSIDFromCache(&v, metricNameRaw); got != want {
			t.Errorf("unexpected %q metric name in TSID cache: got %t, want %t", string(metricNameRaw), got, want)
		}
	}
	assertTagFiltersCached := func(tfss []*TagFilters, tr TimeRange, want bool) {
		t.Helper()

		ptws := s.tb.GetPartitions(tr)
		defer s.tb.PutPartitions(ptws)

		if got, want := len(ptws), 1; got != want {
			t.Fatalf("unexpected partitions count for %v: got %d, want %d", &tr, got, want)
		}
		idb := ptws[0].pt.idb
		tfssTR := tr
		if idb.tr.MinTimestamp > tfssTR.MinTimestamp {
			tfssTR.MinTimestamp = idb.tr.MinTimestamp
		}
		if idb.tr.MaxTimestamp < tfssTR.MaxTimestamp {
			tfssTR.MaxTimestamp = idb.tr.MaxTimestamp
		}
		tfssKey := marshalTagFiltersKey(nil, tfss, tr)
		_, got := idb.getMetricIDsFromTagFiltersCache(nil, tfssKey)
		if got != want {
			t.Errorf("unexpected tag filters in cache %v %v: got %t, want %t", tfss, &tr, got, want)
		}
	}

	assertDeletedMetricIDsCacheSize := func(tr TimeRange, want int) {
		t.Helper()

		ptws := s.tb.GetPartitions(tr)
		defer s.tb.PutPartitions(ptws)

		if got, want := len(ptws), 1; got != want {
			t.Fatalf("unexpected partitions count for %v: got %d, want %d", &tr, got, want)
		}
		idb := ptws[0].pt.idb
		if got := idb.getDeletedMetricIDs().Len(); got != want {
			t.Fatalf("unexpected deletedMetricIDs cache size: got %d, want %d", got, want)
		}
	}

	// The data is inserted but never queried or deleted. Expect all three
	// metrics to be in TSID cache and expect TFSS and deletedMetricIDs caches
	// to be empty.
	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, false)
	assertTagFiltersCached(tfssMetric1, month2, false)
	assertTagFiltersCached(tfssMetric2, month1, false)
	assertTagFiltersCached(tfssMetric2, month2, false)
	assertTagFiltersCached(tfssMetric3, month1, false)
	assertTagFiltersCached(tfssMetric3, month2, false)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	searchMetricNames := func(tfss []*TagFilters, tr TimeRange, wantMetricCount int) {
		t.Helper()
		metrics, err := s.SearchMetricNames(nil, tfss, tr, 2, noDeadline)
		if err != nil {
			t.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
		}
		if got := len(metrics); got != wantMetricCount {
			t.Errorf("SearchMetricNames() unexpected metric count: got %v, want %v", got, wantMetricCount)
		}
	}

	// Search metric1 in month1. The search result must be cached for that tfss
	// for month1 but not for month2.
	searchMetricNames(tfssMetric1, month1, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, false)
	assertTagFiltersCached(tfssMetric2, month1, false)
	assertTagFiltersCached(tfssMetric2, month2, false)
	assertTagFiltersCached(tfssMetric3, month1, false)
	assertTagFiltersCached(tfssMetric3, month2, false)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric1 in month2. month2 does not contain metric1, but the
	// empty result is still cached for month2.
	searchMetricNames(tfssMetric1, month2, 0)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, false)
	assertTagFiltersCached(tfssMetric2, month2, false)
	assertTagFiltersCached(tfssMetric3, month1, false)
	assertTagFiltersCached(tfssMetric3, month2, false)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric2 in month1. month1 does not contain metric2, but the
	// empty result is still cached for month1.
	searchMetricNames(tfssMetric2, month1, 0)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, true)
	assertTagFiltersCached(tfssMetric2, month2, false)
	assertTagFiltersCached(tfssMetric3, month1, false)
	assertTagFiltersCached(tfssMetric3, month2, false)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric2 in month2. month2 contains metric2, therefore the tag
	// filters will be cached for month2.
	searchMetricNames(tfssMetric2, month2, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, true)
	assertTagFiltersCached(tfssMetric2, month2, true)
	assertTagFiltersCached(tfssMetric3, month1, false)
	assertTagFiltersCached(tfssMetric3, month2, false)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric3 in month1. Both month1 and 2 contain metric3;
	// however, the search time range is month1, therefore the tag
	// filters will be cached for month1 only.
	searchMetricNames(tfssMetric3, month1, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, true)
	assertTagFiltersCached(tfssMetric2, month2, true)
	assertTagFiltersCached(tfssMetric3, month1, true)
	assertTagFiltersCached(tfssMetric3, month2, false)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric3 in month2. Now the tag filters will also be cached for
	// month2.
	searchMetricNames(tfssMetric3, month2, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, true)
	assertTagFiltersCached(tfssMetric2, month2, true)
	assertTagFiltersCached(tfssMetric3, month1, true)
	assertTagFiltersCached(tfssMetric3, month2, true)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric1 or 2 in month1. The tag filters must be cached for
	// month1 only.
	searchMetricNames(tfssMetric12, month1, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, true)
	assertTagFiltersCached(tfssMetric2, month2, true)
	assertTagFiltersCached(tfssMetric3, month1, true)
	assertTagFiltersCached(tfssMetric3, month2, true)
	assertTagFiltersCached(tfssMetric12, month1, true)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric1 or 2 in month2. The tag filters must be also be cached
	// for month2.
	searchMetricNames(tfssMetric12, month2, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, true)
	assertTagFiltersCached(tfssMetric2, month2, true)
	assertTagFiltersCached(tfssMetric3, month1, true)
	assertTagFiltersCached(tfssMetric3, month2, true)
	assertTagFiltersCached(tfssMetric12, month1, true)
	assertTagFiltersCached(tfssMetric12, month2, true)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric1,2,3 in month1. The tag filters are cached
	// for month1 only.
	searchMetricNames(tfssMetric123, month1, 2)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, true)
	assertTagFiltersCached(tfssMetric2, month2, true)
	assertTagFiltersCached(tfssMetric3, month1, true)
	assertTagFiltersCached(tfssMetric3, month2, true)
	assertTagFiltersCached(tfssMetric12, month1, true)
	assertTagFiltersCached(tfssMetric12, month2, true)
	assertTagFiltersCached(tfssMetric123, month1, true)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Search for metric1,2,3 in month2. The tag filters are also cached
	// for month2.
	searchMetricNames(tfssMetric123, month2, 2)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, true)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, true)
	assertTagFiltersCached(tfssMetric2, month2, true)
	assertTagFiltersCached(tfssMetric3, month1, true)
	assertTagFiltersCached(tfssMetric3, month2, true)
	assertTagFiltersCached(tfssMetric12, month1, true)
	assertTagFiltersCached(tfssMetric12, month2, true)
	assertTagFiltersCached(tfssMetric123, month1, true)
	assertTagFiltersCached(tfssMetric123, month2, true)
	assertDeletedMetricIDsCacheSize(month1, 0)
	assertDeletedMetricIDsCacheSize(month2, 0)

	deleteSeries := func(tfss []*TagFilters, want int) {
		t.Helper()
		got, err := s.DeleteSeries(nil, tfss, 2)
		if err != nil {
			t.Fatalf("DeleteSeries() failed unexpectedly: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected deleted series count: got %d, want %d", got, want)
		}
	}

	// Delete metric1. TSID cache not must be cleared. Tag filters for month1
	// must be cleared but not for month2 because metric1 is in month1 only.
	// deletedMetricIDsCache size must be 1.
	deleteSeries(tfssMetric1, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, false)
	assertTagFiltersCached(tfssMetric1, month2, true)
	assertTagFiltersCached(tfssMetric2, month1, false)
	assertTagFiltersCached(tfssMetric2, month2, true)
	assertTagFiltersCached(tfssMetric3, month1, false)
	assertTagFiltersCached(tfssMetric3, month2, true)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, true)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, true)
	assertDeletedMetricIDsCacheSize(month1, 1)
	assertDeletedMetricIDsCacheSize(month2, 0)

	// Delete metric2. TSID cache not must be cleared. Tag filters for month2
	// must be cleared and deletedMetricIDsCache size for month2 must be 1.
	deleteSeries(tfssMetric2, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, false)
	assertTagFiltersCached(tfssMetric1, month2, false)
	assertTagFiltersCached(tfssMetric2, month1, false)
	assertTagFiltersCached(tfssMetric2, month2, false)
	assertTagFiltersCached(tfssMetric3, month1, false)
	assertTagFiltersCached(tfssMetric3, month2, false)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 1)
	assertDeletedMetricIDsCacheSize(month2, 1)

	// Delete metric3. TSID cache not must be cleared.
	// deletedMetricIDsCache size for month1 and 2 must be 2.
	deleteSeries(tfssMetric3, 1)

	assertMetricNameCached(mr1Month1.MetricNameRaw, true)
	assertMetricNameCached(mr2Month2.MetricNameRaw, true)
	assertMetricNameCached(mr3Month1.MetricNameRaw, true)
	assertTagFiltersCached(tfssMetric1, month1, false)
	assertTagFiltersCached(tfssMetric1, month2, false)
	assertTagFiltersCached(tfssMetric2, month1, false)
	assertTagFiltersCached(tfssMetric2, month2, false)
	assertTagFiltersCached(tfssMetric3, month1, false)
	assertTagFiltersCached(tfssMetric3, month2, false)
	assertTagFiltersCached(tfssMetric12, month1, false)
	assertTagFiltersCached(tfssMetric12, month2, false)
	assertTagFiltersCached(tfssMetric123, month1, false)
	assertTagFiltersCached(tfssMetric123, month2, false)
	assertDeletedMetricIDsCacheSize(month1, 2)
	assertDeletedMetricIDsCacheSize(month2, 2)
}

func TestStorageDeleteSeriesFromPrevAndCurrIndexDB(t *testing.T) {
	defer testRemoveAll(t)

	rng := rand.New(rand.NewSource(1))
	const numSeries = 100
	trPrev := TimeRange{
		MinTimestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2020, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrsPrev := testGenerateMetricRowsWithPrefix(rng, numSeries, "prev", trPrev)
	trCurr := TimeRange{
		MinTimestamp: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2020, 1, 2, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrsCurr := testGenerateMetricRowsWithPrefix(rng, numSeries, "curr", trCurr)
	trPt := TimeRange{
		MinTimestamp: time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2020, 1, 3, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrsPt := testGenerateMetricRowsWithPrefix(rng, numSeries, "pt", trPt)
	deleteSeries := func(s *Storage, want, wantTotal int) {
		t.Helper()
		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte(".*"), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		got, err := s.DeleteSeries(nil, []*TagFilters{tfs}, 1e9)
		if err != nil {
			t.Fatalf("could not delete series unexpectedly: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected number of deleted series: got %d, want %d", got, want)
		}
		var m Metrics
		s.UpdateMetrics(&m)
		if got, want := m.DeletedMetricsCount, uint64(wantTotal); got != want {
			t.Fatalf("unexpected number of total deleted series: got %d, want %d", got, want)
		}

	}

	s := MustOpenStorage(t.Name(), OpenOptions{})

	// legacy prev idb
	s.AddRows(mrsPrev, defaultPrecisionBits)
	s.DebugFlush()
	deleteSeries(s, numSeries, numSeries)
	s = mustConvertToLegacy(s)

	// legacy curr idb
	s.AddRows(mrsCurr, defaultPrecisionBits)
	s.DebugFlush()
	deleteSeries(s, numSeries, 2*numSeries)
	s = mustConvertToLegacy(s)

	// pt idb
	s.AddRows(mrsPt, defaultPrecisionBits)
	s.DebugFlush()
	deleteSeries(s, numSeries, 3*numSeries)

	s.MustClose()
}

func TestStorageRegisterMetricNamesSerial(t *testing.T) {
	path := "TestStorageRegisterMetricNamesSerial"
	s := MustOpenStorage(path, OpenOptions{})
	if err := testStorageRegisterMetricNames(s); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	s.MustClose()
	fs.MustRemoveDir(path)
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
	fs.MustRemoveDir(path)
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
	lns, err := s.SearchLabelNames(nil, nil, TimeRange{0, math.MaxInt64}, 100, 1e9, noDeadline)
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
	addIDs, err := s.SearchLabelValues(nil, "add_id", nil, TimeRange{0, math.MaxInt64}, addsCount+100, 1e9, noDeadline)
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
	fs.MustRemoveDir(path)
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
	fs.MustRemoveDir(path)
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
	snapshotName := s.MustCreateSnapshot()

	// Verify the snapshot is visible
	snapshots := s.MustListSnapshots()
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
	ptws := s1.tb.GetAllPartitions(nil)
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
	snapshots = s.MustListSnapshots()
	if containsString(snapshots, snapshotName) {
		return fmt.Errorf("snapshot %q must be deleted, but is still visible in %q", snapshotName, snapshots)
	}

	return nil
}

// testListDirEntries returns the all paths inside `root` dir. The `root` dir
// itself and paths that start with `ignorePrefix` are omitted.
func testListDirEntries(t *testing.T, root string, ignorePrefix ...string) []string {
	t.Helper()
	var paths []string
	f := func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		for _, prefix := range ignorePrefix {
			if strings.HasPrefix(path, prefix) {
				return nil
			}
		}
		paths = append(paths, strings.TrimPrefix(path, root))
		return nil
	}
	if err := filepath.WalkDir(root, f); err != nil {
		t.Fatalf("could not walk dir %q: %v", root, err)
	}
	return paths
}

func TestStorageSnapshots_CreateListDelete(t *testing.T) {
	defer testRemoveAll(t)

	rng := rand.New(rand.NewSource(1))
	const numRows = 10000
	minTimestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	maxTimestamp := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC).UnixMilli()
	mrs := testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp)

	root := t.Name()
	s := MustOpenStorage(root, OpenOptions{})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	snapshotName := s.MustCreateSnapshot()
	assertListSnapshots := func(want []string) {
		got := s.MustListSnapshots()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected snapshot list (-want, +got):\n%s", diff)
		}
	}
	assertListSnapshots([]string{snapshotName})

	// Check snapshot dir entries

	var (
		data           = filepath.Join(root, dataDirname)
		smallData      = filepath.Join(data, smallDirname)
		bigData        = filepath.Join(data, bigDirname)
		indexData      = filepath.Join(data, indexdbDirname)
		smallSnapshots = filepath.Join(smallData, snapshotsDirname)
		bigSnapshots   = filepath.Join(bigData, snapshotsDirname)
		indexSnapshots = filepath.Join(indexData, snapshotsDirname)
		smallSnapshot  = filepath.Join(smallSnapshots, snapshotName)
		bigSnapshot    = filepath.Join(bigSnapshots, snapshotName)
		indexSnapshot  = filepath.Join(indexSnapshots, snapshotName)
	)

	assertDirEntries := func(srcDir, snapshotDir string, excludePath ...string) {
		t.Helper()
		dataDirEntries := testListDirEntries(t, srcDir, excludePath...)
		snapshotDirEntries := testListDirEntries(t, snapshotDir)
		if diff := cmp.Diff(dataDirEntries, snapshotDirEntries); diff != "" {
			t.Fatalf("unexpected snapshot dir entries (-want, +got):\n%s", diff)
		}
	}
	assertDirEntries(smallData, smallSnapshot, smallSnapshots)
	assertDirEntries(bigData, bigSnapshot, bigSnapshots)
	assertDirEntries(indexData, indexSnapshot, indexSnapshots)

	// Check snapshot symlinks

	var (
		snapshot     = filepath.Join(root, snapshotsDirname, snapshotName)
		bigSymlink   = filepath.Join(snapshot, dataDirname, bigDirname)
		smallSymlink = filepath.Join(snapshot, dataDirname, smallDirname)
		indexSymlink = filepath.Join(snapshot, dataDirname, indexdbDirname)
	)
	assertSymlink := func(symlink string, wantRealpath string) {
		t.Helper()
		gotRealpath, err := filepath.EvalSymlinks(symlink)
		if err != nil {
			t.Fatalf("Could not evaluate symlink %q: %v", symlink, err)
		}
		if gotRealpath != wantRealpath {
			t.Fatalf("unexpected realpath for symlink %q: got %q, want %q", symlink, gotRealpath, wantRealpath)
		}
	}
	assertSymlink(bigSymlink, bigSnapshot)
	assertSymlink(smallSymlink, smallSnapshot)
	assertSymlink(indexSymlink, indexSnapshot)

	// Check snapshot deletion.

	if err := s.DeleteSnapshot(snapshotName); err != nil {
		t.Fatalf("could not delete snapshot %q: %v", snapshotName, err)
	}
	assertListSnapshots([]string{})

	assertPathDoesNotExist := func(path string) {
		t.Helper()
		if fs.IsPathExist(path) {
			t.Fatalf("path was not expected to exist: %q", path)
		}
	}
	assertPathDoesNotExist(snapshot)
	assertPathDoesNotExist(bigSnapshot)
	assertPathDoesNotExist(smallSnapshot)
	assertPathDoesNotExist(indexSnapshot)
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
	snapshotName := s.MustCreateSnapshot()

	// Delete snapshots older than 1 month
	s.MustDeleteStaleSnapshots(30 * 24 * time.Hour)

	snapshots := s.MustListSnapshots()
	if len(snapshots) != 1 {
		t.Fatalf("expecting one snapshot; got %q", snapshots)
	}
	if snapshots[0] != snapshotName {
		t.Fatalf("snapshot %q is missing in %q", snapshotName, snapshots)
	}

	// Delete the snapshot which is older than 1 nanoseconds
	time.Sleep(2 * time.Nanosecond)
	s.MustDeleteStaleSnapshots(time.Nanosecond)

	snapshots = s.MustListSnapshots()
	if len(snapshots) != 0 {
		t.Fatalf("expecting zero snapshots; got %q", snapshots)
	}
	s.MustClose()
	fs.MustRemoveDir(path)
}

// testRemoveAll removes all storage data produced by a test if the test hasn't
// failed. For this to work, the storage must use t.Name() as the base dir in
// its data path.
//
// In case of failure, the data is kept for further debugging.
func testRemoveAll(t *testing.T) {
	defer func() {
		if !t.Failed() {
			fs.MustRemoveDir(t.Name())
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

func TestStorageSearchMetricNames_VariousTimeRanges(t *testing.T) {
	defer testRemoveAll(t)

	const numMetrics = 10000

	f := func(t *testing.T, tr TimeRange) {
		t.Helper()

		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("metric_%d", i)
			mn := MetricName{MetricGroup: []byte(name)}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = name
		}
		slices.Sort(want)

		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		tfss := NewTagFilters()
		if err := tfss.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		got, err := s.SearchMetricNames(nil, []*TagFilters{tfss}, tr, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
		}
		for i, name := range got {
			var mn MetricName
			if err := mn.UnmarshalString(name); err != nil {
				t.Fatalf("Could not unmarshal metric name %q: %v", name, err)
			}
			got[i] = string(mn.MetricGroup)
		}
		slices.Sort(got)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected metric names (-want, +got):\n%s", diff)
		}
	}

	testStorageOpOnVariousTimeRanges(t, f)
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
			t.Errorf("SearchMetricNames(%v, %v, %d): unexpected error: got %v, want error to happen %v", []any{
				tfss, &opts.tr, opts.maxMetrics, err, opts.wantErr}...)
		}
		if got := len(names); got != opts.wantCount {
			t.Errorf("SearchMetricNames(%v, %v, %d): unexpected metric name count: got %d, want %d", []any{
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

	// Using one filter to search metric names within 41 days. The maxMetrics
	// param is set to match exactly the number of time series that match the
	// filter within that time range. Search operation must complete
	// successfully.
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

	// Using one filter to search metric names within 42 days. The maxMetrics
	// param is set to match exactly the number of time series that match the
	// filter within that time range. Search operation must complete
	// successfully.
	f(&options{
		path:    "40Days/OneTagFilter/MaxMetricsNotExeeded",
		filters: []string{"metric1"},
		tr: TimeRange{
			MinTimestamp: days[0].MinTimestamp,
			MaxTimestamp: days[41].MaxTimestamp,
		},
		maxMetrics: numRows * 42,
		wantCount:  numRows * 42,
	})
}

func TestStorageSearchLabelNames_VariousTimeRanges(t *testing.T) {
	defer testRemoveAll(t)

	const numRows = 10000

	f := func(t *testing.T, tr TimeRange) {
		t.Helper()

		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			MetricGroup: []byte("metric"),
			Tags: []Tag{
				{
					Key:   []byte("tbd"),
					Value: []byte("value"),
				},
			},
		}
		for i := range numRows {
			labelName := fmt.Sprintf("label_%d", i)
			mn.Tags[0].Key = []byte(labelName)
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = labelName
		}
		want = append(want, "__name__")
		slices.Sort(want)

		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		got, err := s.SearchLabelNames(nil, nil, tr, 1e9, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchLabelNames() failed unexpectedly: %v", err)
		}
		slices.Sort(got)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected label names (-want, +got):\n%s", diff)
		}
	}

	testStorageOpOnVariousTimeRanges(t, f)
}

func TestStorageSearchLabelValues_VariousTimeRanges(t *testing.T) {
	defer testRemoveAll(t)

	const numRows = 10000

	f := func(t *testing.T, tr TimeRange) {
		t.Helper()

		mrs := make([]MetricRow, numRows)
		want := make([]string, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			MetricGroup: []byte("metric"),
			Tags: []Tag{
				{
					Key:   []byte("label"),
					Value: []byte("tbd"),
				},
			},
		}
		for i := range numRows {
			labelValue := fmt.Sprintf("value_%d", i)
			mn.Tags[0].Value = []byte(labelValue)
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = labelValue
		}
		slices.Sort(want)

		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		got, err := s.SearchLabelValues(nil, "label", nil, tr, 1e9, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchLabelValues() failed unexpectedly: %v", err)
		}
		slices.Sort(got)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected label values (-want, +got):\n%s", diff)
		}
	}

	testStorageOpOnVariousTimeRanges(t, f)
}

func TestStorageSearchTagValueSuffixes_VariousTimeRanges(t *testing.T) {
	defer testRemoveAll(t)

	const numMetrics = 10000

	f := func(t *testing.T, tr TimeRange) {
		t.Helper()

		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("prefix.metric%04d", i)
			mn := MetricName{MetricGroup: []byte(name)}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = fmt.Sprintf("metric%04d", i)
		}
		slices.Sort(want)

		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		got, err := s.SearchTagValueSuffixes(nil, tr, "", "prefix.", '.', 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchTagValueSuffixes() failed unexpectedly: %v", err)
		}
		slices.Sort(got)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected tag value suffixes (-want, +got):\n%s", diff)
		}
	}

	testStorageOpOnVariousTimeRanges(t, f)
}

func TestStorageSearchGraphitePaths_VariousTimeRanges(t *testing.T) {
	defer testRemoveAll(t)

	f := func(t *testing.T, tr TimeRange) {
		t.Helper()

		const numMetrics = 10000
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("prefix.metric%04d", i)
			mn := MetricName{MetricGroup: []byte(name)}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = tr.MinTimestamp + int64(i)*step
			mrs[i].Value = float64(i)
			want[i] = name
		}
		slices.Sort(want)

		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		got, err := s.SearchGraphitePaths(nil, tr, []byte("*.*"), 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchTagGraphitePaths() failed unexpectedly: %v", err)
		}
		slices.Sort(got)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected graphite paths (-want, +got):\n%s", diff)
		}
	}

	testStorageOpOnVariousTimeRanges(t, f)
}

// testStorageOpOnVariousTimeRanges executes some storage operation on various
// time ranges: 1h, 1d, 1m, etc.
func testStorageOpOnVariousTimeRanges(t *testing.T, op func(t *testing.T, tr TimeRange)) {
	t.Helper()

	t.Run("1h", func(t *testing.T) {
		op(t, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC).UnixMilli(),
		})
	})
	t.Run("1d", func(t *testing.T) {
		op(t, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	t.Run("1m", func(t *testing.T) {
		op(t, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
	t.Run("1y", func(t *testing.T) {
		op(t, TimeRange{
			MinTimestamp: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2000, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		})
	})
}

func TestStorageSearchLabelValues_EmptyValuesAreNotReturned(t *testing.T) {
	defer testRemoveAll(t)

	const numRows = 1000

	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := make([]MetricRow, numRows)
	want := make([]string, numRows)

	for i := range numRows {
		metricName := fmt.Sprintf("metric_%03d", i)
		labelValue := fmt.Sprintf("value_%03d", i)
		mn := MetricName{
			MetricGroup: []byte(metricName),
			Tags: []Tag{
				{
					Key:   []byte("label_with_empty_value"),
					Value: []byte(""),
				},
				{
					Key:   []byte("label_with_non_empty_value"),
					Value: []byte(labelValue),
				},
			},
		}

		mrs[i].MetricNameRaw = mn.marshalRaw(nil)
		mrs[i].Timestamp = tr.MinTimestamp + rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp)
		mrs[i].Value = float64(i)
		want[i] = labelValue
	}

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	assertSearchLabelValues := func(labelName string, want []string) {
		got, err := s.SearchLabelValues(nil, labelName, nil, tr, 1e9, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchLabelValues() failed unexpectedly: %v", err)
		}
		slices.Sort(got)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected label values (-want, +got):\n%s", diff)
		}
	}

	// First, ensure that non-empty label values are returned.
	assertSearchLabelValues("label_with_non_empty_value", want)

	// Now verify that empty label values are not returned.
	assertSearchLabelValues("label_with_empty_value", []string{})
}

func TestStorageGetSeriesCount(t *testing.T) {
	defer testRemoveAll(t)

	// Inserts the numMetrics of the same metrics for each time range from trs
	// and then gets the series count and compares it with wanted value.
	f := func(numMetrics int, trs []TimeRange, want uint64) {
		t.Helper()

		mrs := make([]MetricRow, numMetrics)
		for i := range numMetrics {
			metricName := fmt.Sprintf("metric_%d", i)
			mn := MetricName{
				MetricGroup: []byte(metricName),
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Value = float64(i)
		}

		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		for _, tr := range trs {
			for j := range mrs {
				mrs[j].Timestamp = tr.MinTimestamp + rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp)
			}
			s.AddRows(mrs, defaultPrecisionBits)
		}
		s.DebugFlush()

		got, err := s.GetSeriesCount(noDeadline)
		if err != nil {
			t.Fatalf("GetSeriesCount() failed unexpectedly: %v", err)
		}
		if got != want {
			t.Errorf("unexpected series count: got %d, want %d", got, want)
		}
	}

	const numMetrics = 100
	month := func(m int) TimeRange {
		return TimeRange{
			MinTimestamp: time.Date(2024, time.Month(m), 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(2024, time.Month(m), 20, 0, 0, 0, 0, time.UTC).UnixMilli(),
		}
	}
	var want uint64

	oneMonth := []TimeRange{month(1)}
	// no index inflation since the metrics are inserted only to one indexDB
	want = numMetrics
	f(numMetrics, oneMonth, want)

	twoMonths := []TimeRange{month(1), month(2)}
	// index inflation since the same metrics are inserted into two partitions.
	want = numMetrics * 2
	f(numMetrics, twoMonths, want)

	fourMonths := []TimeRange{month(1), month(2), month(3), month(4)}
	// index inflation since the same metrics are inserted into four partitions.
	want = numMetrics * 4
	f(numMetrics, fourMonths, want)
}

func TestStorageGetTSDBStatus(t *testing.T) {
	defer testRemoveAll(t)

	const (
		numLabelNames    = 50
		numLabelValues   = 30
		numMetricNames   = numLabelNames * numLabelValues
		focusLabel       = "label_0000"
		nameValueRepeats = 10 // greatest common divisor
		valuesPerName    = numLabelValues / nameValueRepeats
	)

	mrs := make([]MetricRow, numMetricNames)
	tr := TimeRange{
		MinTimestamp: time.Date(2025, 1, 13, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 13, 23, 59, 59, 0, time.UTC).UnixMilli(),
	}
	date := uint64(tr.MinTimestamp / msecPerDay)
	for i := range numMetricNames {
		metricName := fmt.Sprintf("metric_%04d", i)
		labelName := fmt.Sprintf("label_%04d", i%numLabelNames)
		labelValue := fmt.Sprintf("value_%04d", i%numLabelValues)
		mn := MetricName{
			MetricGroup: []byte(metricName),
			Tags: []Tag{
				{
					Key:   []byte(labelName),
					Value: []byte(labelValue),
				},
			},
		}
		mrs[i].MetricNameRaw = mn.marshalRaw(nil)
		mrs[i].Timestamp = tr.MinTimestamp + rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp)
		mrs[i].Value = float64(i)
	}

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	var got, want *TSDBStatus

	// Check the date on which there is no data.
	got, err := s.GetTSDBStatus(nil, nil, date-1, "", 6, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("GetTSDBStatus() failed unexpectedly: %v", err)
	}
	want = &TSDBStatus{
		SeriesCountByMetricName:      []TopHeapEntry{},
		SeriesCountByLabelName:       []TopHeapEntry{},
		SeriesCountByFocusLabelValue: []TopHeapEntry{},
		SeriesCountByLabelValuePair:  []TopHeapEntry{},
		LabelValueCountByLabelName:   []TopHeapEntry{},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected label values (-want, +got):\n%s", diff)
	}

	// Check the date on which there is data.
	got, err = s.GetTSDBStatus(nil, nil, date, "label_0000", 6, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("GetTSDBStatus() failed unexpectedly: %v", err)
	}
	want = &TSDBStatus{
		TotalSeries:          numMetricNames,
		TotalLabelValuePairs: numMetricNames + numLabelNames*numLabelValues,
		SeriesCountByMetricName: []TopHeapEntry{
			{Name: "metric_0000", Count: 1},
			{Name: "metric_0001", Count: 1},
			{Name: "metric_0002", Count: 1},
			{Name: "metric_0003", Count: 1},
			{Name: "metric_0004", Count: 1},
			{Name: "metric_0005", Count: 1},
		},
		SeriesCountByLabelName: []TopHeapEntry{
			{Name: "__name__", Count: numMetricNames},
			{Name: "label_0000", Count: numLabelValues},
			{Name: "label_0001", Count: numLabelValues},
			{Name: "label_0002", Count: numLabelValues},
			{Name: "label_0003", Count: numLabelValues},
			{Name: "label_0004", Count: numLabelValues},
		},
		SeriesCountByFocusLabelValue: []TopHeapEntry{
			{Name: "value_0000", Count: nameValueRepeats},
			{Name: "value_0010", Count: nameValueRepeats},
			{Name: "value_0020", Count: nameValueRepeats},
		},
		SeriesCountByLabelValuePair: []TopHeapEntry{
			{Name: "label_0000=value_0000", Count: nameValueRepeats},
			{Name: "label_0000=value_0010", Count: nameValueRepeats},
			{Name: "label_0000=value_0020", Count: nameValueRepeats},
			{Name: "label_0001=value_0001", Count: nameValueRepeats},
			{Name: "label_0001=value_0011", Count: nameValueRepeats},
			{Name: "label_0001=value_0021", Count: nameValueRepeats},
		},
		LabelValueCountByLabelName: []TopHeapEntry{
			{Name: "__name__", Count: numMetricNames},
			{Name: "label_0000", Count: valuesPerName},
			{Name: "label_0001", Count: valuesPerName},
			{Name: "label_0002", Count: valuesPerName},
			{Name: "label_0003", Count: valuesPerName},
			{Name: "label_0004", Count: valuesPerName},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected label values (-want, +got):\n%s", diff)
	}
}

func TestStorageAdjustTimeRange(t *testing.T) {
	defer testRemoveAll(t)

	f := func(disablePerDayIndex bool, searchTR, idbTR, want TimeRange) {
		t.Helper()

		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: disablePerDayIndex,
		})
		defer s.MustClose()
		if got := s.adjustTimeRange(searchTR, idbTR); got != want {
			t.Errorf("unexpected time range: got %v, want %v", &got, &want)
		}
	}

	legacyIDBTimeRange := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: math.MaxInt64,
	}
	partitionIDBTimeRange := TimeRange{
		MinTimestamp: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 2, 28, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	var searchTimeRange TimeRange

	// Zero search time range is adjusted to globalIndexTimeRange regardless
	// whether the -disablePerDayIndex flag is set or not.
	searchTimeRange = TimeRange{}
	f(false, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(false, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)

	// The search time range is smaller than a month (and therefore < 40 days)
	// and is fully included into the partition idb time range.
	// If -disablePerDayIndex is set, the effective search time range is
	// expected to be globalIndexTimeRange. Otherwise it must remain the same
	// after the adjustment.
	searchTimeRange = TimeRange{
		MinTimestamp: partitionIDBTimeRange.MinTimestamp + msecPerDay,
		MaxTimestamp: partitionIDBTimeRange.MaxTimestamp - msecPerDay,
	}
	f(false, searchTimeRange, legacyIDBTimeRange, searchTimeRange)
	f(false, searchTimeRange, partitionIDBTimeRange, searchTimeRange)
	f(true, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)

	// The search time range is the same as partition idb time range.
	// If -disablePerDayIndex is set, the effective search time range is
	// expected to be globalIndexTimeRange for both legacy and parition idb.
	// Otherwise:
	// - For the legacy idb: it must remain the same
	// - For the partition idb: it must be replaced with globalIndexTimeRange.
	searchTimeRange = partitionIDBTimeRange
	f(false, searchTimeRange, legacyIDBTimeRange, searchTimeRange)
	f(false, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)

	// The search time range is smaller than 40 days and fully includes the
	// partition idb time range.
	// If -disablePerDayIndex is set, the effective search time range is
	// expected to be globalIndexTimeRange for both legacy and parition idb.
	// Otherwise:
	// - For the legacy idb: it must remain the same
	// - For the partition idb: it must be replaced with globalIndexTimeRange.
	searchTimeRange = TimeRange{
		MinTimestamp: partitionIDBTimeRange.MinTimestamp - msecPerDay,
		MaxTimestamp: partitionIDBTimeRange.MaxTimestamp + msecPerDay,
	}
	f(false, searchTimeRange, legacyIDBTimeRange, searchTimeRange)
	f(false, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)

	// The search time range is 41 days and fully includes the partition idb
	// time range.
	// If -disablePerDayIndex is set, the effective search time range is
	// expected to be globalIndexTimeRange for both legacy and parition idb.
	// Otherwise it must be replaced with globalIndexTimeRange for both legacy
	// and partition idbs.
	searchTimeRange = TimeRange{
		MinTimestamp: partitionIDBTimeRange.MinTimestamp - msecPerDay,
		MaxTimestamp: partitionIDBTimeRange.MinTimestamp + 41*msecPerDay,
	}
	f(false, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(false, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)

	// The search time range is smaller than 40 days and overlaps with partition
	// idb time range on the left.
	// If -disablePerDayIndex is set, the effective search time range is
	// expected to be globalIndexTimeRange for both legacy and parition idb.
	// Otherwise:
	// - For the legacy idb: it must remain the same
	// - For the partition idb: the MinTimestamp must be adjusted to match the
	// partition idb time range MinTimestamp.
	searchTimeRange = TimeRange{
		MinTimestamp: partitionIDBTimeRange.MinTimestamp - msecPerDay,
		MaxTimestamp: partitionIDBTimeRange.MinTimestamp + msecPerDay,
	}
	f(false, searchTimeRange, legacyIDBTimeRange, searchTimeRange)
	f(false, searchTimeRange, partitionIDBTimeRange, TimeRange{
		MinTimestamp: partitionIDBTimeRange.MinTimestamp,
		MaxTimestamp: searchTimeRange.MaxTimestamp,
	})
	f(true, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)

	// The search time range is smaller than 40 days and overlaps with partition
	// idb time range on the right.
	// If -disablePerDayIndex is set, the effective search time range is
	// expected to be globalIndexTimeRange for both legacy and parition idb.
	// Otherwise:
	// - For the legacy idb, it must remain the same
	// - For the partition idb: its MaxTimestamp must be adjusted to match the
	//   partition idb time range MaxTimestamp.
	searchTimeRange = TimeRange{
		MinTimestamp: partitionIDBTimeRange.MaxTimestamp - msecPerDay,
		MaxTimestamp: partitionIDBTimeRange.MaxTimestamp + msecPerDay,
	}
	f(false, searchTimeRange, legacyIDBTimeRange, searchTimeRange)
	f(false, searchTimeRange, partitionIDBTimeRange, TimeRange{
		MinTimestamp: searchTimeRange.MinTimestamp,
		MaxTimestamp: partitionIDBTimeRange.MaxTimestamp,
	})
	f(true, searchTimeRange, legacyIDBTimeRange, globalIndexTimeRange)
	f(true, searchTimeRange, partitionIDBTimeRange, globalIndexTimeRange)
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
		slices.Sort(got)
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
		wantEmpty:        []string{},
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

// testSearchMetricIDs returns metricIDs for the given tfss and tr.
//
// The returned metricIDs are sorted. The function panics in in case of error.
// The function is not a part of Storage because it is currently used in unit
// tests only.
func testSearchMetricIDs(s *Storage, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) []uint64 {
	search := func(qt *querytracer.Tracer, idb *indexDB, tr TimeRange) (*uint64set.Set, error) {
		return idb.searchMetricIDs(qt, tfss, tr, maxMetrics, deadline)
	}
	merge := func(data []*uint64set.Set) *uint64set.Set {
		all := &uint64set.Set{}
		for _, d := range data {
			all.Union(d)
		}
		return all
	}
	metricIDs, err := searchAndMerge(nil, s, tr, search, merge)
	if err != nil {
		panic(fmt.Sprintf("searching metricIDs failed unexpectedly: %s", err))
	}
	return metricIDs.AppendTo(nil)
}

// testCountAllMetricIDs is a test helper function that counts the IDs of
// all time series within the given time range.
func testCountAllMetricIDs(s *Storage, tr TimeRange) int {
	tfsAll := NewTagFilters()
	if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		panic(fmt.Sprintf("unexpected error in TagFilters.Add: %v", err))
	}
	ids := testSearchMetricIDs(s, []*TagFilters{tfsAll}, tr, 1e9, noDeadline)
	return len(ids)
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
		// The TestStorageAddRowsForVariousDataPatternsConcurrently/perDayIndexes/serial/sameBatchMetrics/sameRowMetrics/sameBatchDates/diffRowDates
		// test fails once the indexDB is rotated. This happens reliably when the number
		// of CPUs is 1. See: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8654.
		//
		// With the higher number of CPUs this failure is very rare.
		// Temporarily relax the strict equality requirement for got and want
		// data until this fixed. It is known why the test is failing but the
		// fix may be non-trivial, See: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8948
		strict := concurrency == 1 && runtime.NumCPU() > 1
		rowsAddedTotal := wantCounts.metrics.RowsAddedTotal

		s := MustOpenStorage(t.Name(), OpenOptions{
			DisablePerDayIndex: disablePerDayIndex,
		})

		testDoConcurrently(s, op, concurrency, splitBatches, batches)
		s.DebugFlush()
		assertCounts(t, s, wantCounts, strict)

		// TODO(rtm0): Add a case when a metricID is present in TSID cache but
		// not in partition idb.

		// Empty the tsidCache to test the case when tsid is retrieved from the
		// index.
		s.resetAndSaveTSIDCache()
		testDoConcurrently(s, op, concurrency, splitBatches, batches)
		s.DebugFlush()
		wantCounts.metrics.RowsAddedTotal += rowsAddedTotal
		assertCounts(t, s, wantCounts, strict)

		// TODO(rtm0): Add a case when a metricID is present in legacy IDB but
		// not in partition idb.

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

	roundToMonth := func(ts int64) int64 {
		t := time.UnixMilli(ts).UTC()
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	}
	// Need to count metric names per month because we now have a separate
	// indexDB per partition.
	monthNames := make(map[int64]map[string]bool)

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
			month := roundToMonth(tr.MinTimestamp)
			if monthNames[month] == nil {
				monthNames[month] = make(map[string]bool)
			}
			names[rowMetricName] = true
			trNames[tr][rowMetricName] = true
			monthNames[month][rowMetricName] = true

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

	// When RegisterMetricNames() is called it only registers the time series
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
			month := roundToMonth(tr.MinTimestamp)
			count = len(monthNames[month])
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

func TestStorageSearchTagValueSuffixes_maxTagValueSuffixes(t *testing.T) {
	defer testRemoveAll(t)

	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	const numMetrics = 1000
	mrs := testGenerateMetricRowsWithPrefix(rng, numMetrics, "metric.", tr)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	assertSuffixCount := func(maxTagValueSuffixes, want int) {
		suffixes, err := s.SearchTagValueSuffixes(nil, tr, "", "metric.", '.', maxTagValueSuffixes, noDeadline)
		if err != nil {
			t.Fatalf("SearchTagValueSuffixes() failed unexpectedly: %v", err)
		}

		if got := len(suffixes); got != want {
			t.Fatalf("unexpected tag value suffix count: got %d, want %d", got, want)
		}
	}

	// First, check that all the suffixes are returned if the limit is higher
	// than numMetrics.
	maxTagValueSuffixes := numMetrics + 1
	wantCount := numMetrics
	assertSuffixCount(maxTagValueSuffixes, wantCount)

	// Now set the max value to one that is smaller than numMetrics. The search
	// result must contain exactly that many suffixes.
	maxTagValueSuffixes = numMetrics / 10
	wantCount = maxTagValueSuffixes
	assertSuffixCount(maxTagValueSuffixes, wantCount)
}

// TestStorageMetrics_IndexDBBlockCaches checks that indexDB block cache metrics
// are collected only once even though there can be more than one indexDB. The
// reason for this is that block caches are shared between all indexDB instances
// and their metrics must be collected only once regardless how many indexDBs
// the storage has.
func TestStorageMetrics_IndexDBBlockCaches(t *testing.T) {
	defer testRemoveAll(t)

	assertMetric := func(name string, got, want uint64) {
		t.Helper()
		if got != want {
			t.Fatalf("unexpected %s value: got %d, want %d", name, got, want)
		}
	}

	assertMetrics := func(s *Storage) {
		t.Helper()

		ptw := s.tb.MustGetPartition(time.Now().UnixMilli())
		defer s.tb.PutPartition(ptw)
		idb := ptw.pt.idb

		var storageMetrics Metrics
		s.UpdateMetrics(&storageMetrics)
		got := storageMetrics.TableMetrics.IndexDBMetrics
		// Block cache metrics are the same for every indexDB, thus use block
		// cache metrics from idb for the current month.
		var want IndexDBMetrics
		idb.UpdateMetrics(&want)

		assertMetric("DataBlocksCacheSize", got.DataBlocksCacheSize, want.DataBlocksCacheSize)
		assertMetric("DataBlocksCacheSizeBytes", got.DataBlocksCacheSizeBytes, want.DataBlocksCacheSizeBytes)
		assertMetric("DataBlocksCacheSizeMaxBytes", got.DataBlocksCacheSizeMaxBytes, want.DataBlocksCacheSizeMaxBytes)
		assertMetric("DataBlocksCacheRequests", got.DataBlocksCacheRequests, want.DataBlocksCacheRequests)
		assertMetric("DataBlocksCacheMisses", got.DataBlocksCacheMisses, want.DataBlocksCacheMisses)
		assertMetric("DataBlocksSparseCacheSize", got.DataBlocksSparseCacheSize, want.DataBlocksSparseCacheSize)
		assertMetric("DataBlocksSparseCacheSizeBytes", got.DataBlocksSparseCacheSizeBytes, want.DataBlocksSparseCacheSizeBytes)
		assertMetric("DataBlocksSparseCacheSizeMaxBytes", got.DataBlocksSparseCacheSizeMaxBytes, want.DataBlocksSparseCacheSizeMaxBytes)
		assertMetric("DataBlocksSparseCacheRequests", got.DataBlocksSparseCacheRequests, want.DataBlocksSparseCacheRequests)
		assertMetric("DataBlocksSparseCacheMisses", got.DataBlocksSparseCacheMisses, want.DataBlocksSparseCacheMisses)
		assertMetric("IndexBlocksCacheSize", got.IndexBlocksCacheSize, want.IndexBlocksCacheSize)
		assertMetric("IndexBlocksCacheSizeBytes", got.IndexBlocksCacheSizeBytes, want.IndexBlocksCacheSizeBytes)
		assertMetric("IndexBlocksCacheSizeMaxBytes", got.IndexBlocksCacheSizeMaxBytes, want.IndexBlocksCacheSizeMaxBytes)
		assertMetric("IndexBlocksCacheRequests", got.IndexBlocksCacheRequests, want.IndexBlocksCacheRequests)
		assertMetric("IndexBlocksCacheMisses", got.IndexBlocksCacheMisses, want.IndexBlocksCacheMisses)
	}

	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Now().UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()

	// Check metrics right after the storage was opened.
	assertMetrics(s)

	// Check metrics right after the data was ingested.
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()
	assertMetrics(s)

	// Check metrics right after the data was read
	tfs := NewTagFilters()
	if err := tfs.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	_, err := s.SearchMetricNames(nil, []*TagFilters{tfs}, tr, 1e9, noDeadline)
	if err != nil {
		t.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
	}
	assertMetrics(s)
}
