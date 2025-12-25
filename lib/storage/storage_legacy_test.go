package storage

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

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
		t.Helper()
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

func TestLegacyStorage_DeleteSeries(t *testing.T) {
	const numMetrics = 1000
	tr := TimeRange{
		MinTimestamp: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 5, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	rng := rand.New(rand.NewSource(1))
	legacyData := testGenerateMetricRowsWithPrefix(rng, numMetrics, "legacy", tr)
	newData := testGenerateMetricRowsWithPrefix(rng, numMetrics, "new", tr)
	tfsAll := NewTagFilters()
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

	s = mustConvertToLegacy(s)
	assertLegacyData(s)

	s.AddRows(newData, defaultPrecisionBits)
	s.DebugFlush()
	assertNewData(s)
	s.MustClose()
}

func TestLegacyStorageSnapshots_CreateListDelete(t *testing.T) {
	defer testRemoveAll(t)

	rng := rand.New(rand.NewSource(1))
	const numRows = 10000
	minTimestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	maxTimestamp := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC).UnixMilli()
	mrs := testGenerateMetricRows(rng, numRows, minTimestamp, maxTimestamp)

	root := t.Name()
	s := MustOpenStorage(root, OpenOptions{})
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()
	// Convert to legacy 2 times in order to have both prev and curr legacy idbs.
	s = mustConvertToLegacy(s)
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
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
		if fs.IsPathExist(path) {
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

func TestStorageConvertToLegacy(t *testing.T) {
	defer testRemoveAll(t)

	assertMetricNames := func(s *Storage, tr TimeRange, wantMRs []MetricRow) {
		t.Helper()
		tfs := NewTagFilters()
		if err := tfs.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		got, err := s.SearchMetricNames(nil, []*TagFilters{tfs}, tr, 1e9, noDeadline)
		if err != nil {
			t.Fatalf("SearchMetricNames() failed unexpectedly: %v", err)
		}
		var mn MetricName
		for i, name := range got {
			if err := mn.UnmarshalString(name); err != nil {
				t.Fatalf("could not unmarshal metric name %q: %v", name, err)
			}
			got[i] = string(mn.MetricGroup)
		}
		slices.Sort(got)
		want := make([]string, len(wantMRs))
		for i, mr := range wantMRs {
			if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
				t.Fatalf("could not unmarshal raw metric name %v: %v", mr.MetricNameRaw, err)
			}
			want[i] = string(mn.MetricGroup)
		}
		slices.Sort(want)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected metric names (-want, +got):\n%s", diff)
		}
	}

	rng := rand.New(rand.NewSource(1))
	const numSeries = 10
	tr1 := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 1, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	tr2 := TimeRange{
		MinTimestamp: tr1.MinTimestamp + msecPerDay,
		MaxTimestamp: tr1.MaxTimestamp + msecPerDay,
	}
	tr3 := TimeRange{
		MinTimestamp: tr2.MinTimestamp + msecPerDay,
		MaxTimestamp: tr2.MaxTimestamp + msecPerDay,
	}
	tr4 := TimeRange{
		MinTimestamp: tr3.MinTimestamp + msecPerDay,
		MaxTimestamp: tr3.MaxTimestamp + msecPerDay,
	}
	trAll := TimeRange{
		MinTimestamp: tr1.MinTimestamp,
		MaxTimestamp: tr4.MaxTimestamp,
	}
	mrs1 := testGenerateMetricRowsWithPrefix(rng, numSeries, "generation1", tr1)
	mrs2 := testGenerateMetricRowsWithPrefix(rng, numSeries, "generation2", tr2)
	mrs3 := testGenerateMetricRowsWithPrefix(rng, numSeries, "generation3", tr3)
	mrs4 := testGenerateMetricRowsWithPrefix(rng, numSeries, "generation4", tr4)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	s.AddRows(mrs1, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
	assertMetricNames(s, trAll, mrs1)

	s.AddRows(mrs2, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
	assertMetricNames(s, trAll, slices.Concat(mrs1, mrs2))

	s.AddRows(mrs3, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
	assertMetricNames(s, trAll, slices.Concat(mrs2, mrs3))

	s.AddRows(mrs4, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
	assertMetricNames(s, trAll, slices.Concat(mrs3, mrs4))

	s.MustClose()
}

// mustConvertToLegacy converts the storage partition indexDBs into a
// legacy indexDB. The original partition indexDBs are removed.
//
// Each invocation of this function will a new legacy indexDB in
// storageDataPath/indexdb dir. The function will keep only 2 most recent
// indexDBs under that path.
//
// The function also deteles all persistent caches.
func mustConvertToLegacy(s *Storage) *Storage {
	// Stop storage, move legacy idbs to tmp dir, delete all caches,
	// re-open storage with pt index only.
	storageDataPath := s.path
	s.MustClose()
	legacyIDBsPathOrig := filepath.Join(s.path, indexdbDirname)
	fs.MustMkdirIfNotExist(legacyIDBsPathOrig)
	legacyIDBsPathTmp := filepath.Join(s.path, "indexdb-legacy")
	if err := os.Rename(legacyIDBsPathOrig, legacyIDBsPathTmp); err != nil {
		panic(fmt.Sprintf("could not rename %q to %q: %v", legacyIDBsPathOrig, legacyIDBsPathTmp, err))
	}
	fs.MustRemoveDir(filepath.Join(storageDataPath, cacheDirname))
	s = MustOpenStorage(storageDataPath, OpenOptions{})

	legacyIDBID := uint64(time.Now().UnixNano())
	legacyIDBName := fmt.Sprintf("%016X", legacyIDBID)
	legacyIDBPath := filepath.Join(legacyIDBsPathTmp, legacyIDBName)
	fs.MustMkdirFailIfExist(legacyIDBPath)
	legacyIDBPartsFile := filepath.Join(legacyIDBPath, partsFilename)
	fs.MustWriteAtomic(legacyIDBPartsFile, []byte("[]"), true)
	legacyIDBTimeRange := TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: math.MaxInt64,
	}
	var isReadOnly atomic.Bool
	isReadOnly.Store(false)
	legacyIDB := mustOpenIndexDB(legacyIDBID, legacyIDBTimeRange, legacyIDBName, legacyIDBPath, s, &isReadOnly, false)

	// Read index items from the partition indexDBs and write them to the legacy
	// indexDB.

	ptws := s.tb.GetPartitions(legacyIDBTimeRange)
	tfsAll := NewTagFilters()
	if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		panic(fmt.Sprintf("unexpected error in TagFilters.Add: %v", err))
	}
	tfssAll := []*TagFilters{tfsAll}
	seenGlobalIndexEntries := make(map[uint64]bool)
	type dateMetricID struct {
		date     uint64
		metricID uint64
	}
	seenPerDayIndexEntries := make(map[dateMetricID]bool)
	for _, ptw := range ptws {
		idb := ptw.pt.idb
		for ts := idb.tr.MinTimestamp; ts < idb.tr.MaxTimestamp; ts += msecPerDay {
			day := TimeRange{
				MinTimestamp: ts,
				MaxTimestamp: ts + msecPerDay - 1,
			}
			date := uint64(ts / msecPerDay)
			tsids, err := idb.SearchTSIDs(nil, tfssAll, day, 1e9, noDeadline)
			if err != nil {
				panic(fmt.Sprintf("could not get TSIDs: %v", err))
			}
			for _, tsid := range tsids {
				metricID := tsid.MetricID
				mnBytes, ok := idb.searchMetricName(nil, metricID, false)
				if !ok {
					panic(fmt.Sprintf("could not get metric name for metricID %d", metricID))
				}
				var mn MetricName
				if err := mn.Unmarshal(mnBytes); err != nil {
					panic(fmt.Sprintf("Could not unmarshal metric name from bytes %q: %v", string(mnBytes), err))
				}
				if !seenGlobalIndexEntries[metricID] {
					legacyIDB.createGlobalIndexes(&tsid, &mn)
					seenGlobalIndexEntries[metricID] = true
				}
				dateMetricID := dateMetricID{
					date:     date,
					metricID: metricID,
				}
				if !seenPerDayIndexEntries[dateMetricID] {
					legacyIDB.createPerDayIndexes(date, &tsid, &mn)
					seenPerDayIndexEntries[dateMetricID] = true
				}
			}
		}
		is := idb.getIndexSearch(noDeadline)
		dmis, err := is.loadDeletedMetricIDs()
		idb.putIndexSearch(is)
		if err != nil {
			panic(fmt.Sprintf("cannot load deleted metricIDs for indexDB %q: %v", idb.name, err))
		}
		legacyIDB.saveDeletedMetricIDs(dmis)
	}

	s.tb.PutPartitions(ptws)
	legacyIDB.MustClose()

	// Stop storage, delete partition idbs, remove caches, move legacy idb dir
	// to its original location, keep only 2 recent legacy idbs.
	s.MustClose()
	fs.MustRemoveDir(filepath.Join(storageDataPath, dataDirname, indexdbDirname))
	fs.MustRemoveDir(filepath.Join(storageDataPath, cacheDirname))
	if err := os.Rename(legacyIDBsPathTmp, legacyIDBsPathOrig); err != nil {
		panic(fmt.Sprintf("could not rename %q to %q: %v", legacyIDBsPathTmp, legacyIDBsPathOrig, err))
	}
	entries := fs.MustReadDir(legacyIDBsPathOrig)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i] < names[j]
	})
	if len(names) > 2 {
		for _, name := range names[:len(names)-2] {
			p := filepath.Join(legacyIDBsPathOrig, name)
			fs.MustRemoveDir(p)
		}
	}

	return MustOpenStorage(s.path, OpenOptions{})
}

