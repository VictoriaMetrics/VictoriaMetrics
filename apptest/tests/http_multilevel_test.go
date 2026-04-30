package tests

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// TestClusterHTTPSelectMultilevelBasic verifies that a vmselect node configured
// with -clusterSelectNode (HTTP-based internal API) correctly fans out queries
// to lower-level vmselect nodes and returns the full result set.
//
// Architecture:
//
//	vminsert --> vmstorage <-- vmselect (L1)  <-- vmselect (L2, -clusterSelectNode=http://L1)
func TestClusterHTTPSelectMultilevelBasic(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselectL1 := tc.MustStartVmselect("vmselect-level1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	// L2 communicates with L1 via HTTP internal API (-clusterSelectNode), not TCP.
	vmselectL2 := tc.MustStartVmselect("vmselect-level2", []string{
		"-clusterSelectNode=http://" + vmselectL1.HTTPAddr(),
	})

	const numMetrics = 500
	records := make([]string, numMetrics)
	wantData := make([]map[string]string, numMetrics)
	for i := range numMetrics {
		name := fmt.Sprintf("http_metric_%d", i)
		records[i] = fmt.Sprintf("%s{job=\"test\",instance=\"host%d\"} %d", name, i%10, rand.IntN(1000))
		wantData[i] = map[string]string{
			"__name__": name,
			"job":      "test",
			"instance": fmt.Sprintf("host%d", i%10),
		}
	}

	qopts := apptest.QueryOpts{Tenant: "0"}
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, qopts)
	vmstorage.ForceFlush(t)

	want := &apptest.PrometheusAPIV1SeriesResponse{
		Status:    "success",
		IsPartial: false,
		Data:      wantData,
	}
	want.Sort()

	// Both L1 and L2 must return the complete, non-partial series set.
	for _, app := range []*apptest.Vmselect{vmselectL1, vmselectL2} {
		tc.Assert(&apptest.AssertOptions{
			Msg: fmt.Sprintf("unexpected /api/v1/series response from %s", app),
			Got: func() any {
				res := app.PrometheusAPIV1Series(t, `{__name__=~"http_metric_.*"}`, qopts)
				res.Sort()
				return res
			},
			Want: want,
		})
	}
}

// TestClusterHTTPSelectMultilevelLabelNamesAndValues verifies that label name
// and label value queries are correctly served through an HTTP-based multi-level
// cluster, including deduplication of results returned from multiple nodes.
func TestClusterHTTPSelectMultilevelLabelNamesAndValues(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselectL1 := tc.MustStartVmselect("vmselect-level1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	vmselectL2 := tc.MustStartVmselect("vmselect-level2", []string{
		"-clusterSelectNode=http://" + vmselectL1.HTTPAddr(),
	})

	records := []string{
		`cpu_usage{env="prod",region="us-east"} 0.8`,
		`cpu_usage{env="staging",region="eu-west"} 0.4`,
		`mem_usage{env="prod",region="us-east"} 1024`,
	}
	qopts := apptest.QueryOpts{Tenant: "0"}
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, qopts)
	vmstorage.ForceFlush(t)

	timeOpts := apptest.QueryOpts{
		Tenant: "0",
		Start:  fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix()),
		End:    fmt.Sprintf("%d", time.Now().Add(10*time.Minute).Unix()),
	}

	// --- Label names ---
	wantLabels := &apptest.PrometheusAPIV1LabelsResponse{
		Status:    "success",
		IsPartial: false,
		Data:      []string{"__name__", "env", "region"},
	}

	for _, app := range []*apptest.Vmselect{vmselectL1, vmselectL2} {
		tc.Assert(&apptest.AssertOptions{
			Msg: fmt.Sprintf("unexpected /api/v1/labels from %s", app),
			Got: func() any {
				res := app.PrometheusAPIV1Labels(t, `{__name__=~"cpu_usage|mem_usage"}`, timeOpts)
				sort.Strings(res.Data)
				return res
			},
			Want: wantLabels,
		})
	}

	// --- Label values for "env" ---
	wantEnvValues := &apptest.PrometheusAPIV1LabelValuesResponse{
		Status:    "success",
		IsPartial: false,
		Data:      []string{"prod", "staging"},
	}

	for _, app := range []*apptest.Vmselect{vmselectL1, vmselectL2} {
		tc.Assert(&apptest.AssertOptions{
			Msg: fmt.Sprintf("unexpected /api/v1/label/env/values from %s", app),
			Got: func() any {
				res := app.PrometheusAPIV1LabelValues(t, "env", `{__name__=~"cpu_usage|mem_usage"}`, timeOpts)
				sort.Strings(res.Data)
				return res
			},
			Want: wantEnvValues,
		})
	}

	// --- Label values for "__name__" ---
	wantNameValues := &apptest.PrometheusAPIV1LabelValuesResponse{
		Status:    "success",
		IsPartial: false,
		Data:      []string{"cpu_usage", "mem_usage"},
	}

	for _, app := range []*apptest.Vmselect{vmselectL1, vmselectL2} {
		tc.Assert(&apptest.AssertOptions{
			Msg: fmt.Sprintf("unexpected /api/v1/label/__name__/values from %s", app),
			Got: func() any {
				res := app.PrometheusAPIV1LabelValues(t, "__name__", `{env="prod"}`, timeOpts)
				sort.Strings(res.Data)
				return res
			},
			Want: wantNameValues,
		})
	}
}

