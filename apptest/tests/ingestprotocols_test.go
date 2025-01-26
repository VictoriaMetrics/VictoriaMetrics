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
	}, at.QueryOpts{})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{__name__=~"influxline.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "influxline_series1",
			},
			{
				"__name__": "influxline_series2",
				"label":    "foo1",
				"label1":   "value1",
				"label2":   "value2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 40},
		},
	})

	// prometheus text exposition format
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`importprometheus_series 10 1707123456700`,                               // 2024-02-05T08:57:36.700Z
		`importprometheus_series2{label="foo",label1="value1"} 20 1707123456800`, // 2024-02-05T08:57:36.800Z
	}, at.QueryOpts{})
	sut.ForceFlush(t)
	f(sut, &opts{
		query: `{__name__=~"importprometheus.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "importprometheus_series",
			},
			{
				"__name__": "importprometheus_series2",
				"label":    "foo",
				"label1":   "value1",
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
	}, at.QueryOpts{})
	vmstorage.ForceFlush(t)
	f(&opts{
		query: `{__name__=~"importprometheus.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "importprometheus_series",
			},
			{
				"__name__": "importprometheus_series2",
				"label":    "foo",
				"label1":   "value1",
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
	}, at.QueryOpts{})
	vmstorage.ForceFlush(t)
	f(&opts{
		query: `{__name__=~"influxline.+"}`,
		wantMetrics: []map[string]string{
			{
				"__name__": "influxline_series1",
			},
			{
				"__name__": "influxline_series2",
				"label":    "foo1",
				"label1":   "value1",
				"label2":   "value2",
			},
		},
		wantSamples: []*at.Sample{
			{Timestamp: 1707123456700, Value: 10},
			{Timestamp: 1707123456800, Value: 40},
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