func TestLegacyNextRetentionDeadlineSeconds(t *testing.T) {
	f := func(currentTime string, retention, offset time.Duration, deadlineExpected string) {
		t.Helper()

		now, err := time.Parse(time.RFC3339, currentTime)
		if err != nil {
			t.Fatalf("cannot parse currentTime=%q: %s", currentTime, err)
		}

		d := legacyNextRetentionDeadlineSeconds(now.Unix(), int64(retention.Seconds()), int64(offset.Seconds()))
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

func TestLegacyStorageRotateIndexDB_AddRows(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	op := func(s *Storage) {
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
	}
	testLegacyRotateIndexDB(t, mrs, op)
}

func TestLegacyStorageRotateIndexDB_RegisterMetricNames(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	op := func(s *Storage) {
		s.RegisterMetricNames(nil, mrs)
		s.DebugFlush()
	}
	testLegacyRotateIndexDB(t, mrs, op)
}

func TestLegacyStorageRotateIndexDB_DeleteSeries(t *testing.T) {
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
	testLegacyRotateIndexDB(t, mrs, op)
}

func TestLegacyStorageRotateIndexDB_CreateSnapshot(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	op := func(s *Storage) {
		_ = s.MustCreateSnapshot()
	}
	testLegacyRotateIndexDB(t, mrs, op)
}

func TestLegacyStorageRotateIndexDB_SearchMetricNames(t *testing.T) {
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

	testLegacyRotateIndexDB(t, mrs, op)
}

func TestLegacyStorageRotateIndexDB_SearchLabelNames(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)

	testLegacyRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.SearchLabelNames(nil, []*TagFilters{}, tr, 1e6, 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchLabelNames() failed unexpectedly: %v", err))
		}
	})
}

