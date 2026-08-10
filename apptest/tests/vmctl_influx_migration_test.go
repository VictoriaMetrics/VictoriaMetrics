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

const (
	influxTestDatabase = "testdb"
	influxTestToken    = "my-secret-token"
	// defaultV1CompatUser mirrors the username vmctl sends when authenticating
	// to InfluxDB 2.x with an API token. The 1.x compatibility API requires a
	// username but ignores its value.
	influxV1CompatUser = "vmctl"
)

func TestSingleVmctlInfluxV2Migration(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())

	baseTS := time.Now().Add(-2 * time.Hour).Truncate(time.Minute).Unix()
	points := newInfluxTestPoints(baseTS)

	influx := newInfluxMockServer(t, points)
	defer influx.close()

	vmctlFlags := []string{
		`influx`,
		`--influx-version=2`,
		`--influx-addr=` + influx.httpAddr(),
		`--influx-token=` + influxTestToken,
		`--influx-database=` + influxTestDatabase,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
		`-s`,
	}

	testVmctlInfluxMigration(tc, vmsingleDst, vmctlFlags, points, baseTS)

	assertInfluxRequests(t, influx, influxV1CompatUser, influxTestToken, "")
}

func TestSingleVmctlInfluxV1Migration(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingleDst := tc.MustStartDefaultVmsingle()
	vmAddr := fmt.Sprintf("http://%s/", vmsingleDst.HTTPAddr())

	baseTS := time.Now().Add(-2 * time.Hour).Truncate(time.Minute).Unix()
	points := newInfluxTestPoints(baseTS)

	influx := newInfluxMockServer(t, points)
	defer influx.close()

	vmctlFlags := []string{
		`influx`,
		`--influx-addr=` + influx.httpAddr(),
		`--influx-user=user`,
		`--influx-password=pass`,
		`--influx-database=` + influxTestDatabase,
		`--vm-addr=` + vmAddr,
		`--disable-progress-bar=true`,
		`-s`,
	}

	testVmctlInfluxMigration(tc, vmsingleDst, vmctlFlags, points, baseTS)
	assertInfluxRequests(t, influx, "user", "pass", "autogen")
}

func TestClusterVmctlInfluxV2Migration(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	cluster := tc.MustStartDefaultCluster()
	vmAddr := fmt.Sprintf("http://%s/", cluster.Vminsert.HTTPAddr())

	baseTS := time.Now().Add(-2 * time.Hour).Truncate(time.Minute).Unix()
	points := newInfluxTestPoints(baseTS)

	influx := newInfluxMockServer(t, points)
	defer influx.close()

	vmctlFlags := []string{
		`influx`,
		`--influx-version=2`,
		`--influx-addr=` + influx.httpAddr(),
		`--influx-token=` + influxTestToken,
		`--influx-database=` + influxTestDatabase,
		`--vm-addr=` + vmAddr,
		`--vm-account-id=0`,
		`--disable-progress-bar=true`,
		`-s`,
	}

	testVmctlInfluxMigration(tc, cluster, vmctlFlags, points, baseTS)
	assertInfluxRequests(t, influx, influxV1CompatUser, influxTestToken, "")
}

func testVmctlInfluxMigration(
	tc *apptest.TestCase,
	queries apptest.PrometheusWriteQuerier,
	vmctlFlags []string,
	points []influxPoint,
	baseTS int64,
) {
	t := tc.T()
	t.Helper()

	queryStart := time.Unix(baseTS-3600, 0).UTC().Format(time.RFC3339)
	queryEnd := time.Unix(baseTS+7200, 0).UTC().Format(time.RFC3339)

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")

	// Nothing is stored before the migration runs.
	got := queries.PrometheusAPIV1Query(t, `{__name__=~".*"}`, apptest.QueryOpts{
		Step: "5m",
		Time: queryStart,
	})
	want := apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Errorf("unexpected response before migration (-want, +got):\n%s", diff)
	}

	tc.MustStartVmctl("vmctl", vmctlFlags)
	queries.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Retries: 300,
		Msg:     `unexpected metrics migrated from influx`,
		Got: func() any {
			r := queries.PrometheusAPIV1Export(t, `{__name__!=""}`, apptest.QueryOpts{
				Start: queryStart,
				End:   queryEnd,
			})
			r.Sort()
			return r.Data.Result
		},
		Want: buildExpectedInfluxResult(t, points),
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}

