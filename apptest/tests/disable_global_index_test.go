package tests

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

type testLegacy156Opts struct {
	disablePerDayIndex bool
	startLegacySUT     func() apptest.PrometheusWriteQuerier
	startSUT           func() apptest.PrometheusWriteQuerier
	stopLegacySUT      func()
	stopSUT            func()
	dropCaches         func()
}

func TestLegacy156SingleUpgradeDowngrade(t *testing.T) {
	t.Run("disablePerDayIndex=false", func(t *testing.T) {
		testLegacy156SingleUpgradeDowngrade(t, false)
	})
	t.Run("disablePerDayIndex=true", func(t *testing.T) {
		testLegacy156SingleUpgradeDowngrade(t, true)
	})
}

func testLegacy156SingleUpgradeDowngrade(t *testing.T, disablePerDayIndex bool) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	storageDataPath := filepath.Join(tc.Dir(), "vmsingle")
	metricIDCachePath := filepath.Join(storageDataPath, "cache", "metricID_tsid")
	metricNameCachePath := filepath.Join(storageDataPath, "cache", "metricName_tsid")

	opts := testLegacy156Opts{
		disablePerDayIndex: disablePerDayIndex,
		startLegacySUT: func() apptest.PrometheusWriteQuerier {
			return tc.MustStartVmsingle_v1_156_0("vmsingle-legacy", []string{
				"-storageDataPath=" + storageDataPath,
				"-retentionPeriod=100y",
				fmt.Sprintf("-disablePerDayIndex=%t", disablePerDayIndex),
			})
		},
		startSUT: func() apptest.PrometheusWriteQuerier {
			return tc.MustStartVmsingle("vmsingle-new", []string{
				"-storageDataPath=" + storageDataPath,
				"-retentionPeriod=100y",
				fmt.Sprintf("-disablePerDayIndex=%t", disablePerDayIndex),
			})
		},
		stopLegacySUT: func() {
			tc.StopApp("vmsingle-legacy")
		},
		stopSUT: func() {
			tc.StopApp("vmsingle-new")
		},
		dropCaches: func() {
			fs.MustRemoveDir(metricIDCachePath)
			fs.MustRemoveDir(metricNameCachePath)
		},
	}

	testLegacy156UpgradeDowngrade(tc, opts)
}

func TestLegacy156ClusterUpgradeDowngrade(t *testing.T) {
	t.Run("disablePerDayIndex=false", func(t *testing.T) {
		testLegacy156ClusterUpgradeDowngrade(t, false)
	})
	t.Run("disablePerDayIndex=true", func(t *testing.T) {
		testLegacy156ClusterUpgradeDowngrade(t, true)
	})
}

func testLegacy156ClusterUpgradeDowngrade(t *testing.T, disablePerDayIndex bool) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	opts := testLegacy156Opts{}
	testLegacy156UpgradeDowngrade(tc, opts)
}

