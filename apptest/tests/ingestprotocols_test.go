package tests

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	pb "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestSingleIngestionProtocols(t *testing.T) {
	os.RemoveAll(t.Name())
	tc := at.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartDefaultVmsingle()
	type opts struct {
		query       string
		wantMetrics []map[string]string
		wantSamples []*at.Sample
	}
	f := func(sut at.PrometheusQuerier, opts *opts) {
		t.Helper()
		wantResult := []*at.QueryResult{}
		for idx, wm := range opts.wantMetrics {
			wantResult = append(wantResult, &at.QueryResult{
				Metric:  wm,
				Samples: []*at.Sample{opts.wantSamples[idx]},
			})

		}
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /export query response",
			Got: func() any {
				got := sut.PrometheusAPIV1Export(t, opts.query, at.QueryOpts{
					Start: "2024-02-05T08:50:00.700Z",
					End:   "2024-02-05T09:00:00.700Z",
				})
				got.Sort()
				return got
			},
			Want: &at.PrometheusAPIV1QueryResponse{Data: &at.QueryData{Result: wantResult}},
			CmpOpts: []cmp.Option{
				cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
			},
		})
	}

	// influx line format
	sut.InfluxWrite(t, []string{
		`influxline series1=10 1707123456700`,                                        // 2024-02-05T08:57:36.700Z
		`influxline,label=foo1,label1=value1,label2=value2 series2=40 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
	})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{__name__=~"influxline.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "influxline_series1",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "influxline_series2",
				"label":    "foo1",
				"label1":   "value1",
				"label2":   "value2",
				"el1":      "elv1",
				"el2":      "elv2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 40},
		},
	})

	// OpenTSDB HTTP write format
	sut.OpenTSDBAPIPut(t, []string{
		`{"metric":"opentsdbimport.foo","value":45.34, "timestamp": "1707123457"}`,
		`{"metric":"opentsdbimport.bar","value":43, "timestamp": "1707123456"}`,
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
	})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{__name__=~"opentsdbimport.*"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "opentsdbimport.bar",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "opentsdbimport.foo",
				"el1":      "elv1",
				"el2":      "elv2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456000, Value: 43},
			{Timestamp: 1707123457000, Value: 45.34},
		},
	})

	// CSV import
	sut.PrometheusAPIV1ImportCSV(t, []string{
		`GOOG,1.23,4.56,NYSE,1707123457`,
		`MSFT,23,56,NASDAQ,1707123457`,
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
		Format:      "2:metric:csv_import,3:metric:csv_import_v2,1:label:ticker,4:label:market,5:time:unix_s",
	})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{__name__=~"csv_import.*"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "csv_import",
				"ticker":   "MSFT",
				"market":   "NASDAQ",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "csv_import",
				"ticker":   "GOOG",
				"market":   "NYSE",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "csv_import_v2",
				"ticker":   "MSFT",
				"market":   "NASDAQ",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "csv_import_v2",
				"ticker":   "GOOG",
				"market":   "NYSE",
				"el1":      "elv1",
				"el2":      "elv2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123457000, Value: 23},
			{Timestamp: 1707123457000, Value: 1.23},
			{Timestamp: 1707123457000, Value: 56},
			{Timestamp: 1707123457000, Value: 4.56},
		},
	})

	// prometheus text exposition format
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`importprometheus_series 10 1707123456700`,                               // 2024-02-05T08:57:36.700Z
		`importprometheus_series2{label="foo",label1="value1"} 20 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
	})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{__name__=~"importprometheus.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "importprometheus_series",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "importprometheus_series2",
				"label":    "foo",
				"label1":   "value1",
				"el1":      "elv1",
				"el2":      "elv2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 20},
		},
	})

	// prometheus remote write format
	pbData := []pb.TimeSeries{
		{
			Labels: []pb.Label{
				{
					Name:  "__name__",
					Value: "prometheusrw_series",
				},
			},
			Samples: []pb.Sample{
				{
					Value:     10,
					Timestamp: 1707123456700, // 2024-02-05T08:57:36.700Z

				},
			},
		},
		{
			Labels: []pb.Label{
				{
					Name:  "__name__",
					Value: "prometheusrw_series2",
				},
				{
					Name:  "label",
					Value: "foo2",
				},
				{
					Name:  "label1",
					Value: "value1",
				},
			},
			Samples: []pb.Sample{
				{
					Value:     20,
					Timestamp: 1707123456800, // 2024-02-05T08:57:36.800Z
				},
			},
		},
	}
	sut.PrometheusAPIV1Write(t, pbData, at.QueryOpts{})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{__name__=~"prometheusrw.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "prometheusrw_series",
			},
			{
				"__name__": "prometheusrw_series2",
				"label":    "foo2",
				"label1":   "value1",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10}, // 2024-02-05T08:57:36.700Z
			{Timestamp: 1707123456800, Value: 20}, // 2024-02-05T08:57:36.700Z
		},
	})

}

