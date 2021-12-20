package storage

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/quick"
	"time"

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
		date     uint64
		metricID uint64
	}
	m := make(map[dmk]bool)
	for i := 0; i < 1e5; i++ {
		date := uint64(i) % 3
		metricID := uint64(i) % 1237
		if !concurrent && c.Has(date, metricID) {
			if !m[dmk{date, metricID}] {
				return fmt.Errorf("c.Has(%d, %d) must return false, but returned true", date, metricID)
			}
			continue
		}
		c.Set(date, metricID)
		m[dmk{date, metricID}] = true
		if !concurrent && !c.Has(date, metricID) {
			return fmt.Errorf("c.Has(%d, %d) must return true, but returned false", date, metricID)
		}
		if i%11234 == 0 {
			c.mu.Lock()
			c.syncLocked()
			c.mu.Unlock()
		}
		if i%34323 == 0 {
			c.Reset()
			m = make(map[dmk]bool)
		}
	}

	// Verify fast path after sync.
	for i := 0; i < 1e5; i++ {
		date := uint64(i) % 3
		metricID := uint64(i) % 123
		c.Set(date, metricID)
	}
	c.mu.Lock()
	c.syncLocked()
	c.mu.Unlock()
	for i := 0; i < 1e5; i++ {
		date := uint64(i) % 3
		metricID := uint64(i) % 123
		if !concurrent && !c.Has(date, metricID) {
			return fmt.Errorf("c.Has(%d, %d) must return true after sync", date, metricID)
		}
	}

	// Verify c.Reset
	if n := c.EntriesCount(); !concurrent && n < 123 {
		return fmt.Errorf("c.EntriesCount must return at least 123; returned %d", n)
	}
	c.Reset()
	if n := c.EntriesCount(); !concurrent && n > 0 {
		return fmt.Errorf("c.EntriesCount must return 0 after reset; returned %d", n)
	}
	return nil
}