// TestClusterHTTPSelectMultilevelSeriesCount verifies that series count queries
// are aggregated correctly across HTTP-based multi-level select nodes.
func TestClusterHTTPSelectMultilevelSeriesCount(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselectL1 := tc.MustStartVmselect("vmselect-level1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	vmselectL2 := tc.MustStartVmselect("vmselect-level2", []string{
		"-clusterSelectNode=http://" + vmselectL1.HTTPAddr(),
	})

	const numSeries = 20
	records := make([]string, numSeries)
	for i := range numSeries {
		records[i] = fmt.Sprintf("series_count_metric_%d %d", i, rand.IntN(100))
	}
	qopts := apptest.QueryOpts{Tenant: "0"}
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, qopts)
	vmstorage.ForceFlush(t)

	wantCount := &apptest.PrometheusAPIV1SeriesCountResponse{
		Status:    "success",
		IsPartial: false,
		Data:      []uint64{numSeries},
	}

	for _, app := range []*apptest.Vmselect{vmselectL1, vmselectL2} {
		tc.Assert(&apptest.AssertOptions{
			Msg: fmt.Sprintf("unexpected /api/v1/series/count from %s", app),
			Got: func() any {
				return app.PrometheusAPIV1SeriesCount(t, qopts)
			},
			Want: wantCount,
		})
	}
}

// TestClusterHTTPSelectMultilevelInstantQuery verifies that instant queries
// (PromQL /api/v1/query) return correct results through HTTP-based multi-level
// select.
func TestClusterHTTPSelectMultilevelInstantQuery(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselectL1 := tc.MustStartVmselect("vmselect-level1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	vmselectL2 := tc.MustStartVmselect("vmselect-level2", []string{
		"-clusterSelectNode=http://" + vmselectL1.HTTPAddr(),
	})

	qopts := apptest.QueryOpts{Tenant: "0"}
	vminsert.PrometheusAPIV1ImportPrometheus(t, []string{
		`instant_query_metric{job="test"} 42`,
	}, qopts)
	vmstorage.ForceFlush(t)

	// Both levels should return the same instant query result.
	// Use a 5-minute lookback window to cover the just-inserted sample.
	queryTime := fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix())
	for _, app := range []*apptest.Vmselect{vmselectL1, vmselectL2} {
		res := app.PrometheusAPIV1Query(t, `instant_query_metric{job="test"}`, apptest.QueryOpts{
			Tenant: "0",
			Time:   queryTime,
		})
		if res.Status != "success" {
			t.Fatalf("[%s] expected status=success, got %q", app, res.Status)
		}
		if res.IsPartial {
			t.Fatalf("[%s] expected isPartial=false, got true", app)
		}
		if len(res.Data.Result) != 1 {
			t.Fatalf("[%s] expected 1 result, got %d: %v", app, len(res.Data.Result), res.Data.Result)
		}
	}
}

