package storage

import (
	"fmt"
	"io/fs"
	"math"
	"math/rand"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	vmfs "github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

func TestMustOpenLegacyIndexDBTables_noTables(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBIsNil(t, prev)
	assertIndexDBIsNil(t, curr)
}

func TestMustOpenLegacyIndexDBTables_prevOnly(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)

	assertPathsExist(t, prevPath)

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBName(t, prev, prevName)
	assertIndexDBIsNil(t, curr)
}

func TestMustOpenLegacyIndexDBTables_currAndPrev(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)
	currName := "123456789ABCDEF1"
	currPath := filepath.Join(legacyIDBPath, currName)
	vmfs.MustMkdirIfNotExist(currPath)

	assertPathsExist(t, prevPath, currPath)

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBName(t, prev, prevName)
	assertIndexDBName(t, curr, currName)
}

func TestMustOpenLegacyIndexDBTables_nextIsRemoved(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)
	currName := "123456789ABCDEF1"
	currPath := filepath.Join(legacyIDBPath, currName)
	vmfs.MustMkdirIfNotExist(currPath)
	nextName := "123456789ABCDEF2"
	nextPath := filepath.Join(legacyIDBPath, nextName)
	vmfs.MustMkdirIfNotExist(nextPath)

	assertPathsExist(t, prevPath, currPath, nextPath)

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBName(t, prev, prevName)
	assertIndexDBName(t, curr, currName)
	assertPathsDoNotExist(t, nextPath)
}

func TestMustOpenLegacyIndexDBTables_nextAndAbsoleteDirsAreRemoved(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()
	absolete1Name := "123456789ABCDEEE"
	absolete1Path := filepath.Join(legacyIDBPath, absolete1Name)
	vmfs.MustMkdirIfNotExist(absolete1Path)
	absolete2Name := "123456789ABCDEEF"
	absolete2Path := filepath.Join(legacyIDBPath, absolete2Name)
	vmfs.MustMkdirIfNotExist(absolete2Path)
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)
	currName := "123456789ABCDEF1"
	currPath := filepath.Join(legacyIDBPath, currName)
	vmfs.MustMkdirIfNotExist(currPath)
	nextName := "123456789ABCDEF2"
	nextPath := filepath.Join(legacyIDBPath, nextName)
	vmfs.MustMkdirIfNotExist(nextPath)

	assertPathsExist(t, absolete1Path, absolete2Path, prevPath, currPath, nextPath)

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBName(t, prev, prevName)
	assertIndexDBName(t, curr, currName)
	assertPathsDoNotExist(t, absolete1Path, absolete2Path, nextPath)
}

func TestLegacyMustRotateIndexDBs(t *testing.T) {
	defer testRemoveAll(t)

	storagePath := t.Name()
	legacyIDBPath := filepath.Join(storagePath, indexdbDirname)
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)
	currName := "123456789ABCDEF1"
	currPath := filepath.Join(legacyIDBPath, currName)
	vmfs.MustMkdirIfNotExist(currPath)

	assertPathsExist(t, prevPath, currPath)

	s := MustOpenStorage(storagePath, OpenOptions{})
	defer s.MustClose()

	var prev, curr *indexDB

	if !s.hasLegacyIndexDBs() {
		t.Fatalf("storage was expected to have legacy indexDBs but it doesn't")
	}
	prev, curr = s.getLegacyIndexDBs()
	assertIndexDBName(t, prev, prevName)
	assertIndexDBName(t, curr, currName)
	assertDirEntries(t, legacyIDBPath, 2, []string{prevName, currName})
	s.putLegacyIndexDBs(prev, curr)

	s.legacyMustRotateIndexDB(time.Now())

	if !s.hasLegacyIndexDBs() {
		t.Fatalf("storage was expected to have legacy indexDBs but it doesn't")
	}
	prev, curr = s.getLegacyIndexDBs()
	assertIndexDBName(t, prev, currName)
	assertIndexDBIsNil(t, curr)
	assertPathsDoNotExist(t, prevPath)
	assertPathsExist(t, currPath)
	assertDirEntries(t, legacyIDBPath, 2, []string{currName})
	s.putLegacyIndexDBs(prev, curr)

	s.legacyMustRotateIndexDB(time.Now())

	if s.hasLegacyIndexDBs() {
		t.Fatalf("storage was expected to have no legacy indexDBs but it has them")
	}
	prev, curr = s.getLegacyIndexDBs()
	assertIndexDBIsNil(t, prev)
	assertIndexDBIsNil(t, curr)
	assertPathsDoNotExist(t, prevPath, currPath)
	assertDirEntries(t, legacyIDBPath, 2, []string{})
	s.putLegacyIndexDBs(prev, curr)
}