func TestUpdateCurrHourMetricIDs(t *testing.T) {
	newStorage := func() *Storage {
		var s Storage
		s.currHourMetricIDs.Store(&hourMetricIDs{})
		s.prevHourMetricIDs.Store(&hourMetricIDs{})
		return &s
	}
	t.Run("empty_pending_metric_ids_stale_curr_hour", func(t *testing.T) {
		s := newStorage()
		hour := uint64(timestampFromTime(time.Now())) / msecPerHour
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: 123,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs()
		hmCurr := s.currHourMetricIDs.Load().(*hourMetricIDs)
		if hmCurr.hour != hour {
			// It is possible new hour occurred. Update the hour and verify it again.
			hour = uint64(timestampFromTime(time.Now())) / msecPerHour
			if hmCurr.hour != hour {
				t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
			}
		}
		if hmCurr.m.Len() != 0 {
			t.Fatalf("unexpected length of hm.m; got %d; want %d", hmCurr.m.Len(), 0)
		}
		if !hmCurr.isFull {
			t.Fatalf("unexpected hmCurr.isFull; got %v; want %v", hmCurr.isFull, true)
		}

		hmPrev := s.prevHourMetricIDs.Load().(*hourMetricIDs)
		if !reflect.DeepEqual(hmPrev, hmOrig) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmOrig)
		}

		if len(s.pendingHourEntries) != 0 {
			t.Fatalf("unexpected len(s.pendingHourEntries); got %d; want %d", len(s.pendingHourEntries), 0)
		}
	})
	t.Run("empty_pending_metric_ids_valid_curr_hour", func(t *testing.T) {
		s := newStorage()
		hour := uint64(timestampFromTime(time.Now())) / msecPerHour
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: hour,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs()
		hmCurr := s.currHourMetricIDs.Load().(*hourMetricIDs)
		if hmCurr.hour != hour {
			// It is possible new hour occurred. Update the hour and verify it again.
			hour = uint64(timestampFromTime(time.Now())) / msecPerHour
			if hmCurr.hour != hour {
				t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
			}
			// Do not run other checks, since they may fail.
			return
		}
		if !reflect.DeepEqual(hmCurr, hmOrig) {
			t.Fatalf("unexpected hmCurr; got %v; want %v", hmCurr, hmOrig)
		}
		if hmCurr.isFull {
			t.Fatalf("unexpected hmCurr.isFull; got %v; want %v", hmCurr.isFull, false)
		}

		hmPrev := s.prevHourMetricIDs.Load().(*hourMetricIDs)
		hmEmpty := &hourMetricIDs{}
		if !reflect.DeepEqual(hmPrev, hmEmpty) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmEmpty)
		}
		if len(s.pendingHourEntries) != 0 {
			t.Fatalf("unexpected len(s.pendingHourEntries); got %d; want %d", len(s.pendingHourEntries), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_stale_curr_hour", func(t *testing.T) {
		s := newStorage()
		s.pendingHourEntries = []pendingHourMetricIDEntry{
			{AccountID: 123, ProjectID: 431, MetricID: 343},
			{AccountID: 123, ProjectID: 431, MetricID: 32424},
			{AccountID: 1, ProjectID: 2, MetricID: 8293432},
		}
		mExpected := &uint64set.Set{}
		for _, e := range s.pendingHourEntries {
			mExpected.Add(e.MetricID)
		}
		byTenantExpected := make(map[accountProjectKey]*uint64set.Set)
		for _, e := range s.pendingHourEntries {
			k := accountProjectKey{
				AccountID: e.AccountID,
				ProjectID: e.ProjectID,
			}
			x := byTenantExpected[k]
			if x == nil {
				x = &uint64set.Set{}
				byTenantExpected[k] = x
			}
			x.Add(e.MetricID)
		}

		hour := uint64(timestampFromTime(time.Now())) / msecPerHour
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: 123,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs()
		hmCurr := s.currHourMetricIDs.Load().(*hourMetricIDs)
		if hmCurr.hour != hour {
			// It is possible new hour occurred. Update the hour and verify it again.
			hour = uint64(timestampFromTime(time.Now())) / msecPerHour
			if hmCurr.hour != hour {
				t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
			}
		}
		if !hmCurr.m.Equal(mExpected) {
			t.Fatalf("unexpected hm.m; got %v; want %v", hmCurr.m, mExpected)
		}
		if !reflect.DeepEqual(hmCurr.byTenant, byTenantExpected) {
			t.Fatalf("unexpected hmPrev.byTenant; got %v; want %v", hmCurr.byTenant, byTenantExpected)
		}
		if !hmCurr.isFull {
			t.Fatalf("unexpected hmCurr.isFull; got %v; want %v", hmCurr.isFull, true)
		}

		hmPrev := s.prevHourMetricIDs.Load().(*hourMetricIDs)
		if !reflect.DeepEqual(hmPrev, hmOrig) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmOrig)
		}
		if len(s.pendingHourEntries) != 0 {
			t.Fatalf("unexpected len(s.pendingHourEntries); got %d; want %d", len(s.pendingHourEntries), 0)
		}
	})
	t.Run("nonempty_pending_metric_ids_valid_curr_hour", func(t *testing.T) {
		s := newStorage()
		s.pendingHourEntries = []pendingHourMetricIDEntry{
			{AccountID: 123, ProjectID: 431, MetricID: 343},
			{AccountID: 123, ProjectID: 431, MetricID: 32424},
			{AccountID: 1, ProjectID: 2, MetricID: 8293432},
		}
		mExpected := &uint64set.Set{}
		for _, e := range s.pendingHourEntries {
			mExpected.Add(e.MetricID)
		}
		byTenantExpected := make(map[accountProjectKey]*uint64set.Set)
		for _, e := range s.pendingHourEntries {
			k := accountProjectKey{
				AccountID: e.AccountID,
				ProjectID: e.ProjectID,
			}
			x := byTenantExpected[k]
			if x == nil {
				x = &uint64set.Set{}
				byTenantExpected[k] = x
			}
			x.Add(e.MetricID)
		}

		hour := uint64(timestampFromTime(time.Now())) / msecPerHour
		hmOrig := &hourMetricIDs{
			m:    &uint64set.Set{},
			hour: hour,
		}
		hmOrig.m.Add(12)
		hmOrig.m.Add(34)
		s.currHourMetricIDs.Store(hmOrig)
		s.updateCurrHourMetricIDs()
		hmCurr := s.currHourMetricIDs.Load().(*hourMetricIDs)
		if hmCurr.hour != hour {
			// It is possible new hour occurred. Update the hour and verify it again.
			hour = uint64(timestampFromTime(time.Now())) / msecPerHour
			if hmCurr.hour != hour {
				t.Fatalf("unexpected hmCurr.hour; got %d; want %d", hmCurr.hour, hour)
			}
			// Do not run other checks, since they may fail.
			return
		}
		m := mExpected.Clone()
		hmOrig.m.ForEach(func(part []uint64) bool {
			for _, metricID := range part {
				m.Add(metricID)
			}
			return true
		})
		if !hmCurr.m.Equal(m) {
			t.Fatalf("unexpected hm.m; got %v; want %v", hmCurr.m, m)
		}
		if !reflect.DeepEqual(hmCurr.byTenant, byTenantExpected) {
			t.Fatalf("unexpected hmPrev.byTenant; got %v; want %v", hmCurr.byTenant, byTenantExpected)
		}
		if hmCurr.isFull {
			t.Fatalf("unexpected hmCurr.isFull; got %v; want %v", hmCurr.isFull, false)
		}

		hmPrev := s.prevHourMetricIDs.Load().(*hourMetricIDs)
		hmEmpty := &hourMetricIDs{}
		if !reflect.DeepEqual(hmPrev, hmEmpty) {
			t.Fatalf("unexpected hmPrev; got %v; want %v", hmPrev, hmEmpty)
		}
		if len(s.pendingHourEntries) != 0 {
			t.Fatalf("unexpected s.pendingHourEntries.Len(); got %d; want %d", len(s.pendingHourEntries), 0)
		}
	})
}

