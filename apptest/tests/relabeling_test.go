package tests

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestSingleIngestionWithRelabeling(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	const relabelFileName = "relabel_config.yaml"
	relabelingRules := `
# add 4 labels in order to call memory allocation
- replacement: value1
  target_label: label1
- replacement: value2
  target_label: label2
- replacement: value3
  target_label: label3
- replacement: value4
  target_label: label4

# drop specific timeseries by name prefix
- action: drop
  if: '{__name__=~"^must_drop.+"}'

# strip prefix from metric name
# and write it into special label
- source_labels: [__name__]
  regex: '^(.+)_(.+)'
  replacement: $1
  target_label: ingestion_protocol
- source_labels: [__name__]
  regex: '^(.+)_(.+)'
  replacement: $2
  target_label: __name__
  `
	relabelFilePath := fmt.Sprintf("%s/%s", t.TempDir(), relabelFileName)
	fs.MustWriteSync(relabelFilePath, []byte(relabelingRules))
	sut := tc.MustStartVmsingle("relabeling-ingest",
		[]string{fmt.Sprintf(`-relabelConfig=%s`, relabelFilePath),
			`-retentionPeriod=100y`})

	type opts struct {
		query       string
		qtime       string
		step        string
		wantMetrics []map[string]string
		wantSamples []*apptest.Sample
	}
	f := func(sut apptest.PrometheusQuerier, opts *opts) {
		t.Helper()
		wantResult := []*apptest.QueryResult{}
		for idx, wm := range opts.wantMetrics {
			wantResult = append(wantResult, &apptest.QueryResult{
				Metric:  wm,
				Samples: []*apptest.Sample{opts.wantSamples[idx]},
			})

		}
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /api/v1/query response",
			Got: func() any {
				return sut.PrometheusAPIV1Query(t, opts.query, apptest.QueryOpts{
					Time: opts.qtime,
					Step: opts.step,
				})
			},
			Want: &apptest.PrometheusAPIV1QueryResponse{Data: &apptest.QueryData{Result: wantResult}},
			CmpOpts: []cmp.Option{
				cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
			},
		})
	}

	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`importprometheus_series{label="foo"} 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`must_drop_series{label="foo"} 20 1707123456800`,        // 2024-02-05T08:57:36.800Z
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{label="foo"}[120ms]`,
		qtime: "1707123456800", // 2024-02-05T08:57:36.800Z
		wantMetrics: []map[string]string{
			{
				"__name__":           "series",
				"label":              "foo",
				"label1":             "value1",
				"label2":             "value2",
				"label3":             "value3",
				"label4":             "value4",
				"ingestion_protocol": "importprometheus",
			},
		},
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10},
		},
	})

	// write influx with multi field set series1 and series2 in order to test
	// memory optimisation at vminsert side
	sut.InfluxWrite(t, []string{
		`influxline,label=foo1 series1=10,series2=30 1707123456700`, // 2024-02-05T08:57:36.700Z
		`must_drop,label=foo1 series1=20,series2=40 1707123456800`,  // 2024-02-05T08:57:36.800Z
	}, apptest.QueryOpts{})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{label="foo1"}[120ms]`,
		qtime: "1707123456800", // 2024-02-05T08:57:36.800Z
		wantMetrics: []map[string]string{
			{
				"__name__":           "series1",
				"label":              "foo1",
				"label1":             "value1",
				"label2":             "value2",
				"label3":             "value3",
				"label4":             "value4",
				"ingestion_protocol": "influxline",
			},
			{
				"__name__":           "series2",
				"label":              "foo1",
				"label1":             "value1",
				"label2":             "value2",
				"label3":             "value3",
				"label4":             "value4",
				"ingestion_protocol": "influxline"},
		},
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456700, Value: 30},
		},
	})

	pbData := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{
						Name:  "__name__",
						Value: "prometheusrw_series",
					},
					{
						Name:  "label",
						Value: "foo2",
					},
				},
				Samples: []prompb.Sample{
					{
						Value:     10,
						Timestamp: 1707123456700, // 2024-02-05T08:57:36.700Z

					},
				},
			},
			{
				Labels: []prompb.Label{
					{
						Name:  "__name__",
						Value: "must_drop_series",
					},
					{
						Name:  "label",
						Value: "foo2",
					},
				},
				Samples: []prompb.Sample{
					{
						Value:     20,
						Timestamp: 1707123456800, // 2024-02-05T08:57:36.800Z
					},
				},
			},
		},
	}
	sut.PrometheusAPIV1Write(t, pbData, apptest.QueryOpts{})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{label="foo2"}[120ms]`,
		qtime: "1707123456800", // 2024-02-05T08:57:36.800Z
		wantMetrics: []map[string]string{
			{
				"__name__":           "series",
				"label":              "foo2",
				"label1":             "value1",
				"label2":             "value2",
				"label3":             "value3",
				"label4":             "value4",
				"ingestion_protocol": "prometheusrw",
			},
		},
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10}, // 2024-02-05T08:57:36.700Z
		},
	})

}