// TestClusterHTTPSelectMultilevelPartialResponse verifies that partial response
// handling is correctly propagated when one HTTP-based lower-level vmselect is
// unavailable.
//
// Architecture:
//
//	                              |--> vmselectL1a (healthy) --> vmstorage1
//	global-vmselect-L2 -----------|
//	                              |--> vmselectL1b (one storage down, partial)
func TestClusterHTTPSelectMultilevelPartialResponse(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage1 := tc.MustStartVmstorage("vmstorage1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage1",
	})
	vmstorage2 := tc.MustStartVmstorage("vmstorage2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage2",
	})

	// L1a: healthy — both storages are available.
	vmselectL1a := tc.MustStartVmselect("vmselect-l1a", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr(),
	})
	// L1b: one storage node is unreachable — will return partial responses.
	vmselectL1b := tc.MustStartVmselect("vmselect-l1b", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + noopTCPServerAddr(t),
	})
	// L2: connects to both L1 nodes via HTTP.
	vmselectL2 := tc.MustStartVmselect("vmselect-l2", []string{
		"-clusterSelectNode=" +
			"http://" + vmselectL1a.HTTPAddr() + "," +
			"http://" + vmselectL1b.HTTPAddr(),
	})

	qopts := apptest.QueryOpts{Tenant: "0"}
	start := time.Now().Unix()
	timeOpts := apptest.QueryOpts{
		Tenant: "0",
		Start:  fmt.Sprintf("%d", start-60),
		End:    fmt.Sprintf("%d", start),
	}

	// 1. /api/v1/query
	// L1a must return a full response.
	resL1a := vmselectL1a.PrometheusAPIV1Query(t, `{__name__=~".*"}`, qopts)
	if resL1a.IsPartial {
		t.Errorf("vmselect-l1a /api/v1/query: expected isPartial=false, got true")
	}

	// L1b must return a partial response (one storage down).
	resL1b := vmselectL1b.PrometheusAPIV1Query(t, `{__name__=~".*"}`, qopts)
	if !resL1b.IsPartial {
		t.Errorf("vmselect-l1b /api/v1/query: expected isPartial=true, got false")
	}

	// L2 must propagate the partial flag from L1b.
	resL2 := vmselectL2.PrometheusAPIV1Query(t, `{__name__=~".*"}`, qopts)
	if !resL2.IsPartial {
		t.Errorf("vmselect-l2 /api/v1/query: expected isPartial=true (propagated from l1b), got false")
	}

	// 2. /api/v1/labels
	// L1a must return full label list.
	labelsL1a := vmselectL1a.PrometheusAPIV1Labels(t, `{__name__="up"}`, timeOpts)
	if labelsL1a.IsPartial {
		t.Errorf("vmselect-l1a /api/v1/labels: expected isPartial=false, got true")
	}

	// L1b must return partial labels.
	labelsL1b := vmselectL1b.PrometheusAPIV1Labels(t, `{__name__="up"}`, timeOpts)
	if !labelsL1b.IsPartial {
		t.Errorf("vmselect-l1b /api/v1/labels: expected isPartial=true, got false")
	}

	// L2 must propagate the partial flag.
	labelsL2 := vmselectL2.PrometheusAPIV1Labels(t, `{__name__="up"}`, timeOpts)
	if !labelsL2.IsPartial {
		t.Errorf("vmselect-l2 /api/v1/labels: expected isPartial=true (propagated from l1b), got false")
	}

	// 3. /api/v1/series
	// L1a must return full series.
	seriesL1a := vmselectL1a.PrometheusAPIV1Series(t, `{__name__="up"}`, timeOpts)
	if seriesL1a.IsPartial {
		t.Errorf("vmselect-l1a /api/v1/series: expected isPartial=false, got true")
	}

	// L1b must return partial series.
	seriesL1b := vmselectL1b.PrometheusAPIV1Series(t, `{__name__="up"}`, timeOpts)
	if !seriesL1b.IsPartial {
		t.Errorf("vmselect-l1b /api/v1/series: expected isPartial=true, got false")
	}

	// L2 must propagate the partial flag.
	seriesL2 := vmselectL2.PrometheusAPIV1Series(t, `{__name__="up"}`, timeOpts)
	if !seriesL2.IsPartial {
		t.Errorf("vmselect-l2 /api/v1/series: expected isPartial=true (propagated from l1b), got false")
	}
}