func TestMetricRowMarshalUnmarshal(t *testing.T) {
	var buf []byte
	typ := reflect.TypeOf(&MetricRow{})
	rnd := rand.New(rand.NewSource(1))

	for i := 0; i < 1000; i++ {
		v, ok := quick.Value(typ, rnd)
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

func TestNextRetentionDuration(t *testing.T) {
	for retentionMonths := float64(0.1); retentionMonths < 120; retentionMonths += 0.3 {
		d := nextRetentionDuration(int64(retentionMonths * msecsPerMonth))
		if d <= 0 {
			currTime := time.Now().UTC()
			nextTime := time.Now().UTC().Add(d)
			t.Fatalf("unexpected retention duration for retentionMonths=%f; got %s; must be %s + %f months", retentionMonths, nextTime, currTime, retentionMonths)
		}
	}
}

func TestStorageOpenClose(t *testing.T) {
	path := "TestStorageOpenClose"
	for i := 0; i < 10; i++ {
		s, err := OpenStorage(path, -1, 1e5, 1e6)
		if err != nil {
			t.Fatalf("cannot open storage: %s", err)
		}
		s.MustClose()
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func TestStorageOpenMultipleTimes(t *testing.T) {
	path := "TestStorageOpenMultipleTimes"
	s1, err := OpenStorage(path, -1, 0, 0)
	if err != nil {
		t.Fatalf("cannot open storage the first time: %s", err)
	}

	for i := 0; i < 10; i++ {
		s2, err := OpenStorage(path, -1, 0, 0)
		if err == nil {
			s2.MustClose()
			t.Fatalf("expecting non-nil error when opening already opened storage")
		}
	}
	s1.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func TestStorageRandTimestamps(t *testing.T) {
	path := "TestStorageRandTimestamps"
	retentionMsecs := int64(60 * msecsPerMonth)
	s, err := OpenStorage(path, retentionMsecs, 0, 0)
	if err != nil {
		t.Fatalf("cannot open storage: %s", err)
	}
	t.Run("serial", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if err := testStorageRandTimestamps(s); err != nil {
				t.Fatal(err)
			}
			s.MustClose()
			s, err = OpenStorage(path, retentionMsecs, 0, 0)
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
		for i := 0; i < cap(ch); i++ {
			select {
			case err := <-ch:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second * 10):
				t.Fatal("timeout")
			}
		}
	})
	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func testStorageRandTimestamps(s *Storage) error {
	const rowsPerAdd = 1e3
	const addsCount = 2
	typ := reflect.TypeOf(int64(0))
	rnd := rand.New(rand.NewSource(1))

	for i := 0; i < addsCount; i++ {
		var mrs []MetricRow
		var mn MetricName
		mn.Tags = []Tag{
			{[]byte("job"), []byte("webservice")},
			{[]byte("instance"), []byte("1.2.3.4")},
		}
		for j := 0; j < rowsPerAdd; j++ {
			mn.MetricGroup = []byte(fmt.Sprintf("metric_%d", rand.Intn(100)))
			metricNameRaw := mn.marshalRaw(nil)
			timestamp := int64(rnd.NormFloat64() * 1e12)
			if j%2 == 0 {
				ts, ok := quick.Value(typ, rnd)
				if !ok {
					return fmt.Errorf("cannot create random timestamp via quick.Value")
				}
				timestamp = ts.Interface().(int64)
			}
			value := rnd.NormFloat64() * 1e12

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
	if m.TableMetrics.SmallRowsCount == 0 {
		return fmt.Errorf("expecting at least one row in the table")
	}
	return nil
}

func TestStorageDeleteMetrics(t *testing.T) {
	path := "TestStorageDeleteMetrics"
	s, err := OpenStorage(path, 0, 0, 0)
	if err != nil {
		t.Fatalf("cannot open storage: %s", err)
	}

	// Verify no tag keys exist
	tks, err := s.SearchTagKeys(0, 0, 1e5, noDeadline)
	if err != nil {
		t.Fatalf("error in SearchTagKeys at the start: %s", err)
	}
	if len(tks) != 0 {
		t.Fatalf("found non-empty tag keys at the start: %q", tks)
	}

	t.Run("serial", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if err = testStorageDeleteMetrics(s, 0); err != nil {
				t.Fatalf("unexpected error on iteration %d: %s", i, err)
			}

			// Re-open the storage in order to check how deleted metricIDs
			// are persisted.
			s.MustClose()
			s, err = OpenStorage(path, 0, 0, 0)
			if err != nil {
				t.Fatalf("cannot open storage after closing on iteration %d: %s", i, err)
			}
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		ch := make(chan error, 3)
		for i := 0; i < cap(ch); i++ {
			go func(workerNum int) {
				var err error
				for j := 0; j < 2; j++ {
					err = testStorageDeleteMetrics(s, workerNum)
					if err != nil {
						break
					}
				}
				ch <- err
			}(i)
		}
		for i := 0; i < cap(ch); i++ {
			select {
			case err := <-ch:
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			case <-time.After(30 * time.Second):
				t.Fatalf("timeout")
			}
		}
	})

	// Verify no more tag keys exist
	tks, err = s.SearchTagKeys(0, 0, 1e5, noDeadline)
	if err != nil {
		t.Fatalf("error in SearchTagKeys after the test: %s", err)
	}
	if len(tks) != 0 {
		t.Fatalf("found non-empty tag keys after the test: %q", tks)
	}

	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func testStorageDeleteMetrics(s *Storage, workerNum int) error {
	const rowsPerMetric = 100
	const metricsCount = 30

	workerTag := []byte(fmt.Sprintf("workerTag_%d", workerNum))
	accountID := uint32(workerNum)
	projectID := uint32(123)

	tksAll := make(map[string]bool)
	tksAll[""] = true // __name__
	for i := 0; i < metricsCount; i++ {
		var mrs []MetricRow
		var mn MetricName
		mn.AccountID = accountID
		mn.ProjectID = projectID
		job := fmt.Sprintf("job_%d_%d", i, workerNum)
		instance := fmt.Sprintf("instance_%d_%d", i, workerNum)
		mn.Tags = []Tag{
			{[]byte("job"), []byte(job)},
			{[]byte("instance"), []byte(instance)},
			{workerTag, []byte("foobar")},
		}
		for i := range mn.Tags {
			tksAll[string(mn.Tags[i].Key)] = true
		}
		mn.MetricGroup = []byte(fmt.Sprintf("metric_%d_%d", i, workerNum))
		metricNameRaw := mn.marshalRaw(nil)

		for j := 0; j < rowsPerMetric; j++ {
			timestamp := rand.Int63n(1e10)
			value := rand.NormFloat64() * 1e6

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
	tvs, err := s.SearchTagValues(accountID, projectID, workerTag, 1e5, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagValues before metrics removal: %w", err)
	}
	if len(tvs) == 0 {
		return fmt.Errorf("unexpected empty number of tag values for workerTag")
	}

	// Verify tag keys exist
	tks, err := s.SearchTagKeys(accountID, projectID, 1e5, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagKeys before metrics removal: %w", err)
	}
	if err := checkTagKeys(tks, tksAll); err != nil {
		return fmt.Errorf("unexpected tag keys before metrics removal: %w", err)
	}

	var sr Search
	tr := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: 2e10,
	}
	metricBlocksCount := func(tfs *TagFilters) int {
		// Verify the number of blocks
		n := 0
		sr.Init(s, []*TagFilters{tfs}, tr, 1e5, noDeadline)
		for sr.NextMetricBlock() {
			n++
		}
		sr.MustClose()
		return n
	}
	for i := 0; i < metricsCount; i++ {
		tfs := NewTagFilters(accountID, projectID)
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
		deletedCount, err := s.DeleteMetrics([]*TagFilters{tfs})
		if err != nil {
			return fmt.Errorf("cannot delete metrics: %w", err)
		}
		if deletedCount == 0 {
			return fmt.Errorf("expecting non-zero number of deleted metrics on iteration %d", i)
		}
		if n := metricBlocksCount(tfs); n != 0 {
			return fmt.Errorf("expecting zero metric blocks after DeleteMetrics call for tfs=%s; got %d blocks", tfs, n)
		}

		// Try deleting empty tfss
		deletedCount, err = s.DeleteMetrics(nil)
		if err != nil {
			return fmt.Errorf("cannot delete empty tfss: %w", err)
		}
		if deletedCount != 0 {
			return fmt.Errorf("expecting zero deleted metrics for empty tfss; got %d", deletedCount)
		}
	}

	// Make sure no more metrics left for the given workerNum
	tfs := NewTagFilters(accountID, projectID)
	if err := tfs.Add(nil, []byte(fmt.Sprintf("metric_.+_%d", workerNum)), false, true); err != nil {
		return fmt.Errorf("cannot add regexp tag filter for worker metrics: %w", err)
	}
	if n := metricBlocksCount(tfs); n != 0 {
		return fmt.Errorf("expecting zero metric blocks after deleting all the metrics; got %d blocks", n)
	}
	tvs, err = s.SearchTagValues(accountID, projectID, workerTag, 1e5, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagValues after all the metrics are removed: %w", err)
	}
	if len(tvs) != 0 {
		return fmt.Errorf("found non-empty tag values for %q after metrics removal: %q", workerTag, tvs)
	}

	return nil
}

func checkTagKeys(tks []string, tksExpected map[string]bool) error {
	if len(tks) < len(tksExpected) {
		return fmt.Errorf("unexpected number of tag keys found; got %d; want at least %d; tks=%q, tksExpected=%v", len(tks), len(tksExpected), tks, tksExpected)
	}
	hasItem := func(k string, tks []string) bool {
		for _, kk := range tks {
			if k == kk {
				return true
			}
		}
		return false
	}
	for k := range tksExpected {
		if !hasItem(k, tks) {
			return fmt.Errorf("cannot find %q in tag keys %q", k, tks)
		}
	}
	return nil
}

func TestStorageRegisterMetricNamesSerial(t *testing.T) {
	path := "TestStorageRegisterMetricNamesSerial"
	s, err := OpenStorage(path, 0, 0, 0)
	if err != nil {
		t.Fatalf("cannot open storage: %s", err)
	}
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
	s, err := OpenStorage(path, 0, 0, 0)
	if err != nil {
		t.Fatalf("cannot open storage: %s", err)
	}
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
	const accountID = 123
	const projectID = 421

	addIDsMap := make(map[string]struct{})
	for i := 0; i < addsCount; i++ {
		var mrs []MetricRow
		var mn MetricName
		addID := fmt.Sprintf("%d", i)
		addIDsMap[addID] = struct{}{}
		mn.AccountID = accountID
		mn.ProjectID = projectID
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
		if err := s.RegisterMetricNames(mrs); err != nil {
			return fmt.Errorf("unexpected error in AddMetrics: %w", err)
		}
	}
	var addIDsExpected []string
	for k := range addIDsMap {
		addIDsExpected = append(addIDsExpected, k)
	}
	sort.Strings(addIDsExpected)

	// Verify the storage contains the added metric names.
	s.DebugFlush()

	// Verify that SearchTagKeys returns correct result.
	tksExpected := []string{
		"",
		"add_id",
		"instance",
		"job",
	}
	tks, err := s.SearchTagKeys(accountID, projectID, 100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagKeys: %w", err)
	}
	sort.Strings(tks)
	if !reflect.DeepEqual(tks, tksExpected) {
		return fmt.Errorf("unexpected tag keys returned from SearchTagKeys;\ngot\n%q\nwant\n%q", tks, tksExpected)
	}

	// Verify that SearchTagKeys returns empty results for incorrect accountID, projectID
	tks, err = s.SearchTagKeys(accountID+1, projectID+1, 100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagKeys for incorrect accountID, projectID: %w", err)
	}
	if len(tks) > 0 {
		return fmt.Errorf("SearchTagKeys with incorrect accountID, projectID returns unexpected non-empty result:\n%q", tks)
	}

	// Verify that SearchTagKeysOnTimeRange returns correct result.
	now := timestampFromTime(time.Now())
	start := now - msecPerDay
	end := now + 60*1000
	tr := TimeRange{
		MinTimestamp: start,
		MaxTimestamp: end,
	}
	tks, err = s.SearchTagKeysOnTimeRange(accountID, projectID, tr, 100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagKeysOnTimeRange: %w", err)
	}
	sort.Strings(tks)
	if !reflect.DeepEqual(tks, tksExpected) {
		return fmt.Errorf("unexpected tag keys returned from SearchTagKeysOnTimeRange;\ngot\n%q\nwant\n%q", tks, tksExpected)
	}

	// Verify that SearchTagKeysOnTimeRange returns empty results for incrorrect accountID, projectID
	tks, err = s.SearchTagKeysOnTimeRange(accountID+1, projectID+1, tr, 100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagKeysOnTimeRange for incorrect accountID, projectID: %w", err)
	}
	if len(tks) > 0 {
		return fmt.Errorf("SearchTagKeysOnTimeRange with incorrect accountID, projectID returns unexpected non-empty result:\n%q", tks)
	}

	// Verify that SearchTagValues returns correct result.
	addIDs, err := s.SearchTagValues(accountID, projectID, []byte("add_id"), addsCount+100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagValues: %w", err)
	}
	sort.Strings(addIDs)
	if !reflect.DeepEqual(addIDs, addIDsExpected) {
		return fmt.Errorf("unexpected tag values returned from SearchTagValues;\ngot\n%q\nwant\n%q", addIDs, addIDsExpected)
	}

	// Verify that SearchTagValues return empty results for incorrect accountID, projectID
	addIDs, err = s.SearchTagValues(accountID+1, projectID+1, []byte("add_id"), addsCount+100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagValues for incorrect accountID, projectID: %w", err)
	}
	if len(addIDs) > 0 {
		return fmt.Errorf("SearchTagValues with incorrect accountID, projectID returns unexpected non-empty result:\n%q", addIDs)
	}

	// Verify that SearchTagValuesOnTimeRange returns correct result.
	addIDs, err = s.SearchTagValuesOnTimeRange(accountID, projectID, []byte("add_id"), tr, addsCount+100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagValuesOnTimeRange: %w", err)
	}
	sort.Strings(addIDs)
	if !reflect.DeepEqual(addIDs, addIDsExpected) {
		return fmt.Errorf("unexpected tag values returned from SearchTagValuesOnTimeRange;\ngot\n%q\nwant\n%q", addIDs, addIDsExpected)
	}

	// Verify that SearchTagValuesOnTimeRange returns empty results for incorrect accountID, projectID
	addIDs, err = s.SearchTagValuesOnTimeRange(accountID+1, projectID+1, []byte("addd_id"), tr, addsCount+100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchTagValuesOnTimeRange for incorrect accoundID, projectID: %w", err)
	}
	if len(addIDs) > 0 {
		return fmt.Errorf("SearchTagValuesOnTimeRange with incorrect accountID, projectID returns unexpected non-empty result:\n%q", addIDs)
	}

	// Verify that SearchMetricNames returns correct result.
	tfs := NewTagFilters(accountID, projectID)
	if err := tfs.Add([]byte("add_id"), []byte("0"), false, false); err != nil {
		return fmt.Errorf("unexpected error in TagFilters.Add: %w", err)
	}
	mns, err := s.SearchMetricNames([]*TagFilters{tfs}, tr, metricsPerAdd*addsCount*100+100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchMetricNames: %w", err)
	}
	if len(mns) < metricsPerAdd {
		return fmt.Errorf("unexpected number of metricNames returned from SearchMetricNames; got %d; want at least %d", len(mns), int(metricsPerAdd))
	}
	for i, mn := range mns {
		addID := mn.GetTagValue("add_id")
		if string(addID) != "0" {
			return fmt.Errorf("unexpected addID for metricName #%d; got %q; want %q", i, addID, "0")
		}
		job := mn.GetTagValue("job")
		if string(job) != "webservice" {
			return fmt.Errorf("unexpected job for metricName #%d; got %q; want %q", i, job, "webservice")
		}
	}

	// Verify that SearchMetricNames returns empty results for incorrect accountID, projectID
	tfs = NewTagFilters(accountID+1, projectID+1)
	if err := tfs.Add([]byte("add_id"), []byte("0"), false, false); err != nil {
		return fmt.Errorf("unexpected error in TagFilters.Add: %w", err)
	}
	mns, err = s.SearchMetricNames([]*TagFilters{tfs}, tr, metricsPerAdd*addsCount*100+100, noDeadline)
	if err != nil {
		return fmt.Errorf("error in SearchMetricNames for incorrect accountID, projectID: %w", err)
	}
	if len(mns) > 0 {
		return fmt.Errorf("SearchMetricNames with incorrect accountID, projectID returns unexpected non-empty result:\n%+v", mns)
	}

	return nil
}

func TestStorageAddRowsSerial(t *testing.T) {
	path := "TestStorageAddRowsSerial"
	s, err := OpenStorage(path, 0, 1e5, 1e5)
	if err != nil {
		t.Fatalf("cannot open storage: %s", err)
	}
	if err := testStorageAddRows(s); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	s.MustClose()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}

func TestStorageAddRowsConcurrent(t *testing.T) {
	path := "TestStorageAddRowsConcurrent"
	s, err := OpenStorage(path, 0, 1e5, 1e5)
	if err != nil {
		t.Fatalf("cannot open storage: %s", err)
	}
	ch := make(chan error, 3)
	for i := 0; i < cap(ch); i++ {
		go func() {
			ch <- testStorageAddRows(s)
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

func testStorageAddRows(s *Storage) error {
	const rowsPerAdd = 1e3
	const addsCount = 10

	for i := 0; i < addsCount; i++ {
		var mrs []MetricRow
		var mn MetricName
		mn.Tags = []Tag{
			{[]byte("job"), []byte("webservice")},
			{[]byte("instance"), []byte("1.2.3.4")},
		}
		for j := 0; j < rowsPerAdd; j++ {
			mn.AccountID = uint32(rand.Intn(2))
			mn.ProjectID = uint32(rand.Intn(3))
			mn.MetricGroup = []byte(fmt.Sprintf("metric_%d", rand.Intn(100)))
			metricNameRaw := mn.marshalRaw(nil)
			timestamp := rand.Int63n(1e10)
			value := rand.NormFloat64() * 1e6

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

	// Verify the storage contains rows.
	minRowsExpected := uint64(rowsPerAdd) * addsCount
	var m Metrics
	s.UpdateMetrics(&m)
	if m.TableMetrics.SmallRowsCount < minRowsExpected {
		return fmt.Errorf("expecting at least %d rows in the table; got %d", minRowsExpected, m.TableMetrics.SmallRowsCount)
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
	snapshotPath := s.path + "/snapshots/" + snapshotName
	s1, err := OpenStorage(snapshotPath, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("cannot open storage from snapshot: %w", err)
	}

	// Verify the snapshot contains rows
	var m1 Metrics
	s1.UpdateMetrics(&m1)
	if m1.TableMetrics.SmallRowsCount < minRowsExpected {
		return fmt.Errorf("snapshot %q must contain at least %d rows; got %d", snapshotPath, minRowsExpected, m1.TableMetrics.SmallRowsCount)
	}

	// Verify that force merge for the snapshot leaves only a single part per partition.
	if err := s1.ForceMergePartitions(""); err != nil {
		return fmt.Errorf("error when force merging partitions: %w", err)
	}
	ptws := s1.tb.GetPartitions(nil)
	for _, ptw := range ptws {
		pws := ptw.pt.GetParts(nil)
		numParts := len(pws)
		ptw.pt.PutParts(pws)
		if numParts != 1 {
			s1.tb.PutPartitions(ptws)
			return fmt.Errorf("unexpected number of parts for partition %q after force merge; got %d; want 1", ptw.pt.name, numParts)
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
	s, err := OpenStorage(path, 0, 0, 0)
	if err != nil {
		t.Fatalf("cannot open storage: %s", err)
	}

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
				s.mustRotateIndexDB()
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
	const rowsCount = 1e3

	var mn MetricName
	mn.Tags = []Tag{
		{[]byte("job"), []byte(fmt.Sprintf("webservice_%d", workerNum))},
		{[]byte("instance"), []byte("1.2.3.4")},
	}
	for i := 0; i < rowsCount; i++ {
		mn.AccountID = 123
		mn.ProjectID = uint32(i % 3)
		mn.MetricGroup = []byte(fmt.Sprintf("metric_%d_%d", workerNum, rand.Intn(10)))
		metricNameRaw := mn.marshalRaw(nil)
		timestamp := rand.Int63n(1e10)
		value := rand.NormFloat64() * 1e6

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
	if m.TableMetrics.SmallRowsCount < minRowsExpected {
		return fmt.Errorf("expecting at least %d rows in the table; got %d", minRowsExpected, m.TableMetrics.SmallRowsCount)
	}
	return nil
}

func containsString(a []string, s string) bool {
	for i := range a {
		if a[i] == s {
			return true
		}
	}
	return false
}