func assertPathsExist(t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		if !vmfs.IsPathExist(path) {
			t.Fatalf("path does not exist: %s", path)
		}
	}
}

func assertPathsDoNotExist(t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		if vmfs.IsPathExist(path) {
			t.Fatalf("path exists: %s", path)
		}
	}
}

func assertDirEntries(t *testing.T, dir string, depth int, want []string) {
	t.Helper()

	got := []string{}

	f := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Only include entries at the given depth level.
		if strings.Count(path, "/") != depth {
			return nil
		}
		got = append(got, entry.Name())
		return nil
	}
	if err := filepath.WalkDir(dir, f); err != nil {
		t.Fatalf("could not walk dir %q: %v", dir, err)
	}

	slices.Sort(got)
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected dir entries (-want, +got):\n%s", diff)
	}
}

func assertIndexDBName(t *testing.T, idb *indexDB, want string) {
	t.Helper()

	if idb == nil {
		t.Fatalf("unexpected idb: got nil, want non-nil")
	}
	if got := idb.name; got != want {
		t.Errorf("unexpected idb name: got %s, want %s", got, want)
	}
}

func assertIndexDBIsNil(t *testing.T, idb *indexDB) {
	t.Helper()

	if idb != nil {
		t.Fatalf("unexpected idb: got %s, want nil", idb.name)
	}
}

