package tests

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestStorageUsesSparseCacheForFinalMerge(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartVmsingle("sparse-cache-final-merge", []string{`-retentionPeriod=100y`, `-downsampling.period={__name__=~"metric.*"}:5m:1s`})

	// insert metrics daily over past 2 months
	const metricsPerStep = 10
	const steps = 35
	const stepSize = int64(1 * 24 * 60 * 60 * 1000)
	records := make([]string, metricsPerStep)
	ts := time.Now().Add(-2 * 30 * 24 * time.Hour).UnixMilli()
	for range steps {
		for i := range metricsPerStep {
			name := fmt.Sprintf("metric_%d", i)
			records[i] = fmt.Sprintf("%s %d %d", name, rand.IntN(1000), ts)
		}
		sut.PrometheusAPIV1ImportPrometheus(t, records, apptest.QueryOpts{})
		ts += stepSize
	}
	sut.ForceFlush(t)
	sut.ForceMerge(t)

	// todo: replace with a more reliable way to check if the merge is completed
	// wait for merge to be completed
	time.Sleep(5 * time.Second)

	v := sut.GetIntMetric(t, `vm_cache_requests_total{type="indexdb/dataBlocksSparse"}`)
	if v <= 0 {
		t.Fatalf(`unexpected vm_cache_requests_total{type="indexdb/dataBlocksSparse"} value: %d`, v)
	}
}

func TestStorageDoesNotUseSparseCacheForRegularMerge(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartVmsingle("sparse-cache-regular-merge", []string{`-retentionPeriod=100y`, `-downsampling.period={__name__=~"metric.*"}:5m:1s`})

	// insert metrics into current month only
	const metricsPerStep = 10
	const steps = 2
	const stepSize = 60 * 1000
	records := make([]string, metricsPerStep)
	ts := time.Now().Add(-1 * time.Hour).UnixMilli()
	for range steps {
		for i := range metricsPerStep {
			name := fmt.Sprintf("metric_%d", i)
			records[i] = fmt.Sprintf("%s %d %d", name, rand.IntN(1000), ts)
		}
		sut.PrometheusAPIV1ImportPrometheus(t, records, apptest.QueryOpts{})
		ts += stepSize
	}
	sut.ForceFlush(t)
	sut.ForceMerge(t)

	// todo: replace with a more reliable way to check if the merge is completed
	// wait for merge to be completed
	time.Sleep(5 * time.Second)

	v := sut.GetIntMetric(t, `vm_cache_requests_total{type="indexdb/dataBlocksSparse"}`)
	if v == 0 {
		t.Fatalf(`unexpected vm_cache_requests_total{type="indexdb/dataBlocksSparse"} value: %d`, v)
	}
}