// buildExpectedInfluxResult derives the expected VictoriaMetrics contents from
// the same points the mock server serves: metric name is
// `<measurement>_<field>`, tags become labels, the source database is added as
// the `db` label, booleans become 1/0 and string fields are skipped.
func buildExpectedInfluxResult(t *testing.T, points []influxPoint) []*apptest.QueryResult {
	t.Helper()

	grouped := map[string]*apptest.QueryResult{}
	for _, p := range points {
		if p.FieldType == "string" {
			continue
		}
		name := fmt.Sprintf("%s_%s", p.Measurement, p.Field)
		metric := map[string]string{
			"__name__": name,
			"db":       influxTestDatabase,
		}
		for k, v := range p.Tags {
			metric[k] = v
		}
		key := tagsKey(metric)
		if _, ok := grouped[key]; !ok {
			grouped[key] = &apptest.QueryResult{Metric: metric}
		}
		grouped[key].Samples = append(grouped[key].Samples, &apptest.Sample{
			Timestamp: p.Timestamp * 1000,
			Value:     influxExpectedValue(t, p.Value),
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

// influxExpectedValue converts a field value the way vmctl does: numbers pass
// through and booleans become 1/0.
func influxExpectedValue(t *testing.T, v any) float64 {
	t.Helper()

	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case bool:
		if val {
			return 1
		}
		return 0
	default:
		t.Fatalf("unexpected field value type %T in test fixture; only float64, int64 and bool are supported", v)
		return 0
	}
}

// assertInfluxRequests verifies how vmctl authenticated to InfluxDB and which
// database and retention policy it requested.
func assertInfluxRequests(t *testing.T, s *influxMockServer, wantUser, wantPass, wantRP string) {
	t.Helper()

	reqs := s.queryRequests()
	if len(reqs) == 0 {
		t.Fatalf("vmctl issued no /query requests")
	}
	for _, r := range reqs {
		if !r.HasAuth {
			t.Fatalf("no basic auth on %s", r.Query)
		}
		if r.User != wantUser || r.Password != wantPass {
			t.Fatalf("unexpected credentials for q=%q\ngot\n(%q, %q)\nwant\n(%q, %q)",
				r.Query.Get("q"), r.User, r.Password, wantUser, wantPass)
		}
		if rp := r.Query.Get("rp"); rp != wantRP {
			t.Fatalf("unexpected rp for q=%q\ngot\n%q\nwant\n%q", r.Query.Get("q"), rp, wantRP)
		}
		if db := r.Query.Get("db"); db != influxTestDatabase {
			t.Fatalf("unexpected db for q=%q\ngot\n%q\nwant\n%q", r.Query.Get("q"), db, influxTestDatabase)
		}
	}
}

// newInfluxTestPoints returns points covering the cases that matter for the
// migration: numeric field types, a boolean, a string field which must be
// skipped, a series missing one of the measurement tags, and a measurement name
// containing special characters.
func newInfluxTestPoints(baseTS int64) []influxPoint {
	var points []influxPoint
	for i := range 10 {
		ts := baseTS + int64(i*60)

		// Two series of `cpu`: the second one has no `env` tag, which exercises
		// the empty-tag conditions vmctl adds to its SELECT statements.
		points = append(points,
			influxPoint{"cpu", map[string]string{"host": "h1", "env": "prod"}, "value", "float", ts, float64(i) + 0.5},
			influxPoint{"cpu", map[string]string{"host": "h1", "env": "prod"}, "count", "integer", ts, int64(i)},
			influxPoint{"cpu", map[string]string{"host": "h1", "env": "prod"}, "flag", "boolean", ts, i%2 == 0},
			influxPoint{"cpu", map[string]string{"host": "h1", "env": "prod"}, "note", "string", ts, "ignored"},
			influxPoint{"cpu", map[string]string{"host": "h2"}, "value", "float", ts, float64(i) - 2.25},
			influxPoint{"cpu", map[string]string{"host": "h2"}, "note", "string", ts, "ignored"},
			// Special characters in a measurement name, cf. issue #10892.
			influxPoint{"user_percent.mem.zwickel+", map[string]string{"host": "h1"}, "value", "float", ts, float64(i * 3)},
		)
	}
	return points
}
