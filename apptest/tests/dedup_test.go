package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestSingleDeduplication_dedulicationIsOff(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle",
		"-retentionPeriod=100y",
		"-dedup.minScrapeInterval=0",
	})

	testDeduplication(tc, sut, false)
}

func TestSingleDeduplication_dedulicationIsOn(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle",
		"-retentionPeriod=100y",
		"-dedup.minScrapeInterval=10s",
	})

	testDeduplication(tc, sut, true)
}

func TestClusterDeduplication_deduplicationIsOff(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartCluster(&apptest.ClusterOptions{
		Vmstorage1Instance: "vmstorage1",
		Vmstorage1Flags: []string{
			"-dedup.minScrapeInterval=0",
		},
		Vmstorage2Instance: "vmstorage2",
		Vmstorage2Flags: []string{
			"-dedup.minScrapeInterval=0",
		},
		VminsertInstance: "vminsert",
		VmselectInstance: "vmselect",
	})

	testDeduplication(tc, sut, false)
}

func TestClusterDeduplication_deduplicationIsOn(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartCluster(&apptest.ClusterOptions{
		Vmstorage1Instance: "vmstorage1",
		Vmstorage1Flags: []string{
			"-dedup.minScrapeInterval=10s",
		},
		Vmstorage2Instance: "vmstorage2",
		Vmstorage2Flags: []string{
			"-dedup.minScrapeInterval=10s",
		},
		VminsertInstance: "vminsert",
		VmselectInstance: "vmselect",
	})

	testDeduplication(tc, sut, true)
}

// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#deduplication
func testDeduplication(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier, deduplicationIsOn bool) {
	t := tc.T()

	firstDayOfThisMonth := func() time.Time {
		t := time.Now().UTC()
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	// Intentionally check that deduplication works for the current month, since
	// by reading the code it may seem that deduplication for the current month
	// is skipped.
	//
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6965
	start := firstDayOfThisMonth()
	end := start.Add(1 * time.Hour)
	ts1 := start.Add(1 * time.Second).UnixMilli()
	ts3 := start.Add(3 * time.Second).UnixMilli()
	ts5 := start.Add(5 * time.Second).UnixMilli()
	ts10 := start.Add(10 * time.Second).UnixMilli()
	data := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{{Name: "__name__", Value: "metric1"}},
				Samples: []prompb.Sample{
					{Timestamp: ts1, Value: 3},
					{Timestamp: ts3, Value: 10},
					{Timestamp: ts5, Value: 5},
				},
			},
			{
				Labels: []prompb.Label{{Name: "__name__", Value: "metric2"}},
				Samples: []prompb.Sample{
					{Timestamp: ts1, Value: 3},
					{Timestamp: ts3, Value: decimal.StaleNaN},
					{Timestamp: ts5, Value: 5},
				},
			},
			{
				Labels: []prompb.Label{{Name: "__name__", Value: "metric3"}},
				Samples: []prompb.Sample{
					{Timestamp: ts10, Value: 30},
					{Timestamp: ts10, Value: 100},
					{Timestamp: ts10, Value: 50},
				},
			},
			{
				Labels: []prompb.Label{{Name: "__name__", Value: "metric4"}},
				Samples: []prompb.Sample{
					{Timestamp: ts10, Value: 30},
					{Timestamp: ts10, Value: decimal.StaleNaN},
					{Timestamp: ts10, Value: 50},
				},
			},
		},
	}

	sut.PrometheusAPIV1Write(t, data, apptest.QueryOpts{})
	sut.ForceFlush(t)
	sut.ForceMerge(t)

	wantDuplicates := &apptest.PrometheusAPIV1QueryResponse{
		Status: "success",
		Data: &apptest.QueryData{
			ResultType: "matrix",
			Result: []*apptest.QueryResult{
				{Metric: map[string]string{"__name__": "metric1"}, Samples: []*apptest.Sample{
					{Timestamp: ts1, Value: 3},
					{Timestamp: ts3, Value: 10},
					{Timestamp: ts5, Value: 5},
				}},
				{Metric: map[string]string{"__name__": "metric2"}, Samples: []*apptest.Sample{
					{Timestamp: ts1, Value: 3},
					{Timestamp: ts3, Value: decimal.StaleNaN},
					{Timestamp: ts5, Value: 5},
				}},
				{Metric: map[string]string{"__name__": "metric3"}, Samples: []*apptest.Sample{
					{Timestamp: ts10, Value: 30},
					{Timestamp: ts10, Value: 50},
					{Timestamp: ts10, Value: 100},
				}},
				{Metric: map[string]string{"__name__": "metric4"}, Samples: []*apptest.Sample{
					{Timestamp: ts10, Value: 30},
					{Timestamp: ts10, Value: 50},
					{Timestamp: ts10, Value: decimal.StaleNaN},
				}},
			},
		},
	}
	wantDeduped := &apptest.PrometheusAPIV1QueryResponse{
		Status: "success",
		Data: &apptest.QueryData{
			ResultType: "matrix",
			Result: []*apptest.QueryResult{
				{Metric: map[string]string{"__name__": "metric1"}, Samples: []*apptest.Sample{
					// VictoriaMetrics leaves a single raw sample with the
					// biggest timestamp for each time series per each
					// -dedup.minScrapeInterval discrete interval if
					// -dedup.minScrapeInterval is set to positive duration.
					{Timestamp: ts5, Value: 5},
				}},
				{Metric: map[string]string{"__name__": "metric2"}, Samples: []*apptest.Sample{
					// Even if NaN is present among duplicates, VictoriaMetrics
					// still chooses the sample with the biggest timestamp.
					{Timestamp: ts5, Value: 5},
				}},
				{Metric: map[string]string{"__name__": "metric3"}, Samples: []*apptest.Sample{
					// If multiple raw samples have the same timestamp on the
					// given -dedup.minScrapeInterval discrete interval, then
					// the sample with the biggest value is kept.
					{Timestamp: ts10, Value: 100},
				}},
				{Metric: map[string]string{"__name__": "metric4"}, Samples: []*apptest.Sample{
					// If multiple raw samples have the same timestamp on the
					// given -dedup.minScrapeInterval discrete interval,
					// always prefer a non-decimal.StaleNaN value,
					// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10196
					{Timestamp: ts10, Value: 50},
				}},
			},
		},
	}

	want := wantDuplicates
	if deduplicationIsOn {
		want = wantDeduped
	}

	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected response",
		Got: func() any {
			got := sut.PrometheusAPIV1Export(t, `{__name__=~"metric.*"}`, apptest.QueryOpts{
				ReduceMemUsage: "1",
				Start:          fmt.Sprintf("%d", start.UnixMilli()),
				End:            fmt.Sprintf("%d", end.UnixMilli()),
			})
			// Delete cluster-specific labels from the metric name since they are
			// irrelevant for the test.
			for _, result := range got.Data.Result {
				delete(result.Metric, "vm_account_id")
				delete(result.Metric, "vm_project_id")
			}
			got.Sort()
			return got
		},
		Want: want,
		CmpOpts: []cmp.Option{
			cmpopts.EquateNaNs(),
		},
	})
}