func testLegacy156UpgradeDowngrade(tc *apptest.TestCase, opts testLegacy156Opts) {
	t := tc.T()

	const numMetrics = 1

	now := time.Now().UTC()
	start1 := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end1 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).UnixMilli()
	legacy1 := apptest.GenerateTestData("legacy1", numMetrics, start1, end1)
	new1 := apptest.GenerateTestData("new1", numMetrics, start1, end1)

	// Start legacy storage, ingest some data, ensure it is searcheable, stop
	// legacy storage.
	sutLegacy := opts.startLegacySUT()
	sutLegacy.PrometheusAPIV1ImportPrometheus(t, legacy1.Samples, apptest.QueryOpts{})
	sutLegacy.ForceFlush(t)
	apptest.AssertSeries(tc, sutLegacy, "legacy.*", "", start1, end1, legacy1.WantSeries)
	apptest.AssertLabels(tc, sutLegacy, "legacy.*", "", start1, end1, legacy1.WantLabels)
	apptest.AssertLabelValues(tc, sutLegacy, "legacy.*", "label", "", start1, end1, legacy1.WantLabelValues)
	apptest.AssertQueryResults(tc, sutLegacy, "legacy.*", "", start1, end1, legacy1.Step, legacy1.WantQueryResults)
	opts.stopLegacySUT()
	// Also drop caches to ensure that the index is searched instead of cache.
	opts.dropCaches()

	// Start new storage and ensure that data ingested with legacy storage is
	// searcheable.
	sutNew := opts.startSUT()
	apptest.AssertSeries(tc, sutNew, "legacy.*", "", start1, end1, legacy1.WantSeries)
	apptest.AssertLabels(tc, sutNew, "legacy.*", "", start1, end1, legacy1.WantLabels)
	apptest.AssertLabelValues(tc, sutNew, "legacy.*", "label", "", start1, end1, legacy1.WantLabelValues)
	apptest.AssertQueryResults(tc, sutNew, "legacy.*", "", start1, end1, legacy1.Step, legacy1.WantQueryResults)

	// Ingest some data with new storage, ensure that it is searcheable, stop
	// new storage.
	sutNew.PrometheusAPIV1ImportPrometheus(t, new1.Samples, apptest.QueryOpts{})
	sutNew.ForceFlush(t)
	apptest.AssertSeries(tc, sutNew, "new.*", "", start1, end1, new1.WantSeries)
	apptest.AssertLabels(tc, sutNew, "new.*", "", start1, end1, new1.WantLabels)
	apptest.AssertLabelValues(tc, sutNew, "new.*", "label", "", start1, end1, new1.WantLabelValues)
	apptest.AssertQueryResults(tc, sutNew, "new.*", "", start1, end1, new1.Step, new1.WantQueryResults)
	opts.stopSUT()
	opts.dropCaches()

	// Start legacy storage again and ensure legacy data is still searcheable.
	sutLegacy = opts.startLegacySUT()
	apptest.AssertSeries(tc, sutLegacy, "legacy.*", "", start1, end1, legacy1.WantSeries)
	apptest.AssertLabels(tc, sutLegacy, "legacy.*", "", start1, end1, legacy1.WantLabels)
	apptest.AssertLabelValues(tc, sutLegacy, "legacy.*", "label", "", start1, end1, legacy1.WantLabelValues)
	apptest.AssertQueryResults(tc, sutLegacy, "legacy.*", "", start1, end1, legacy1.Step, legacy1.WantQueryResults)

	// Then search new data with legacy storage.
	if opts.disablePerDayIndex {
		// If both legacy and new storages were running with per-day index
		// disabled, then new data will be searcheable with legacy storage
		// because in both cases the data was written to global index.
		apptest.AssertSeries(tc, sutLegacy, "new.*", "", start1, end1, new1.WantSeries)
		apptest.AssertLabels(tc, sutLegacy, "new.*", "", start1, end1, new1.WantLabels)
		apptest.AssertLabelValues(tc, sutLegacy, "new.*", "label", "", start1, end1, new1.WantLabelValues)
		apptest.AssertQueryResults(tc, sutLegacy, "new.*", "", start1, end1, new1.Step, new1.WantQueryResults)
	} else {
		// If both legacy and new storages were running with per-day index
		// enabled, then new data will not be searcheable with legacy storage
		// because new storage does not write to global index anymore and legacy
		// storage needs global index data for per-day searches.
		apptest.AssertSeries(tc, sutLegacy, "new.*", "", start1, end1, []map[string]string{})
		apptest.AssertLabels(tc, sutLegacy, "new.*", "", start1, end1, []string{"__name__"})
		apptest.AssertLabelValues(tc, sutLegacy, "new.*", "label", "", start1, end1, []string{})
		apptest.AssertQueryResults(tc, sutLegacy, "new.*", "", start1, end1, new1.Step, []*apptest.QueryResult{})
	}

	opts.stopLegacySUT()
}
