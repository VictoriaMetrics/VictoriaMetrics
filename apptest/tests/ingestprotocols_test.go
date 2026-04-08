package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestSingleIngestionProtocols(t *testing.T) {
	fs.MustRemoveDir(t.Name())
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartDefaultVmsingle()
	type opts struct {
		query       string
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
			Msg: "unexpected /export query response",
			Got: func() any {
				got := sut.PrometheusAPIV1Export(t, opts.query, apptest.QueryOpts{
					Start: "2024-02-05T08:50:00.700Z",
					End:   "2024-02-05T09:00:00.700Z",
				})
				got.Sort()
				return got
			},
			Want: &apptest.PrometheusAPIV1QueryResponse{Data: &apptest.QueryData{Result: wantResult}},
			CmpOpts: []cmp.Option{
				cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
			},
		})
	}

	// influx line format
	sut.InfluxWrite(t, []string{
		`influxline series1=10 1707123456700`,                                        // 2024-02-05T08:57:36.700Z
		`influxline,label=foo1,label1=value1,label2=value2 series2=40 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, apptest.QueryOpts{
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 40},
		},
	})

	// OpenTSDB HTTP write format
	sut.OpenTSDBAPIPut(t, []string{
		`{"metric":"opentsdbimport.foo","value":45.34, "timestamp": "1707123457"}`,
		`{"metric":"opentsdbimport.bar","value":43, "timestamp": "1707123456"}`,
	}, apptest.QueryOpts{
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456000, Value: 43},
			{Timestamp: 1707123457000, Value: 45.34},
		},
	})

	// CSV import
	sut.PrometheusAPIV1ImportCSV(t, []string{
		`GOOG,1.23,4.56,NYSE,1707123457`,
		`MSFT,23,56,NASDAQ,1707123457`,
	}, apptest.QueryOpts{
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123457000, Value: 23},
			{Timestamp: 1707123457000, Value: 1.23},
			{Timestamp: 1707123457000, Value: 56},
			{Timestamp: 1707123457000, Value: 4.56},
		},
	})

	// prometheus text exposition format
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`# HELP importprometheus_series some help message`,
		`# TYPE importprometheus_series gauge`,
		`importprometheus_series 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`# HELP importprometheus_series2 some help message second one`,
		`# TYPE importprometheus_series2 gauge`,
		`importprometheus_series2{label="foo",label1="value1"} 20 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, apptest.QueryOpts{
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 20},
		},
	})

	// prometheus remote write format
	pbData := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{
						Name:  "__name__",
						Value: "prometheusrw_series",
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
				Samples: []prompb.Sample{
					{
						Value:     20,
						Timestamp: 1707123456800, // 2024-02-05T08:57:36.800Z
					},
				},
			},
		},
		Metadata: []prompb.MetricMetadata{
			{
				Type:             1,
				MetricFamilyName: "prometheusrw_series",
				Help:             "some help",
				Unit:             "",
			},
			{
				Type:             1,
				MetricFamilyName: "prometheusrw_series2",
				Help:             "some help2",
				Unit:             "",
			},
		},
	}
	sut.PrometheusAPIV1Write(t, pbData, apptest.QueryOpts{})
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10}, // 2024-02-05T08:57:36.700Z
			{Timestamp: 1707123456800, Value: 20}, // 2024-02-05T08:57:36.700Z
		},
	})

	// zabbixconnector format
	sut.ZabbixConnectorHistory(t,
		[]string{
			`{"host":{"host":"h1","name":"n1"},"item_tags":[], "itemid":1,"name":"zabbixconnector_series","clock":1707123456,"ns":700000000,"value":10,"type":0}`,
			`{"host":{"host":"h2","name":"n2"},"item_tags":[{"tag":"foo2","value":"value1"}], "itemid":1,"name":"zabbixconnector_series2","clock":1707123456,"ns":800000000,"value":20,"type":0}`,
		},
		apptest.QueryOpts{})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{__name__=~"zabbixconnector.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "zabbixconnector_series",
				"host":     "h1",
				"hostname": "n1",
			},
			{
				"__name__": "zabbixconnector_series2",
				"host":     "h2",
				"hostname": "n2",
				"tag_foo2": "value1",
			},
		},
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10}, // 2024-02-05T08:57:36.700Z
			{Timestamp: 1707123456800, Value: 20}, // 2024-02-05T08:57:36.700Z
		},
	})

}

func TestSingleCardinalityLimiter(t *testing.T) {
	waitFor := func(f func() bool) {
		const (
			retries = 20
			period  = 100 * time.Millisecond
		)

		t.Helper()

		for i := 0; i < retries; i++ {
			if f() {
				return
			}
			time.Sleep(period)
		}
		t.Fatalf("timed out waiting for retry #%d", retries)
	}

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	singleHourly := tc.MustStartVmsingle("vmsingle-hourly", []string{
		"-retentionPeriod=100y",
		"-storage.maxHourlySeries=1",
	})

	singleHourly.PrometheusAPIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	if v := singleHourly.GetIntMetric(t, "vm_hourly_series_limit_max_series"); v != 1 {
		t.Fatalf("unexpected vm_hourly_series_limit_max_series value: %d", v)
	}

	if v := singleHourly.GetIntMetric(t, "vm_hourly_series_limit_current_series"); v != 1 {
		t.Fatalf("unexpected vm_hourly_series_limit_current_series value: %d", v)
	}

	if v := singleHourly.GetIntMetric(t, "vm_hourly_series_limit_rows_dropped_total"); v != 0 {
		t.Fatalf("unexpected vm_hourly_series_limit_rows_dropped_total value: %d", v)
	}

	singleHourly.PrometheusAPIV1ImportPrometheus(t, []string{
		"foo_bar2 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	waitFor(
		func() bool {
			return singleHourly.GetIntMetric(t, "vm_hourly_series_limit_rows_dropped_total") > 0
		},
	)

	singleDaily := tc.MustStartVmsingle("vmsingle-daily", []string{
		"-retentionPeriod=100y",
		"-storage.maxDailySeries=1",
	})

	singleDaily.PrometheusAPIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	if v := singleDaily.GetIntMetric(t, "vm_daily_series_limit_max_series"); v != 1 {
		t.Fatalf("unexpected vm_daily_series_limit_max_series value: %d", v)
	}

	if v := singleDaily.GetIntMetric(t, "vm_daily_series_limit_current_series"); v != 1 {
		t.Fatalf("unexpected vm_daily_series_limit_current_series value: %d", v)
	}

	if v := singleDaily.GetIntMetric(t, "vm_daily_series_limit_rows_dropped_total"); v != 0 {
		t.Fatalf("unexpected vm_daily_series_limit_rows_dropped_total value: %d", v)
	}

	singleDaily.PrometheusAPIV1ImportPrometheus(t, []string{
		"foo_bar2 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	waitFor(
		func() bool {
			return singleDaily.GetIntMetric(t, "vm_daily_series_limit_rows_dropped_total") > 0
		},
	)

	singleUnlimited := tc.MustStartVmsingle("vmsingle-unlimited", []string{
		"-retentionPeriod=100y",
		"-storage.maxHourlySeries=-1",
		"-storage.maxDailySeries=-1",
	})
	metrics := make([]string, 0, 100)
	for i := range 100 {
		metrics = append(metrics, fmt.Sprintf("foo_bar%d 1 1652169600000", i)) // 2022-05-10T08:00:00Z
	}

	singleUnlimited.PrometheusAPIV1ImportPrometheus(t, metrics, apptest.QueryOpts{})

	waitFor(
		func() bool {
			return singleUnlimited.GetIntMetric(t, "vm_hourly_series_limit_current_series") > 0
		},
	)

	if v := singleUnlimited.GetIntMetric(t, "vm_hourly_series_limit_max_series"); v == 0 {
		t.Fatalf("unexpected vm_hourly_series_limit_max_series value: %d", v)
	}

	if v := singleUnlimited.GetIntMetric(t, "vm_hourly_series_limit_current_series"); v != 100 {
		t.Fatalf("unexpected vm_hourly_series_limit_current_series value: %d", v)
	}

	if v := singleUnlimited.GetIntMetric(t, "vm_hourly_series_limit_rows_dropped_total"); v != 0 {
		t.Fatalf("unexpected vm_hourly_series_limit_rows_dropped_total value: %d", v)
	}

	if v := singleUnlimited.GetIntMetric(t, "vm_daily_series_limit_max_series"); v == 0 {
		t.Fatalf("unexpected vm_daily_series_limit_max_series value: %d", v)
	}

	if v := singleUnlimited.GetIntMetric(t, "vm_daily_series_limit_current_series"); v != 100 {
		t.Fatalf("unexpected vm_daily_series_limit_current_series value: %d", v)
	}

	if v := singleUnlimited.GetIntMetric(t, "vm_daily_series_limit_rows_dropped_total"); v != 0 {
		t.Fatalf("unexpected vm_daily_series_limit_rows_dropped_total value: %d", v)
	}
}

func TestClusterIngestionProtocols(t *testing.T) {
	fs.MustRemoveDir(t.Name())
	tc := apptest.NewTestCase(t)
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
		wantSamples []*apptest.Sample
	}
	f := func(opts *opts) {
		t.Helper()
		wantResult := []*apptest.QueryResult{}
		for idx, wm := range opts.wantMetrics {
			wantResult = append(wantResult, &apptest.QueryResult{
				Metric:  wm,
				Samples: []*apptest.Sample{opts.wantSamples[idx]},
			})

		}
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected /export query response",
			Got: func() any {
				got := vmselect.PrometheusAPIV1Export(t, opts.query, apptest.QueryOpts{
					Start: "2024-02-05T08:50:00.700Z",
					End:   "2024-02-05T09:00:00.700Z",
				})
				got.Sort()
				return got
			},
			Want: &apptest.PrometheusAPIV1QueryResponse{Data: &apptest.QueryData{Result: wantResult}},
			CmpOpts: []cmp.Option{
				cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
			},
		})
	}

	// prometheus text exposition format
	vminsert.PrometheusAPIV1ImportPrometheus(t, []string{
		`# HELP importprometheus_series some help message`,
		`# TYPE importprometheus_series gauge`,
		`importprometheus_series 10 1707123456700`, // 2024-02-05T08:57:36.700Z
		`# HELP importprometheus_series2 some help message second one`,
		`# TYPE importprometheus_series2 gauge`,
		`importprometheus_series2{label="foo",label1="value1"} 20 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, apptest.QueryOpts{
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 20},
		},
	})

	// influx line format
	vminsert.InfluxWrite(t, []string{
		`influxline series1=10 1707123456700`,                                        // 2024-02-05T08:57:36.700Z
		`influxline,label=foo1,label1=value1,label2=value2 series2=40 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, apptest.QueryOpts{
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 40},
		},
	})

	// CSV import
	vminsert.PrometheusAPIV1ImportCSV(t, []string{
		`GOOG,1.23,4.56,NYSE,1707123457`, // 2024-02-05T08:57:37.000Z
		`MSFT,23,56,NASDAQ,1707123457`,   // 2024-02-05T08:57:37.000Z
	}, apptest.QueryOpts{
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
		wantSamples: []*apptest.Sample{
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
	}, apptest.QueryOpts{
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456000, Value: 43},    // 2024-02-05T08:57:36.000Z
			{Timestamp: 1707123457000, Value: 45.34}, // 2024-02-05T08:57:37.000Z
		},
	})

	// prometheus remote write format
	pbData := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{
						Name:  "__name__",
						Value: "prometheusrw_series",
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
				Samples: []prompb.Sample{
					{
						Value:     20,
						Timestamp: 1707123456800, // 2024-02-05T08:57:36.800Z
					},
				},
			},
		},
		Metadata: []prompb.MetricMetadata{
			{
				Type:             1,
				MetricFamilyName: "prometheusrw_series",
				Help:             "some help",
				Unit:             "",
			},
			{
				Type:             1,
				MetricFamilyName: "prometheusrw_series2",
				Help:             "some help2",
				Unit:             "",
			},
		},
	}
	vminsert.PrometheusAPIV1Write(t, pbData, apptest.QueryOpts{})
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
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10}, // 2024-02-05T08:57:36.700Z
			{Timestamp: 1707123456800, Value: 20}, // 2024-02-05T08:57:36.700Z
		},
	})
	// zabbixconnector format
	vminsert.ZabbixConnectorHistory(t,
		[]string{
			`{"host":{"host":"h1","name":"n1"},"item_tags":[], "itemid":1,"name":"zabbixconnector_series","clock":1707123456,"ns":700000000,"value":10,"type":0}`,
			`{"host":{"host":"h2","name":"n2"},"item_tags":[{"tag":"foo2","value":"value1"}], "itemid":1,"name":"zabbixconnector_series2","clock":1707123456,"ns":800000000,"value":20,"type":0}`,
		},
		apptest.QueryOpts{})
	vmstorage.ForceFlush(t)
	f(&opts{
		query: `{__name__=~"zabbixconnector.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "zabbixconnector_series",
				"host":     "h1",
				"hostname": "n1",
			},
			{
				"__name__": "zabbixconnector_series2",
				"host":     "h2",
				"hostname": "n2",
				"tag_foo2": "value1",
			},
		},
		wantSamples: []*apptest.Sample{
			{Timestamp: 1707123456700, Value: 10}, // 2024-02-05T08:57:36.700Z
			{Timestamp: 1707123456800, Value: 20}, // 2024-02-05T08:57:36.700Z
		},
	})

}

func TestClusterCardinalityLimiter(t *testing.T) {
	waitFor := func(f func() bool) {
		const (
			retries = 20
			period  = 100 * time.Millisecond
		)

		t.Helper()

		for i := 0; i < retries; i++ {
			if f() {
				return
			}
			time.Sleep(period)
		}
		t.Fatalf("timed out waiting for retry #%d", retries)
	}

	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	// Test hourly series limit
	vmstorageHourly := tc.MustStartVmstorage("vmstorage-hourly", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-hourly",
		"-retentionPeriod=100y",
		"-storage.maxHourlySeries=1",
	})
	vminsertHourly := tc.MustStartVminsert("vminsert-hourly", []string{
		"-storageNode=" + vmstorageHourly.VminsertAddr(),
	})

	vminsertHourly.PrometheusAPIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	if v := vmstorageHourly.GetIntMetric(t, "vm_hourly_series_limit_max_series"); v != 1 {
		t.Fatalf("unexpected vm_hourly_series_limit_max_series value: %d", v)
	}

	if v := vmstorageHourly.GetIntMetric(t, "vm_hourly_series_limit_current_series"); v != 1 {
		t.Fatalf("unexpected vm_hourly_series_limit_current_series value: %d", v)
	}

	if v := vmstorageHourly.GetIntMetric(t, "vm_hourly_series_limit_rows_dropped_total"); v != 0 {
		t.Fatalf("unexpected vm_hourly_series_limit_rows_dropped_total value: %d", v)
	}

	vminsertHourly.PrometheusAPIV1ImportPrometheus(t, []string{
		"foo_bar2 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	waitFor(
		func() bool {
			return vmstorageHourly.GetIntMetric(t, "vm_hourly_series_limit_rows_dropped_total") > 0
		},
	)

	// Test daily series limit
	vmstorageDaily := tc.MustStartVmstorage("vmstorage-daily", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-daily",
		"-retentionPeriod=100y",
		"-storage.maxDailySeries=1",
	})
	vminsertDaily := tc.MustStartVminsert("vminsert-daily", []string{
		"-storageNode=" + vmstorageDaily.VminsertAddr(),
	})

	vminsertDaily.PrometheusAPIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	if v := vmstorageDaily.GetIntMetric(t, "vm_daily_series_limit_max_series"); v != 1 {
		t.Fatalf("unexpected vm_daily_series_limit_max_series value: %d", v)
	}

	if v := vmstorageDaily.GetIntMetric(t, "vm_daily_series_limit_current_series"); v != 1 {
		t.Fatalf("unexpected vm_daily_series_limit_current_series value: %d", v)
	}

	if v := vmstorageDaily.GetIntMetric(t, "vm_daily_series_limit_rows_dropped_total"); v != 0 {
		t.Fatalf("unexpected vm_daily_series_limit_rows_dropped_total value: %d", v)
	}

	vminsertDaily.PrometheusAPIV1ImportPrometheus(t, []string{
		"foo_bar2 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	waitFor(
		func() bool {
			return vmstorageDaily.GetIntMetric(t, "vm_daily_series_limit_rows_dropped_total") > 0
		},
	)

	// Test unlimited series
	vmstorageUnlimited := tc.MustStartVmstorage("vmstorage-unlimited", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-unlimited",
		"-retentionPeriod=100y",
		"-storage.maxHourlySeries=-1",
		"-storage.maxDailySeries=-1",
	})
	vminsertUnlimited := tc.MustStartVminsert("vminsert-unlimited", []string{
		"-storageNode=" + vmstorageUnlimited.VminsertAddr(),
	})

	metrics := make([]string, 0, 100)
	for i := range 100 {
		metrics = append(metrics, fmt.Sprintf("foo_bar%d 1 1652169600000", i)) // 2022-05-10T08:00:00Z
	}

	vminsertUnlimited.PrometheusAPIV1ImportPrometheus(t, metrics, apptest.QueryOpts{})

	waitFor(
		func() bool {
			return vmstorageUnlimited.GetIntMetric(t, "vm_hourly_series_limit_current_series") > 0
		},
	)

	if v := vmstorageUnlimited.GetIntMetric(t, "vm_hourly_series_limit_max_series"); v == 0 {
		t.Fatalf("unexpected vm_hourly_series_limit_max_series value: %d", v)
	}

	if v := vmstorageUnlimited.GetIntMetric(t, "vm_hourly_series_limit_current_series"); v != 100 {
		t.Fatalf("unexpected vm_hourly_series_limit_current_series value: %d", v)
	}

	if v := vmstorageUnlimited.GetIntMetric(t, "vm_hourly_series_limit_rows_dropped_total"); v != 0 {
		t.Fatalf("unexpected vm_hourly_series_limit_rows_dropped_total value: %d", v)
	}

	if v := vmstorageUnlimited.GetIntMetric(t, "vm_daily_series_limit_max_series"); v == 0 {
		t.Fatalf("unexpected vm_daily_series_limit_max_series value: %d", v)
	}

	if v := vmstorageUnlimited.GetIntMetric(t, "vm_daily_series_limit_current_series"); v != 100 {
		t.Fatalf("unexpected vm_daily_series_limit_current_series value: %d", v)
	}

	if v := vmstorageUnlimited.GetIntMetric(t, "vm_daily_series_limit_rows_dropped_total"); v != 0 {
		t.Fatalf("unexpected vm_daily_series_limit_rows_dropped_total value: %d", v)
	}
}