func TestLegacyStorageRotateIndexDB_SearchLabelValues(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)

	testLegacyRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.SearchLabelValues(nil, "__name__", []*TagFilters{}, tr, 1e6, 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchLabelValues() failed unexpectedly: %v", err))
		}
	})
}

func TestLegacyStorageRotateIndexDB_SearchTagValueSuffixes(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric.", tr)

	testLegacyRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.SearchTagValueSuffixes(nil, tr, "", "metric.", '.', 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchTagValueSuffixes() failed unexpectedly: %v", err))
		}
	})
}

func TestLegacyStorageRotateIndexDB_SearchGraphitePaths(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric.", tr)

	testLegacyRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.SearchGraphitePaths(nil, tr, []byte("*.*"), 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("SearchGraphitePaths() failed unexpectedly: %v", err))
		}
	})
}

func TestLegacyStorageRotateIndexDB_GetSeriesCount(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)

	testLegacyRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.GetSeriesCount(noDeadline)
		if err != nil {
			panic(fmt.Sprintf("GetSeriesCount() failed unexpectedly: %v", err))
		}
	})
}

func TestLegacyStorageRotateIndexDB_GetTSDBStatus(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	tr := TimeRange{
		MinTimestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", tr)
	date := uint64(tr.MinTimestamp) / msecPerDay

	testLegacyRotateIndexDB(t, mrs, func(s *Storage) {
		_, err := s.GetTSDBStatus(nil, nil, date, "", 10, 1e6, noDeadline)
		if err != nil {
			panic(fmt.Sprintf("GetTSDBStatus failed unexpectedly: %v", err))
		}
	})
}

func TestLegacyStorageRotateIndexDB_NotifyReadWriteMode(t *testing.T) {
	op := func(s *Storage) {
		// Set readonly so that the background workers started by
		// notifyReadWriteMode exit early.
		s.isReadOnly.Store(true)
		s.notifyReadWriteMode()
	}

	testLegacyRotateIndexDB(t, []MetricRow{}, op)
}

func TestLegacyStorageRotateIndexDB_UpdateMetrics(t *testing.T) {
	op := func(s *Storage) {
		s.UpdateMetrics(&Metrics{})
	}

	testLegacyRotateIndexDB(t, []MetricRow{}, op)
}

func TestLegacyStorageRotateIndexDB_Search(t *testing.T) {
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

	testLegacyRotateIndexDB(t, mrs, func(s *Storage) {
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

// testLegacyRotateIndexDB checks that storage handles gracefully indexDB rotation
// that happens concurrently with some operation (ingestion or search). The
// operation is expected to finish successfully and there must be no panics.
func testLegacyRotateIndexDB(t *testing.T, mrs []MetricRow, op func(s *Storage)) {
	defer testRemoveAll(t)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()
	// Convert to legacy 2 times in order to have both prev and curr legacy idbs.
	s = mustConvertToLegacy(s)
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
	defer s.MustClose()

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
		s.legacyMustRotateIndexDB(time.Now())
	}

	close(stop)
	wg.Wait()
}

func TestMustOpenLegacyIndexDBTables_noTables(t *testing.T) {
	defer testRemoveAll(t)

	storageDataPath := t.Name()
	s := MustOpenStorage(storageDataPath, OpenOptions{})
	defer s.MustClose()
	legacyIDBs := s.legacyIndexDBs.Load()
	assertIndexDBIsNil(t, legacyIDBs.getIDBPrev())
	assertIndexDBIsNil(t, legacyIDBs.getIDBCurr())
}

func TestMustOpenLegacyIndexDBTables_prevOnly(t *testing.T) {
	defer testRemoveAll(t)

	storageDataPath := t.Name()
	idbPath := filepath.Join(storageDataPath, indexdbDirname)

	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(idbPath, prevName)
	createEmptyIndexdb(prevPath)
	assertPathsExist(t, prevPath)

	s := MustOpenStorage(storageDataPath, OpenOptions{})
	defer s.MustClose()
	legacyIDBs := s.legacyIndexDBs.Load()
	assertIndexDBName(t, legacyIDBs.getIDBPrev(), prevName)
	assertIndexDBIsNil(t, legacyIDBs.getIDBCurr())
}

func TestMustOpenLegacyIndexDBTables_currAndPrev(t *testing.T) {
	defer testRemoveAll(t)

	storageDataPath := t.Name()
	idbPath := filepath.Join(storageDataPath, indexdbDirname)

	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(idbPath, prevName)
	createEmptyIndexdb(prevPath)

	currName := "123456789ABCDEF1"
	currPath := filepath.Join(idbPath, currName)
	createEmptyIndexdb(currPath)

	assertPathsExist(t, prevPath, currPath)

	s := MustOpenStorage(storageDataPath, OpenOptions{})
	defer s.MustClose()
	legacyIDBs := s.legacyIndexDBs.Load()
	assertIndexDBName(t, legacyIDBs.getIDBPrev(), prevName)
	assertIndexDBName(t, legacyIDBs.getIDBCurr(), currName)
}

func TestMustOpenLegacyIndexDBTables_nextIsRemoved(t *testing.T) {
	defer testRemoveAll(t)

	storageDataPath := t.Name()
	idbPath := filepath.Join(storageDataPath, indexdbDirname)
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(idbPath, prevName)
	createEmptyIndexdb(prevPath)

	currName := "123456789ABCDEF1"
	currPath := filepath.Join(idbPath, currName)
	createEmptyIndexdb(currPath)

	nextName := "123456789ABCDEF2"
	nextPath := filepath.Join(idbPath, nextName)
	createEmptyIndexdb(nextPath)

	assertPathsExist(t, prevPath, currPath, nextPath)

	s := MustOpenStorage(storageDataPath, OpenOptions{})
	defer s.MustClose()
	legacyIDBs := s.legacyIndexDBs.Load()
	assertIndexDBName(t, legacyIDBs.getIDBPrev(), prevName)
	assertIndexDBName(t, legacyIDBs.getIDBCurr(), currName)
	assertPathsDoNotExist(t, nextPath)
}

func TestMustOpenLegacyIndexDBTables_nextAndObsoleteDirsAreRemoved(t *testing.T) {
	defer testRemoveAll(t)

	storageDataPath := t.Name()
	idbPath := filepath.Join(storageDataPath, indexdbDirname)

	obsolete1Name := "123456789ABCDEEE"
	obsolete1Path := filepath.Join(idbPath, obsolete1Name)
	createEmptyIndexdb(obsolete1Path)

	obsolete2Name := "123456789ABCDEEF"
	obsolete2Path := filepath.Join(idbPath, obsolete2Name)
	createEmptyIndexdb(obsolete2Path)

	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(idbPath, prevName)
	createEmptyIndexdb(prevPath)

	currName := "123456789ABCDEF1"
	currPath := filepath.Join(idbPath, currName)
	createEmptyIndexdb(currPath)

	nextName := "123456789ABCDEF2"
	nextPath := filepath.Join(idbPath, nextName)
	createEmptyIndexdb(nextPath)

	assertPathsExist(t, obsolete1Path, obsolete2Path, prevPath, currPath, nextPath)

	s := MustOpenStorage(storageDataPath, OpenOptions{})
	defer s.MustClose()
	legacyIDBs := s.legacyIndexDBs.Load()
	assertIndexDBName(t, legacyIDBs.getIDBPrev(), prevName)
	assertIndexDBName(t, legacyIDBs.getIDBCurr(), currName)
	assertPathsDoNotExist(t, obsolete1Path, obsolete2Path, nextPath)
}

func TestLegacyMustRotateIndexDBs_dirNames(t *testing.T) {
	defer testRemoveAll(t)

	storageDataPath := t.Name()
	idbPath := filepath.Join(storageDataPath, indexdbDirname)

	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(idbPath, prevName)
	createEmptyIndexdb(prevPath)

	currName := "123456789ABCDEF1"
	currPath := filepath.Join(idbPath, currName)
	createEmptyIndexdb(currPath)

	assertPathsExist(t, prevPath, currPath)

	s := MustOpenStorage(storageDataPath, OpenOptions{})
	defer s.MustClose()
	legacyIDBs := s.legacyIndexDBs.Load()
	assertIndexDBName(t, legacyIDBs.getIDBPrev(), prevName)
	assertIndexDBName(t, legacyIDBs.getIDBCurr(), currName)
	assertPathsExist(t, prevPath, currPath)

	s.legacyMustRotateIndexDB(time.Now())
	legacyIDBs = s.legacyIndexDBs.Load()
	assertIndexDBName(t, legacyIDBs.getIDBPrev(), currName)
	assertIndexDBIsNil(t, legacyIDBs.getIDBCurr())
	assertPathsDoNotExist(t, prevPath)
	assertPathsExist(t, currPath)

	s.legacyMustRotateIndexDB(time.Now())
	legacyIDBs = s.legacyIndexDBs.Load()
	assertIndexDBIsNil(t, legacyIDBs.getIDBPrev())
	assertIndexDBIsNil(t, legacyIDBs.getIDBCurr())
	assertPathsDoNotExist(t, prevPath, currPath)
}

func createEmptyIndexdb(path string) {
	fs.MustMkdirIfNotExist(path)
	partsFilePath := filepath.Join(path, "parts.json")
	fs.MustWriteAtomic(partsFilePath, []byte("[]"), false)
}

func assertPathsExist(t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		if !fs.IsPathExist(path) {
			t.Fatalf("path does not exist: %s", path)
		}
	}
}

func assertPathsDoNotExist(t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		if fs.IsPathExist(path) {
			t.Fatalf("path exists: %s", path)
		}
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