func TestClusterIngestionProtocols(t *testing.T) {
	os.RemoveAll(t.Name())
	tc := at.NewTestCase(t)
	defer tc.Stop()
	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})

	type opts struct {
		query       string
		wantMetrics []map[string]string
		wantSamples []*at.Sample
	}
	f := func(opts *opts) {
		t.Helper()
		wantResult := []*at.QueryResult{}
		for idx, wm := range opts.wantMetrics {
			wantResult = append(wantResult, &at.QueryResult{
				Metric:  wm,
				Samples: []*at.Sample{opts.wantSamples[idx]},
			})

		}
		tc.Assert(&at.AssertOptions{
			Msg: "unexpected /export query response",
			Got: func() any {
				got := vmselect.PrometheusAPIV1Export(t, opts.query, at.QueryOpts{
					Start: "2024-02-05T08:50:00.700Z",
					End:   "2024-02-05T09:00:00.700Z",
				})
				got.Sort()
				return got
			},
			Want: &at.PrometheusAPIV1QueryResponse{Data: &at.QueryData{Result: wantResult}},
			CmpOpts: []cmp.Option{
				cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
			},
		})
	}

	// prometheus text exposition format
	vminsert.PrometheusAPIV1ImportPrometheus(t, []string{
		`importprometheus_series 10 1707123456700`,                               // 2024-02-05T08:57:36.700Z
		`importprometheus_series2{label="foo",label1="value1"} 20 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
	})
	vmstorage.ForceFlush(t)
	f(&opts{
		query: `{__name__=~"importprometheus.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "importprometheus_series",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "importprometheus_series2",
				"label":    "foo",
				"label1":   "value1",
				"el1":      "elv1",
				"el2":      "elv2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 20},
		},
	})

	// influx line format
	vminsert.InfluxWrite(t, []string{
		`influxline series1=10 1707123456700`,                                        // 2024-02-05T08:57:36.700Z
		`influxline,label=foo1,label1=value1,label2=value2 series2=40 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
	})
	vmstorage.ForceFlush(t)
	f(&opts{
		query: `{__name__=~"influxline.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "influxline_series1",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "influxline_series2",
				"label":    "foo1",
				"label1":   "value1",
				"label2":   "value2",
				"el1":      "elv1",
				"el2":      "elv2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 40},
		},
	})

	// CSV import
	vminsert.PrometheusAPIV1ImportCSV(t, []string{
		`GOOG,1.23,4.56,NYSE,1707123457`, // 2024-02-05T08:57:37.000Z
		`MSFT,23,56,NASDAQ,1707123457`,   // 2024-02-05T08:57:37.000Z
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
		Format:      "2:metric:csv_import,3:metric:csv_import_v2,1:label:ticker,4:label:market,5:time:unix_s",
	})
	vmstorage.ForceFlush(t)
	f(&opts{
		query: `{__name__=~"csv_import.*"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "csv_import",
				"ticker":   "MSFT",
				"market":   "NASDAQ",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "csv_import",
				"ticker":   "GOOG",
				"market":   "NYSE",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "csv_import_v2",
				"ticker":   "MSFT",
				"market":   "NASDAQ",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "csv_import_v2",
				"ticker":   "GOOG",
				"market":   "NYSE",
				"el1":      "elv1",
				"el2":      "elv2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123457000, Value: 23},   // 2024-02-05T08:57:37.000Z
			{Timestamp: 1707123457000, Value: 1.23}, // 2024-02-05T08:57:37.000Z
			{Timestamp: 1707123457000, Value: 56},   // 2024-02-05T08:57:37.000Z
			{Timestamp: 1707123457000, Value: 4.56}, // 2024-02-05T08:57:37.000Z
		},
	})

	// openTSDB HTTP write format
	vminsert.OpenTSDBAPIPut(t, []string{
		`{"metric":"opentsdbimport.foo","value":45.34, "timestamp": "1707123457"}`, // 2024-02-05T08:57:37.000Z
		`{"metric":"opentsdbimport.bar","value":43, "timestamp": "1707123456"}`,    // 2024-02-05T08:57:36.000Z
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
	})
	vmstorage.ForceFlush(t)
	f(&opts{
		query: `{__name__=~"opentsdbimport.*"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "opentsdbimport.bar",
				"el1":      "elv1",
				"el2":      "elv2",
			},
			{
				"__name__": "opentsdbimport.foo",
				"el1":      "elv1",
				"el2":      "elv2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456000, Value: 43},    // 2024-02-05T08:57:36.000Z
			{Timestamp: 1707123457000, Value: 45.34}, // 2024-02-05T08:57:37.000Z
		},
	})

	// prometheus remote write format
	pbData := []pb.TimeSeries{
		{
			Labels: []pb.Label{
				{
					Name:  "__name__",
					Value: "prometheusrw_series",
				},
			},
			Samples: []pb.Sample{
				{
					Value:     10,
					Timestamp: 1707123456700, // 2024-02-05T08:57:36.700Z

				},
			},
		},
		{
			Labels: []pb.Label{
				{
					Name:  "__name__",
					Value: "prometheusrw_series2",
				},
				{
					Name:  "label",
					Value: "foo2",
				},
				{
					Name:  "label1",
					Value: "value1",
				},
			},
			Samples: []pb.Sample{
				{
					Value:     20,
					Timestamp: 1707123456800, // 2024-02-05T08:57:36.800Z
				},
			},
		},
	}
	vminsert.PrometheusAPIV1Write(t, pbData, at.QueryOpts{})
	vmstorage.ForceFlush(t)
	f(&opts{
		query: `{__name__=~"prometheusrw.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "prometheusrw_series",
			},
			{
				"__name__": "prometheusrw_series2",
				"label":    "foo2",
				"label1":   "value1",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10}, // 2024-02-05T08:57:36.700Z
			{Timestamp: 1707123456800, Value: 20}, // 2024-02-05T08:57:36.700Z
		},
	})
}