// TestClusterHTTPSelectMultilevelThreeLevels verifies a three-level HTTP-based
// cluster: vmselect L3 → L2 → L1 → vmstorage.
//
// This tests that the HTTP internal API can be chained across more than two
// levels while still returning complete, non-partial results when all nodes are
// healthy.
func TestClusterHTTPSelectMultilevelThreeLevels(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselectL1 := tc.MustStartVmselect("vmselect-level1", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	vmselectL2 := tc.MustStartVmselect("vmselect-level2", []string{
		"-clusterSelectNode=http://" + vmselectL1.HTTPAddr(),
	})
	vmselectL3 := tc.MustStartVmselect("vmselect-level3", []string{
		"-clusterSelectNode=http://" + vmselectL2.HTTPAddr(),
	})

	const numMetrics = 100
	records := make([]string, numMetrics)
	wantData := make([]map[string]string, numMetrics)
	for i := range numMetrics {
		name := fmt.Sprintf("three_level_metric_%d", i)
		records[i] = fmt.Sprintf("%s %d", name, rand.IntN(100))
		wantData[i] = map[string]string{"__name__": name}
	}

	qopts := apptest.QueryOpts{Tenant: "0"}
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, qopts)
	vmstorage.ForceFlush(t)

	want := &apptest.PrometheusAPIV1SeriesResponse{
		Status:    "success",
		IsPartial: false,
		Data:      wantData,
	}
	want.Sort()

	for _, app := range []*apptest.Vmselect{vmselectL1, vmselectL2, vmselectL3} {
		tc.Assert(&apptest.AssertOptions{
			Msg: fmt.Sprintf("unexpected /api/v1/series from %s", app),
			Got: func() any {
				res := app.PrometheusAPIV1Series(t, `{__name__=~"three_level_metric_.*"}`, qopts)
				res.Sort()
				return res
			},
			Want: want,
		})
	}
}

// TestClusterHTTPSelectMultilevelVsTCPMultilevel verifies that the HTTP-based
// multi-level setup returns identical results to the equivalent TCP-based
// (clusternative) multi-level setup for the same data.
//
// Architecture:
//
//	vminsert --> vmstorage <-- vmselectTCP (L1, -storageNode)
//	                      <-- vmselectHTTP (L1, -storageNode)
//
//	vmselectViaTCP  (L2) uses -storageNode  = vmselectTCP.ClusternativeListenAddr
//	vmselectViaHTTP (L2) uses -clusterSelectNode = http://vmselectHTTP.HTTPAddr
func TestClusterHTTPSelectMultilevelVsTCPMultilevel(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})

	// Two independent L1 nodes pointing to the same vmstorage.
	vmselectL1tcp := tc.MustStartVmselect("vmselect-l1-tcp", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	vmselectL1http := tc.MustStartVmselect("vmselect-l1-http", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})

	// L2 via TCP clusternative.
	vmselectL2tcp := tc.MustStartVmselect("vmselect-l2-tcp", []string{
		"-storageNode=" + vmselectL1tcp.ClusternativeListenAddr(),
	})
	// L2 via HTTP.
	vmselectL2http := tc.MustStartVmselect("vmselect-l2-http", []string{
		"-clusterSelectNode=http://" + vmselectL1http.HTTPAddr(),
	})

	const numMetrics = 200
	records := make([]string, numMetrics)
	wantData := make([]map[string]string, numMetrics)
	for i := range numMetrics {
		name := fmt.Sprintf("compare_metric_%d", i)
		records[i] = fmt.Sprintf("%s %d", name, rand.IntN(100))
		wantData[i] = map[string]string{"__name__": name}
	}

	qopts := apptest.QueryOpts{Tenant: "0"}
	vminsert.PrometheusAPIV1ImportPrometheus(t, records, qopts)
	vmstorage.ForceFlush(t)

	want := &apptest.PrometheusAPIV1SeriesResponse{
		Status:    "success",
		IsPartial: false,
		Data:      wantData,
	}
	want.Sort()

	for _, app := range []*apptest.Vmselect{vmselectL2tcp, vmselectL2http} {
		tc.Assert(&apptest.AssertOptions{
			Msg: fmt.Sprintf("unexpected /api/v1/series from %s", app),
			Got: func() any {
				res := app.PrometheusAPIV1Series(t, `{__name__=~"compare_metric_.*"}`, qopts)
				res.Sort()
				return res
			},
			Want: want,
		})
	}
}
