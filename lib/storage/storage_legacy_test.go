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
	const (
		accountID = 12
		projectID = 34
	)
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("%s_metric_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte(name),
				AccountID:   accountID,
				ProjectID:   projectID,
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
		tfsAll := NewTagFilters(accountID, projectID)
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
	testSearchOpWithLegacyIndexDBs(t, accountID, projectID, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_SearchLabelNames(t *testing.T) {
	const (
		accountID = 12
		projectID = 34
	)
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("%s_label_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte("metric"),
				AccountID:   accountID,
				ProjectID:   projectID,
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
		got, err := s.SearchLabelNames(nil, accountID, projectID, nil, tr, 1e9, 1e9, noDeadline)
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
	testSearchOpWithLegacyIndexDBs(t, accountID, projectID, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_SearchLabelValues(t *testing.T) {
	const (
		accountID = 12
		projectID = 34
	)
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			value := fmt.Sprintf("%s_value_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte("metric"),
				AccountID:   accountID,
				ProjectID:   projectID,
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
		got, err := s.SearchLabelValues(nil, accountID, projectID, "label", nil, tr, 1e9, 1e9, noDeadline)
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
	testSearchOpWithLegacyIndexDBs(t, accountID, projectID, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_SearchTagValueSuffixes(t *testing.T) {
	const (
		accountID = 12
		projectID = 34
	)
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("%s_metric_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte("prefix." + name),
				AccountID:   accountID,
				ProjectID:   projectID,
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
		got, err := s.SearchTagValueSuffixes(nil, accountID, projectID, tr, "", "prefix.", '.', 1e9, noDeadline)
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
	testSearchOpWithLegacyIndexDBs(t, accountID, projectID, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_SearchGraphitePaths(t *testing.T) {
	const (
		accountID = 12
		projectID = 34
	)
	genData := func(numMetrics int, prefix string, tr TimeRange) ([]MetricRow, []string) {
		mrs := make([]MetricRow, numMetrics)
		want := make([]string, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("prefix.%s_metric_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte(name),
				AccountID:   accountID,
				ProjectID:   projectID,
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
		got, err := s.SearchGraphitePaths(nil, accountID, projectID, tr, []byte("*.*"), 1e9, noDeadline)
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
	testSearchOpWithLegacyIndexDBs(t, accountID, projectID, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_Search(t *testing.T) {
	const (
		accountID = 12
		projectID = 34
	)
	genData := func(numMetrics int, prefix string, tr TimeRange) []MetricRow {
		mrs := make([]MetricRow, numMetrics)
		for i := range numMetrics {
			name := fmt.Sprintf("%s_metric_%03d", prefix, i)
			mn := MetricName{
				MetricGroup: []byte(name),
				AccountID:   accountID,
				ProjectID:   projectID,
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
		tfsAll := NewTagFilters(accountID, projectID)
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
	testSearchOpWithLegacyIndexDBs(t, accountID, projectID, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_GetSeriesCount(t *testing.T) {
	const (
		accountID  = 12
		projectID  = 34
		numMetrics = 1000
	)
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	rng := rand.New(rand.NewSource(1))
	legacyData := testGenerateMetricRowsWithPrefixForTenantID(rng, accountID, projectID, numMetrics, "legacy", tr)
	newData := testGenerateMetricRowsWithPrefixForTenantID(rng, accountID, projectID, numMetrics, "new", tr)

	assertSearchResults := func(s *Storage, want uint64) {
		t.Helper()
		got, err := s.GetSeriesCount(accountID, projectID, noDeadline)
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
	testSearchOpWithLegacyIndexDBs(t, accountID, projectID, legacyData, newData, assertLegacyData, assertNewData)
}

func TestLegacyStorage_DeleteSeries(t *testing.T) {
	const (
		accountID  = 12
		projectID  = 34
		numMetrics = 1000
	)
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	rng := rand.New(rand.NewSource(1))
	legacyData := testGenerateMetricRowsWithPrefixForTenantID(rng, accountID, projectID, numMetrics, "legacy", tr)
	newData := testGenerateMetricRowsWithPrefixForTenantID(rng, accountID, projectID, numMetrics, "new", tr)
	tfsAll := NewTagFilters(accountID, projectID)
	if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		t.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}
	tfssAll := []*TagFilters{tfsAll}

	assertSeriesCount := func(s *Storage, want int) {
		t.Helper()
		got, err := s.SearchMetricNames(nil, tfssAll, tr, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
		}
		if len(got) != want {
			t.Fatalf("unexpected metric count: got %d, want %d", len(got), want)
		}
	}

	assertLegacyData := func(s *Storage) {
		t.Helper()
		want := len(legacyData)
		assertSeriesCount(s, want)
	}
	assertNewData := func(s *Storage) {
		t.Helper()
		want := len(legacyData) + len(newData)
		assertSeriesCount(s, want)

		got, err := s.DeleteSeries(nil, tfssAll, 1e9)
		if err != nil {
			t.Fatalf("DeleteSeries() failed unexpectedly: %v", err)
		}
		if got != want {
			t.Fatalf("Unexpected number of deleted series: got %d, want %d", got, want)
		}

		assertSeriesCount(s, 0)
	}
	testSearchOpWithLegacyIndexDBs(t, accountID, projectID, legacyData, newData, assertLegacyData, assertNewData)
}

// testSearchWithLegacyIndexDBs a search operation when the index data
// is located both partition and legacy indexDBs.
func testSearchOpWithLegacyIndexDBs(t *testing.T, accountID, projectID uint32, legacyData, newData []MetricRow, assertLegacyData, assertNewData func(s *Storage)) {
	defer testRemoveAll(t)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	s.AddRows(legacyData, defaultPrecisionBits)
	s.DebugFlush()
	assertLegacyData(s)
	s.MustClose()

	testStorageConvertToLegacy(t, accountID, projectID)
	s = MustOpenStorage(t.Name(), OpenOptions{})
	assertLegacyData(s)
	s.AddRows(newData, defaultPrecisionBits)
	s.DebugFlush()
	assertNewData(s)
	s.MustClose()
}

func TestLegacyStorageSnapshots_CreateListDelete(t *testing.T) {
	defer testRemoveAll(t)

	rng := rand.New(rand.NewSource(1))
	const (
		accountID = 12
		projectID = 34
		numRows   = 10000
	)
	minTimestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	maxTimestamp := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC).UnixMilli()
	mrs := testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp)

	root := t.Name()
	s := MustOpenStorage(root, OpenOptions{})
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()
	s.MustClose()
	testStorageConvertToLegacy(t, accountID, projectID)
	s = MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()

	var (
		data                 = filepath.Join(root, dataDirname)
		smallData            = filepath.Join(data, smallDirname)
		bigData              = filepath.Join(data, bigDirname)
		indexData            = filepath.Join(data, indexdbDirname)
		smallSnapshots       = filepath.Join(smallData, snapshotsDirname)
		bigSnapshots         = filepath.Join(bigData, snapshotsDirname)
		indexSnapshots       = filepath.Join(indexData, snapshotsDirname)
		legacyIndexData      = filepath.Join(root, indexdbDirname)
		legacyIndexSnapshots = filepath.Join(legacyIndexData, snapshotsDirname)
	)

	snapshot1Name := s.MustCreateSnapshot()
	assertListSnapshots := func(want []string) {
		got := s.MustListSnapshots()
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected snapshot list (-want, +got):\n%s", diff)
		}
	}
	assertListSnapshots([]string{snapshot1Name})

	var (
		snapshot1            = filepath.Join(root, snapshotsDirname, snapshot1Name)
		smallSnapshot1       = filepath.Join(smallSnapshots, snapshot1Name)
		smallSymlink1        = filepath.Join(snapshot1, dataDirname, smallDirname)
		bigSnapshot1         = filepath.Join(bigSnapshots, snapshot1Name)
		bigSymlink1          = filepath.Join(snapshot1, dataDirname, bigDirname)
		indexSnapshot1       = filepath.Join(indexSnapshots, snapshot1Name)
		indexSymlink1        = filepath.Join(snapshot1, dataDirname, indexdbDirname)
		legacyIndexSnapshot1 = filepath.Join(legacyIndexSnapshots, snapshot1Name)
		legacyIndexSymlink1  = filepath.Join(snapshot1, indexdbDirname)
	)

	// Check snapshot1 dir entries
	assertDirEntries := func(srcDir, snapshotDir string, excludePath ...string) {
		t.Helper()
		dataDirEntries := testListDirEntries(t, srcDir, excludePath...)
		snapshotDirEntries := testListDirEntries(t, snapshotDir)
		if diff := cmp.Diff(dataDirEntries, snapshotDirEntries); diff != "" {
			t.Fatalf("unexpected snapshot dir entries (-want, +got):\n%s", diff)
		}
	}
	assertDirEntries(smallData, smallSnapshot1, smallSnapshots)
	assertDirEntries(bigData, bigSnapshot1, bigSnapshots)
	assertDirEntries(indexData, indexSnapshot1, indexSnapshots)
	assertDirEntries(legacyIndexData, legacyIndexSnapshot1, legacyIndexSnapshots)

	// Check snapshot1 symlinks
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
	assertSymlink(bigSymlink1, bigSnapshot1)
	assertSymlink(smallSymlink1, smallSnapshot1)
	assertSymlink(indexSymlink1, indexSnapshot1)
	assertSymlink(legacyIndexSymlink1, legacyIndexSnapshot1)

	// Rotate indexdb. Only one legacy indexDB must remain.
	s.legacyMustRotateIndexDB(time.Now().UTC())

	// Create snapshot2
	snapshot2Name := s.MustCreateSnapshot()
	assertListSnapshots([]string{snapshot1Name, snapshot2Name})

	var (
		snapshot2            = filepath.Join(root, snapshotsDirname, snapshot2Name)
		smallSnapshot2       = filepath.Join(smallSnapshots, snapshot2Name)
		smallSymlink2        = filepath.Join(snapshot2, dataDirname, smallDirname)
		bigSnapshot2         = filepath.Join(bigSnapshots, snapshot2Name)
		bigSymlink2          = filepath.Join(snapshot2, dataDirname, bigDirname)
		indexSnapshot2       = filepath.Join(indexSnapshots, snapshot2Name)
		indexSymlink2        = filepath.Join(snapshot2, dataDirname, indexdbDirname)
		legacyIndexSnapshot2 = filepath.Join(legacyIndexSnapshots, snapshot2Name)
		legacyIndexSymlink2  = filepath.Join(snapshot2, indexdbDirname)
	)

	// Check snapshot2 dir entries
	assertDirEntries(smallData, smallSnapshot2, smallSnapshots)
	assertDirEntries(bigData, bigSnapshot2, bigSnapshots)
	assertDirEntries(indexData, indexSnapshot2, indexSnapshots)
	assertDirEntries(legacyIndexData, legacyIndexSnapshot2, legacyIndexSnapshots)

	// Check snapshot2 symlinks
	assertSymlink(bigSymlink2, bigSnapshot2)
	assertSymlink(smallSymlink2, smallSnapshot2)
	assertSymlink(indexSymlink2, indexSnapshot2)
	assertSymlink(legacyIndexSymlink2, legacyIndexSnapshot2)

	// Rotate indexdb once again. There shouldn't be any legacy indexDBs left.
	s.legacyMustRotateIndexDB(time.Now().UTC())

	// Create snapshot3
	snapshot3Name := s.MustCreateSnapshot()
	assertListSnapshots([]string{snapshot1Name, snapshot2Name, snapshot3Name})

	var (
		snapshot3            = filepath.Join(root, snapshotsDirname, snapshot3Name)
		smallSnapshot3       = filepath.Join(smallSnapshots, snapshot3Name)
		smallSymlink3        = filepath.Join(snapshot3, dataDirname, smallDirname)
		bigSnapshot3         = filepath.Join(bigSnapshots, snapshot3Name)
		bigSymlink3          = filepath.Join(snapshot3, dataDirname, bigDirname)
		indexSnapshot3       = filepath.Join(indexSnapshots, snapshot3Name)
		indexSymlink3        = filepath.Join(snapshot3, dataDirname, indexdbDirname)
		legacyIndexSnapshot3 = filepath.Join(legacyIndexSnapshots, snapshot3Name)
		legacyIndexSymlink3  = filepath.Join(snapshot3, indexdbDirname)
	)

	assertPathDoesNotExist := func(path string) {
		t.Helper()
		if vmfs.IsPathExist(path) {
			t.Fatalf("path was not expected to exist: %q", path)
		}
	}

	// Check snapshot3 dir entries
	assertDirEntries(smallData, smallSnapshot3, smallSnapshots)
	assertDirEntries(bigData, bigSnapshot3, bigSnapshots)
	assertDirEntries(indexData, indexSnapshot3, indexSnapshots)
	assertPathDoesNotExist(legacyIndexSnapshot3)

	// Check snapshot3 symlinks
	assertSymlink(bigSymlink3, bigSnapshot3)
	assertSymlink(smallSymlink3, smallSnapshot3)
	assertSymlink(indexSymlink3, indexSnapshot3)
	assertPathDoesNotExist(legacyIndexSymlink3)

	// Check snapshot deletion.
	for _, name := range []string{snapshot1Name, snapshot2Name, snapshot3Name} {
		if err := s.DeleteSnapshot(name); err != nil {
			t.Fatalf("could not delete snapshot %q: %v", name, err)
		}
	}
	assertListSnapshots([]string{})
	assertPathDoesNotExist(snapshot1)
	assertPathDoesNotExist(snapshot2)
	assertPathDoesNotExist(snapshot3)
	assertPathDoesNotExist(bigSnapshot1)
	assertPathDoesNotExist(bigSnapshot2)
	assertPathDoesNotExist(bigSnapshot3)
	assertPathDoesNotExist(smallSnapshot1)
	assertPathDoesNotExist(smallSnapshot2)
	assertPathDoesNotExist(smallSnapshot3)
	assertPathDoesNotExist(indexSnapshot1)
	assertPathDoesNotExist(indexSnapshot2)
	assertPathDoesNotExist(indexSnapshot3)
	assertPathDoesNotExist(legacyIndexSnapshot1)
	assertPathDoesNotExist(legacyIndexSnapshot2)
}

// testStorageConvertToLegacy converts the storage partition indexDBs into the
// legacy indexDB. The original partition indexDBs are removed.
//
// The index is copied to curr indexDB. The function also creates an empty prev
// indexDB directory.
//
// The storageDataPath is expected to be t.Name().
func testStorageConvertToLegacy(t *testing.T, accountID, projectID uint32) {
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

	// Read index items from the partition indexDBs and write them to the legacy
	// curr indexDB.

	idbs := s.tb.GetIndexDBs(TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: math.MaxInt64,
	})
	tfsAll := NewTagFilters(accountID, projectID)
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
			tsids, err := idb.getTSIDsFromMetricIDs(nil, accountID, projectID, metricIDs, noDeadline)
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
				mnBytes, ok := idb.searchMetricName(nil, metricID, accountID, projectID, false)
				if !ok {
					t.Fatalf("could not get metric name for metricID %d", metricID)
				}
				var mn MetricName
				if err := mn.Unmarshal(mnBytes); err != nil {
					t.Fatalf("Could not unmarshal metric name from bytes %q: %v", string(mnBytes), err)
				}
				if !seenGlobalIndexEntries[metricID] {
					legacyIDBCurr.createGlobalIndexes(&tsid, &mn)
					seenGlobalIndexEntries[metricID] = true
				}
				dateMetricID := dateMetricID{
					date:     date,
					metricID: metricID,
				}
				if !seenPerDayIndexEntries[dateMetricID] {
					legacyIDBCurr.createPerDayIndexes(date, &tsid, &mn)
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