func TestLegacyStorage_SearchMetricNames(t *testing.T) {
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("%s_metric_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte(name),
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp
			mrs[i].Value = float64(i)
			want[i] = name
		}
		return mrs, want
	}
	const numMetrics = 1000
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	legacyData, wantLegacy := genData(numMetrics, "legacy", tr)
	newData, wantNew := genData(numMetrics, "new", tr)
	wantNew = append(wantNew, wantLegacy...)
	slices.Sort(wantNew)

	assertSearchResults := func(s *Storage, want []string) {
		t.Helper()
		tfsAll := NewTagFilters()
		if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		tfssAll := []*TagFilters{tfsAll}
		got, err := s.SearchMetricNames(nil, tfssAll, tr, 1e9, noDeadline)
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

	assertLegacyData := func(s *Storage) {
		assertSearchResults(s, wantLegacy)
	}
	assertNewData := func(s *Storage) {
		assertSearchResults(s, wantNew)
	}
	testSearchOpWithLegacyIndexDBs(t, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_SearchLabelNames(t *testing.T) {
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("%s_label_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte("metric"),
				Tags: []Tag{
					{[]byte(name), []byte("value")},
				},
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp
			mrs[i].Value = float64(i)
			want[i] = name
		}
		return mrs, want
	}
	const numMetrics = 1000
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	legacyData, wantLegacy := genData(numMetrics, "legacy", tr)
	newData, wantNew := genData(numMetrics, "new", tr)
	wantNew = append(wantNew, wantLegacy...)

	assertSearchResults := func(s *Storage, want []string) {
		t.Helper()
		got, err := s.SearchLabelNames(nil, nil, tr, 1e9, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchLabelNames() failed unexpectedly: %v", err)
		}
		slices.Sort(got)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected label names (-want, +got):\n%s", diff)
		}
	}

	assertLegacyData := func(s *Storage) {
		want := append(wantLegacy, "__name__")
		slices.Sort(want)
		assertSearchResults(s, want)
	}
	assertNewData := func(s *Storage) {
		want := append(wantNew, "__name__")
		slices.Sort(want)
		assertSearchResults(s, want)
	}
	testSearchOpWithLegacyIndexDBs(t, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_SearchLabelValues(t *testing.T) {
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			value := fmt.Sprintf("%s_value_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte("metric"),
				Tags: []Tag{
					{[]byte("label"), []byte(value)},
				},
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp
			mrs[i].Value = float64(i)
			want[i] = value
		}
		return mrs, want
	}
	const numMetrics = 1000
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	legacyData, wantLegacy := genData(numMetrics, "legacy", tr)
	newData, wantNew := genData(numMetrics, "new", tr)
	wantNew = append(wantNew, wantLegacy...)
	slices.Sort(wantNew)

	assertSearchResults := func(s *Storage, want []string) {
		t.Helper()
		got, err := s.SearchLabelValues(nil, "label", nil, tr, 1e9, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchLabelValues() failed unexpectedly: %v", err)
		}
		slices.Sort(got)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected label values (-want, +got):\n%s", diff)
		}
	}

	assertLegacyData := func(s *Storage) {
		t.Helper()
		assertSearchResults(s, wantLegacy)
	}
	assertNewData := func(s *Storage) {
		t.Helper()
		assertSearchResults(s, wantNew)
	}
	testSearchOpWithLegacyIndexDBs(t, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_SearchTagValueSuffixes(t *testing.T) {
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("%s_metric_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte("prefix." + name),
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp
			mrs[i].Value = float64(i)
			want[i] = name
		}
		return mrs, want
	}
	const numMetrics = 1000
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	legacyData, wantLegacy := genData(numMetrics, "legacy", tr)
	newData, wantNew := genData(numMetrics, "new", tr)
	wantNew = append(wantNew, wantLegacy...)
	slices.Sort(wantNew)

	assertSearchResults := func(s *Storage, want []string) {
		t.Helper()
		got, err := s.SearchTagValueSuffixes(nil, tr, "", "prefix.", '.', 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchTagValueSuffixes() failed unexpectedly: %v", err)
		}
		slices.Sort(got)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected tag value suffixes (-want, +got):\n%s", diff)
		}
	}

	assertLegacyData := func(s *Storage) {
		t.Helper()
		assertSearchResults(s, wantLegacy)
	}
	assertNewData := func(s *Storage) {
		t.Helper()
		assertSearchResults(s, wantNew)
	}
	testSearchOpWithLegacyIndexDBs(t, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_SearchGraphitePaths(t *testing.T) {
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("prefix.%s_metric_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte(name),
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp
			mrs[i].Value = float64(i)
			want[i] = name
		}
		return mrs, want
	}
	const numMetrics = 1000
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	legacyData, wantLegacy := genData(numMetrics, "legacy", tr)
	newData, wantNew := genData(numMetrics, "new", tr)
	wantNew = append(wantNew, wantLegacy...)
	slices.Sort(wantNew)

	assertSearchResults := func(s *Storage, want []string) {
		t.Helper()
		got, err := s.SearchGraphitePaths(nil, tr, []byte("*.*"), 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchTagGraphitePaths() failed unexpectedly: %v", err)
		}
		slices.Sort(got)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected graphite paths (-want, +got):\n%s", diff)
		}
	}

	assertLegacyData := func(s *Storage) {
		t.Helper()
		assertSearchResults(s, wantLegacy)
	}
	assertNewData := func(s *Storage) {
		t.Helper()
		assertSearchResults(s, wantNew)
	}
	testSearchOpWithLegacyIndexDBs(t, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_Search(t *testing.T) {
	genData := func(numMetrics int, prefix string, tr TimeRange) []MetricRow {
		mrs := make([]MetricRow, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("%s_metric_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte(name),
			}
			mrs[i].MetricNameRaw = mn.marshalRaw(nil)
			mrs[i].Timestamp = rand.Int63n(tr.MaxTimestamp-tr.MinTimestamp) + tr.MinTimestamp
			mrs[i].Value = float64(i)
		}
		return mrs
	}
	const numMetrics = 1000
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	legacyData := genData(numMetrics, "legacy", tr)
	newData := genData(numMetrics, "new", tr)

	assertSearchResults := func(s *Storage, want []MetricRow) {
		tfsAll := NewTagFilters()
		if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		if err := testAssertSearchResult(s, tr, tfsAll, want); err != nil {
			t.Fatalf("unexpected search results: %v", err)
		}
	}

	assertLegacyData := func(s *Storage) {
		t.Helper()
		want := legacyData
		assertSearchResults(s, want)
	}
	assertNewData := func(s *Storage) {
		t.Helper()
		want := slices.Concat(legacyData, newData)
		assertSearchResults(s, want)
	}
	testSearchOpWithLegacyIndexDBs(t, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_GetSeriesCount(t *testing.T) {
	const numMetrics = 1000
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	rng := rand.New(rand.NewSource(1))
	legacyData := testGenerateMetricRowsWithPrefix(rng, numMetrics, "legacy", tr)
	newData := testGenerateMetricRowsWithPrefix(rng, numMetrics, "new", tr)

	assertSearchResults := func(s *Storage, want uint64) {
		got, err := s.GetSeriesCount(noDeadline)
		if err != nil {
			t.Fatalf("GetSeriesCount() failed unexpectedly: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected metric count: got %d, want %d", got, want)
		}
	}

	assertLegacyData := func(s *Storage) {
		t.Helper()
		want := uint64(len(legacyData))
		assertSearchResults(s, want)
	}
	assertNewData := func(s *Storage) {
		t.Helper()
		want := uint64(len(legacyData) + len(newData))
		assertSearchResults(s, want)
	}
	testSearchOpWithLegacyIndexDBs(t, legacyData, newData, assertLegacyData, assertNewData)
}

// testSearchWithLegacyIndexDBs a search operation when the index data
// is located both partition and legacy indexDBs.
func testSearchOpWithLegacyIndexDBs(t *testing.T, legacyData, newData []MetricRow, assertLegacyData, assertNewData func(s *Storage)) {
	defer testRemoveAll(t)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	s.AddRows(legacyData, defaultPrecisionBits)
	s.DebugFlush()
	assertLegacyData(s)
	s.MustClose()

	testStorageConvertToLegacy(t)
	s = MustOpenStorage(t.Name(), OpenOptions{})
	assertLegacyData(s)
	s.AddRows(newData, defaultPrecisionBits)
	s.DebugFlush()
	assertNewData(s)
	s.MustClose()
}

// testStorageConvertToLegacy converts the storage partition indexDBs into the
// legacy indexDB. The original partition indexDBs are removed.
//
// The index is copied to curr indexDB. The function also creates an empty prev
// indexDB directory.
//
// The storageDataPath is expected to be t.Name().
func testStorageConvertToLegacy(t *testing.T) {
	t.Helper()

	storageDataPath := t.Name()
	legacyIDBPath := filepath.Join(storageDataPath, indexdbDirname)
	if vmfs.IsPathExist(legacyIDBPath) {
		t.Fatalf("legacy indexDB already exists: %q", legacyIDBPath)
	}

	s := MustOpenStorage(t.Name(), OpenOptions{})

	// Create legacy prev and curr indexDBs and open legacy curr indexDB.

	legacyIDBPrevName := "0000000000000001"
	legacyIDBCurrName := "0000000000000002"
	legacyIDBPrevPath := filepath.Join(storageDataPath, indexdbDirname, legacyIDBPrevName)
	legacyIDBCurrPath := filepath.Join(storageDataPath, indexdbDirname, legacyIDBCurrName)
	vmfs.MustMkdirFailIfExist(legacyIDBPrevPath)
	vmfs.MustMkdirFailIfExist(legacyIDBCurrPath)
	legacyIDBTimeRange := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: math.MaxInt64,
	}
	var isReadOnly atomic.Bool
	isReadOnly.Store(false)
	legacyIDBCurr := mustOpenIndexDB(2, legacyIDBTimeRange, legacyIDBCurrName, legacyIDBCurrPath, s, &isReadOnly)
	legacyISCurr := legacyIDBCurr.getIndexSearch(noDeadline)
	legacyIDBCurr.putIndexSearch(legacyISCurr)

	// Read index items from the partition indexDBs and write them to the legacy
	// curr indexDB.

	idbs := s.tb.GetIndexDBs(TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: math.MaxInt64,
	})
	tfsAll := NewTagFilters()
	if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	tfssAll := []*TagFilters{tfsAll}
	seenGlobalIndexEntries := make(map[uint64]bool)
	type dateMetricID struct {
		date     uint64
		metricID uint64
	}
	seenPerDayIndexEntries := make(map[dateMetricID]bool)
	for _, idb := range idbs {
		for ts := idb.tr.MinTimestamp; ts < idb.tr.MaxTimestamp; ts += msecPerDay {
			day := TimeRange{
				MinTimestamp: ts,
				MaxTimestamp: ts + msecPerDay - 1,
			}
			date := uint64(ts / msecPerDay)
			metricIDs, err := idb.searchMetricIDs(nil, tfssAll, day, 1e9, noDeadline)
			if err != nil {
				t.Fatalf("could not search metricIDs: %v", err)
			}
			tsids, err := idb.getTSIDsFromMetricIDs(nil, metricIDs, noDeadline)
			if err != nil {
				t.Fatalf("could not get TSIDs from metricIDs: %v", err)
			}
			for i, metricID := range metricIDs {
				if tsids[i].MetricID != metricID {
					t.Fatalf("metricID and TSID slices do not match")
				}
			}
			for _, tsid := range tsids {
				metricID := tsid.MetricID
				mnBytes, ok := idb.searchMetricName(nil, metricID, false)
				if !ok {
					t.Fatalf("could not get metric name for metricID %d", metricID)
				}
				var mn MetricName
				if err := mn.Unmarshal(mnBytes); err != nil {
					t.Fatalf("Could not unmarshal metric name from bytes %q: %v", string(mnBytes), err)
				}
				if !seenGlobalIndexEntries[metricID] {
					legacyISCurr.createGlobalIndexes(&tsid, &mn)
					seenGlobalIndexEntries[metricID] = true
				}
				dateMetricID := dateMetricID{
					date:     date,
					metricID: metricID,
				}
				if !seenPerDayIndexEntries[dateMetricID] {
					legacyISCurr.createPerDayIndexes(date, &tsid, &mn)
					seenPerDayIndexEntries[dateMetricID] = true
				}
			}
		}
	}

	s.tb.PutIndexDBs(idbs)
	legacyIDBCurr.MustClose()
	s.MustClose()

	// Remove partition indexDBs.
	vmfs.MustRemoveAll(filepath.Join(storageDataPath, dataDirname, indexdbDirname))
}
