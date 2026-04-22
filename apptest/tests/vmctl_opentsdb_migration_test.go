package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestSingleVmctlOpenTSDBProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())

	// Generate 60 points at 1-minute intervals starting 2 hours ago.
	// This ensures data falls within vmctl's default query window (now - retention).
	baseTS := time.Now().Add(-2 * time.Hour).Truncate(time.Minute).Unix()
	points := make([]openTSDBPoint, 0, 60)
	for i := range 60 {
		points = append(points, openTSDBPoint{
			Metric:    "test.cpu",
			Tags:      map[string]string{"host": "h1", "env": "prod"},
			Timestamp: baseTS + int64(i*60),
			Value:     float64(i),
		})
	}

	otsdb := newOpenTSDBMockServer(t, points)
	defer otsdb.close()

	vmctlFlags := []string{
		`opentsdb`,
		`--otsdb-addr=` + otsdb.httpAddr(),
		`--vm-addr=` + vmAddr,
		`--otsdb-retentions=ssum-1m-avg:1d:1d`,
		`--otsdb-filters=test`,
		`--otsdb-normalize`,
		`--disable-progress-bar=true`,
		`-s`,
	}

	testOpenTSDBProtocol(tc, vmsingleDst, vmctlFlags, points, "test_cpu", baseTS)
}

func TestClusterVmctlOpenTSDBProtocol(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	cluster := tc.MustStartDefaultCluster()
	vmAddr := fmt.Sprintf("http://%s/", cluster.Vminsert.HTTPAddr())

	// Generate 60 points at 1-minute intervals starting 2 hours ago.
	baseTS := time.Now().Add(-2 * time.Hour).Truncate(time.Minute).Unix()
	points := make([]openTSDBPoint, 0, 60)
	for i := range 60 {
		points = append(points, openTSDBPoint{
			Metric:    "test.mem",
			Tags:      map[string]string{"host": "h1"},
			Timestamp: baseTS + int64(i*60),
			Value:     float64(i * 2),
		})
	}

	otsdb := newOpenTSDBMockServer(t, points)
	defer otsdb.close()

	vmctlFlags := []string{
		`opentsdb`,
		`--otsdb-addr=` + otsdb.httpAddr(),
		`--vm-addr=` + vmAddr,
		`--otsdb-retentions=sum-1m-avg:1d:1d`,
		`--otsdb-filters=test`,
		`--otsdb-normalize`,
		`--disable-progress-bar=true`,
		`--vm-account-id=0`,
		`-s`,
	}

	testOpenTSDBProtocol(tc, cluster, vmctlFlags, points, "test_mem", baseTS)
}

func testOpenTSDBProtocol(
	tc *apptest.TestCase,
	queries apptest.PrometheusWriteQuerier,
	vmctlFlags []string,
	points []openTSDBPoint,
	vmMetricName string,
	baseTS int64,
) {
	t := tc.T()
	t.Helper()

	// Build dynamic time range covering all data points with 1-hour padding.
	queryStart := time.Unix(baseTS-3600, 0).UTC().Format(time.RFC3339)
	queryEnd := time.Unix(baseTS+7200, 0).UTC().Format(time.RFC3339)

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	got := queries.PrometheusAPIV1Query(t, `{__name__=~".*"}`, apptest.QueryOpts{
		Step: "5m",
		Time: queryStart,
	})
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	tc.MustStartVmctl("vmctl", vmctlFlags)
	queries.ForceFlush(t)

	expected := buildExpectedOpenTSDBResult(points, vmMetricName)

	tc.Assert(&apptest.AssertOptions{
		Retries: 300,
		Msg:     `unexpected metrics stored via opentsdb protocol`,
		Got: func() any {
			r := queries.PrometheusAPIV1Export(t, fmt.Sprintf(`{__name__=%q}`, vmMetricName), apptest.QueryOpts{
				Start: queryStart,
				End:   queryEnd,
			})
			r.Sort()
			return r.Data.Result
		},
		Want: expected,
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}

func buildExpectedOpenTSDBResult(points []openTSDBPoint, vmMetricName string) []*apptest.QueryResult {
	grouped := map[string]*apptest.QueryResult{}
	for _, p := range points {
		metric := map[string]string{"__name__": vmMetricName}
		for k, v := range p.Tags {
			metric[k] = v
		}
		key := tagsKey(metric)
		if _, ok := grouped[key]; !ok {
			grouped[key] = &apptest.QueryResult{Metric: metric}
		}
		grouped[key].Samples = append(grouped[key].Samples, &apptest.Sample{
			Timestamp: p.Timestamp * 1000,
			Value:     p.Value,
		})
	}
	out := make([]*apptest.QueryResult, 0, len(grouped))
	for _, v := range grouped {
		out = append(out, v)
	}
	resp := apptest.PrometheusAPIV1QueryResponse{
		Data: &apptest.QueryData{Result: out},
	}
	resp.Sort()
	return resp.Data.Result
}
