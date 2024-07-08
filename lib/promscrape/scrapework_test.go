package promscrape

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

func TestIsAutoMetric(t *testing.T) {
	f := func(metric string, resultExpected bool) {
		t.Helper()
		result := isAutoMetric(metric)
		if result != resultExpected {
			t.Fatalf("unexpected result for isAutoMetric(%q); got %v; want %v", metric, result, resultExpected)
		}
	}
	f("up", true)
	f("scrape_duration_seconds", true)
	f("scrape_samples_scraped", true)
	f("scrape_samples_post_metric_relabeling", true)
	f("scrape_series_added", true)
	f("scrape_timeout_seconds", true)
	f("scrape_samples_limit", true)
	f("scrape_series_limit_samples_dropped", true)
	f("scrape_series_limit", true)
	f("scrape_series_current", true)

	f("foobar", false)
	f("exported_up", false)
	f("upx", false)
}

func TestAppendExtraLabels(t *testing.T) {
	f := func(sourceLabels, extraLabels string, honorLabels bool, resultExpected string) {
		t.Helper()
		src := promutils.MustNewLabelsFromString(sourceLabels)
		extra := promutils.MustNewLabelsFromString(extraLabels)
		var labels promutils.Labels
		labels.Labels = appendExtraLabels(src.GetLabels(), extra.GetLabels(), 0, honorLabels)
		result := labels.String()
		if result != resultExpected {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, resultExpected)
		}
	}
	f("{}", "{}", true, "{}")
	f("{}", "{}", false, "{}")
	f("foo", "{}", true, `{__name__="foo"}`)
	f("foo", "{}", false, `{__name__="foo"}`)
	f("foo", "bar", true, `{__name__="foo"}`)
	f("foo", "bar", false, `{exported___name__="foo",__name__="bar"}`)
	f(`{a="b"}`, `{c="d"}`, true, `{a="b",c="d"}`)
	f(`{a="b"}`, `{c="d"}`, false, `{a="b",c="d"}`)
	f(`{a="b"}`, `{a="d"}`, true, `{a="b"}`)
	f(`{a="b"}`, `{a="d"}`, false, `{exported_a="b",a="d"}`)
	f(`{a="b",exported_a="x"}`, `{a="d"}`, true, `{a="b",exported_a="x"}`)
	f(`{a="b",exported_a="x"}`, `{a="d"}`, false, `{exported_a="b",exported_exported_a="x",a="d"}`)
	f(`{a="b"}`, `{a="d",exported_a="x"}`, true, `{a="b",exported_a="x"}`)
	f(`{a="b"}`, `{a="d",exported_a="x"}`, false, `{exported_exported_a="b",a="d",exported_a="x"}`)
	f(`{foo="a",exported_foo="b"}`, `{exported_foo="c"}`, true, `{foo="a",exported_foo="b"}`)
	f(`{foo="a",exported_foo="b"}`, `{exported_foo="c"}`, false, `{foo="a",exported_exported_foo="b",exported_foo="c"}`)
	f(`{foo="a",exported_foo="b"}`, `{exported_foo="c",bar="x"}`, true, `{foo="a",exported_foo="b",bar="x"}`)
	f(`{foo="a",exported_foo="b"}`, `{exported_foo="c",bar="x"}`, false, `{foo="a",exported_exported_foo="b",exported_foo="c",bar="x"}`)
	f(`{foo="a",exported_foo="b"}`, `{exported_foo="c",foo="d"}`, true, `{foo="a",exported_foo="b"}`)
	f(`{foo="a",exported_foo="b"}`, `{exported_foo="c",foo="d"}`, false, `{exported_foo="a",exported_exported_foo="b",exported_foo="c",foo="d"}`)
}

func TestScrapeWorkScrapeInternalFailure(t *testing.T) {
	dataExpected := `
		up 0 123
		scrape_samples_scraped 0 123
		scrape_response_size_bytes 0 123
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 0 123
		scrape_series_added 0 123
		scrape_timeout_seconds 42 123
`
	timeseriesExpected := parseData(dataExpected)

	var sw scrapeWork
	sw.Config = &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
	}

	readDataCalls := 0
	sw.ReadData = func(_ *bytesutil.ByteBuffer) error {
		readDataCalls++
		return fmt.Errorf("error when reading data")
	}

	pushDataCalls := 0
	var pushDataErr error
	sw.PushData = func(_ *auth.Token, wr *prompbmarshal.WriteRequest) {
		if err := expectEqualTimeseries(wr.Timeseries, timeseriesExpected); err != nil {
			pushDataErr = fmt.Errorf("unexpected data pushed: %w\ngot\n%#v\nwant\n%#v", err, wr.Timeseries, timeseriesExpected)
		}
		pushDataCalls++
	}

	timestamp := int64(123000)
	tsmGlobal.Register(&sw)
	if err := sw.scrapeInternal(timestamp, timestamp); err == nil {
		t.Fatalf("expecting non-nil error")
	}
	tsmGlobal.Unregister(&sw)
	if pushDataErr != nil {
		t.Fatalf("unexpected error: %s", pushDataErr)
	}
	if readDataCalls != 1 {
		t.Fatalf("unexpected number of readData calls; got %d; want %d", readDataCalls, 1)
	}
	if pushDataCalls != 1 {
		t.Fatalf("unexpected number of pushData calls; got %d; want %d", pushDataCalls, 1)
	}
}

func TestScrapeWorkScrapeInternalSuccess(t *testing.T) {
	f := func(data string, cfg *ScrapeWork, dataExpected string) {
		t.Helper()

		timeseriesExpected := parseData(dataExpected)

		var sw scrapeWork
		sw.Config = cfg

		readDataCalls := 0
		sw.ReadData = func(dst *bytesutil.ByteBuffer) error {
			readDataCalls++
			dst.B = append(dst.B, data...)
			return nil
		}

		pushDataCalls := 0
		var pushDataErr error
		sw.PushData = func(_ *auth.Token, wr *prompbmarshal.WriteRequest) {
			pushDataCalls++
			if len(wr.Timeseries) > len(timeseriesExpected) {
				pushDataErr = fmt.Errorf("too many time series obtained; got %d; want %d\ngot\n%+v\nwant\n%+v",
					len(wr.Timeseries), len(timeseriesExpected), wr.Timeseries, timeseriesExpected)
				return
			}
			tsExpected := timeseriesExpected[:len(wr.Timeseries)]
			timeseriesExpected = timeseriesExpected[len(tsExpected):]
			if err := expectEqualTimeseries(wr.Timeseries, tsExpected); err != nil {
				pushDataErr = fmt.Errorf("unexpected data pushed: %w\ngot\n%v\nwant\n%v", err, wr.Timeseries, tsExpected)
				return
			}
		}

		timestamp := int64(123000)
		tsmGlobal.Register(&sw)
		if err := sw.scrapeInternal(timestamp, timestamp); err != nil {
			if !strings.Contains(err.Error(), "sample_limit") {
				t.Fatalf("unexpected error: %s", err)
			}
		}
		tsmGlobal.Unregister(&sw)
		if pushDataErr != nil {
			t.Fatalf("unexpected error: %s", pushDataErr)
		}
		if readDataCalls != 1 {
			t.Fatalf("unexpected number of readData calls; got %d; want %d", readDataCalls, 1)
		}
		if pushDataCalls == 0 {
			t.Fatalf("missing pushData calls")
		}
		if len(timeseriesExpected) != 0 {
			t.Fatalf("%d series weren't pushed", len(timeseriesExpected))
		}
	}

	f(``, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
	}, `
		up 1 123
		scrape_samples_scraped 0 123
		scrape_response_size_bytes 0 123
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 0 123
		scrape_series_added 0 123
		scrape_timeout_seconds 42 123
	`)
	f(`
		foo{bar="baz",empty_label=""} 34.45 3
		abc -2
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
	}, `
		foo{bar="baz"} 34.45 123
		abc -2 123
		up 1 123
		scrape_samples_scraped 2 123
		scrape_response_size_bytes 51 123
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 2 123
		scrape_series_added 2 123
		scrape_timeout_seconds 42 123
	`)
	f(`
		foo{bar="baz"} 34.45 3
		abc -2
	`, &ScrapeWork{
		ScrapeTimeout:   time.Second * 42,
		HonorTimestamps: true,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"foo": "x",
		}),
	}, `
		foo{bar="baz",foo="x"} 34.45 3
		abc{foo="x"} -2 123
		up{foo="x"} 1 123
		scrape_samples_scraped{foo="x"} 2 123
		scrape_response_size_bytes{foo="x"} 36 123
		scrape_duration_seconds{foo="x"} 0 123
		scrape_samples_post_metric_relabeling{foo="x"} 2 123
		scrape_series_added{foo="x"} 2 123
		scrape_timeout_seconds{foo="x"} 42 123
	`)
	f(`
		foo{job="orig",bar="baz"} 34.45
		bar{y="2",job="aa",a="b",x="1"} -3e4 2345
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   false,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"job": "override",
		}),
	}, `
		foo{exported_job="orig",job="override",bar="baz"} 34.45 123
		bar{exported_job="aa",job="override",x="1",a="b",y="2"} -3e4 123
		up{job="override"} 1 123
		scrape_samples_scraped{job="override"} 2 123
		scrape_response_size_bytes{job="override"} 80 123
		scrape_duration_seconds{job="override"} 0 123
		scrape_samples_post_metric_relabeling{job="override"} 2 123
		scrape_series_added{job="override"} 2 123
		scrape_timeout_seconds{job="override"} 42 123
	`)
	// Empty instance override. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/453
	f(`
		no_instance{instance="",job="some_job",label="val1",test=""} 5555
		test_with_instance{instance="some_instance",job="some_job",label="val2",test=""} 1555
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   true,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"instance": "foobar",
			"job":      "xxx",
		}),
	}, `
		no_instance{job="some_job",label="val1"} 5555 123
		test_with_instance{instance="some_instance",job="some_job",label="val2"} 1555 123
		up{instance="foobar",job="xxx"} 1 123
		scrape_samples_scraped{instance="foobar",job="xxx"} 2 123
		scrape_response_size_bytes{instance="foobar",job="xxx"} 158 123
		scrape_duration_seconds{instance="foobar",job="xxx"} 0 123
		scrape_samples_post_metric_relabeling{instance="foobar",job="xxx"} 2 123
		scrape_series_added{instance="foobar",job="xxx"} 2 123
		scrape_timeout_seconds{instance="foobar",job="xxx"} 42 123
	`)
	f(`
		no_instance{instance="",job="some_job",label="val1",test=""} 5555
		test_with_instance{instance="some_instance",job="some_job",label="val2",test=""} 1555
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   false,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"instance": "foobar",
			"job":      "xxx",
		}),
	}, `
		no_instance{exported_job="some_job",instance="foobar",job="xxx",label="val1"} 5555 123
		test_with_instance{exported_instance="some_instance",exported_job="some_job",instance="foobar",job="xxx",label="val2"} 1555 123
		up{instance="foobar",job="xxx"} 1 123
		scrape_samples_scraped{instance="foobar",job="xxx"} 2 123
		scrape_response_size_bytes{instance="foobar",job="xxx"} 158 123
		scrape_duration_seconds{instance="foobar",job="xxx"} 0 123
		scrape_samples_post_metric_relabeling{instance="foobar",job="xxx"} 2 123
		scrape_series_added{instance="foobar",job="xxx"} 2 123
		scrape_timeout_seconds{instance="foobar",job="xxx"} 42 123
	`)
	f(`
		foo{job="orig",bar="baz"} 34.45
		bar{job="aa",a="b"} -3e4 2345
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   true,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"job": "override",
		}),
	}, `
		foo{job="orig",bar="baz"} 34.45 123
		bar{job="aa",a="b"} -3e4 123
		up{job="override"} 1 123
		scrape_samples_scraped{job="override"} 2 123
		scrape_response_size_bytes{job="override"} 68 123
		scrape_duration_seconds{job="override"} 0 123
		scrape_samples_post_metric_relabeling{job="override"} 2 123
		scrape_series_added{job="override"} 2 123
		scrape_timeout_seconds{job="override"} 42 123
	`)
	f(`
		foo{bar="baz"} 34.44
		bar{a="b",c="d"} -3e4
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   true,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"job":         "xx",
			"__address__": "foo.com",
		}),
		MetricRelabelConfigs: mustParseRelabelConfigs(`
- action: replace
  source_labels: ["__address__", "job"]
  separator: "/"
  target_label: "instance"
- action: labeldrop
  regex: c
`),
	}, `
		foo{bar="baz",job="xx",instance="foo.com/xx"} 34.44 123
		bar{a="b",job="xx",instance="foo.com/xx"} -3e4 123
		up{job="xx"} 1 123
		scrape_samples_scraped{job="xx"} 2 123
		scrape_response_size_bytes{job="xx"} 49 123
		scrape_duration_seconds{job="xx"} 0 123
		scrape_samples_post_metric_relabeling{job="xx"} 2 123
		scrape_series_added{job="xx"} 2 123
		scrape_timeout_seconds{job="xx"} 42 123
	`)
	f(`
		foo{bar="baz"} 34.44
		bar{a="b",c="d",} -3e4
		dropme{foo="bar"} 334
		dropme{xxx="yy",ss="dsf"} 843
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   true,
		Labels: promutils.NewLabelsFromMap(map[string]string{
			"job":      "xx",
			"instance": "foo.com",
		}),
		MetricRelabelConfigs: mustParseRelabelConfigs(`
- action: drop
  separator: ""
  source_labels: [a, c]
  regex: "^bd$"
- action: drop
  source_labels: [__name__]
  regex: "dropme|up"
`),
	}, `
		foo{bar="baz",job="xx",instance="foo.com"} 34.44 123
		up{job="xx",instance="foo.com"} 1 123
		scrape_samples_scraped{job="xx",instance="foo.com"} 4 123
		scrape_response_size_bytes{job="xx",instance="foo.com"} 106 123
		scrape_duration_seconds{job="xx",instance="foo.com"} 0 123
		scrape_samples_post_metric_relabeling{job="xx",instance="foo.com"} 1 123
		scrape_series_added{job="xx",instance="foo.com"} 4 123
		scrape_timeout_seconds{job="xx",instance="foo.com"} 42 123
	`)
	// Scrape metrics with names clashing with auto metrics
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3406
	f(`
		up{bar="baz"} 34.44
		bar{a="b",c="d"} -3e4
		scrape_series_added 3.435
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
	}, `
		up{bar="baz"} 34.44 123
		bar{a="b",c="d"} -3e4 123
		exported_scrape_series_added 3.435 123
		up 1 123
		scrape_duration_seconds 0 123
		scrape_response_size_bytes 76 123
		scrape_samples_scraped 3 123
		scrape_samples_post_metric_relabeling 3 123
		scrape_timeout_seconds 42 123
		scrape_series_added 3 123
	`)
	f(`
		up{bar="baz"} 34.44
		bar{a="b",c="d"} -3e4
		scrape_series_added 3.435
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   true,
	}, `
		up{bar="baz"} 34.44 123
		bar{a="b",c="d"} -3e4 123
		scrape_series_added 3.435 123
		up 1 123
		scrape_samples_scraped 3 123
		scrape_response_size_bytes 76 123
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 3 123
		scrape_series_added 3 123
		scrape_timeout_seconds 42 123
	`)
	// Scrape success with the given SampleLimit.
	f(`
		foo{bar="baz"} 34.44
		bar{a="b",c="d"} -3e4
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		SampleLimit:   2,
	}, `
		foo{bar="baz"} 34.44 123
		bar{a="b",c="d"} -3e4 123
		up 1 123
		scrape_samples_limit 2 123
		scrape_response_size_bytes 49 123
		scrape_samples_scraped 2 123
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 2 123
		scrape_series_added 2 123
		scrape_timeout_seconds 42 123
	`)
	// Scrape failure because of the exceeded SampleLimit
	f(`
		foo{bar="baz"} 34.44
		bar{a="b",c="d"} -3e4
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   true,
		SampleLimit:   1,
		SeriesLimit:   123,
	}, `
		up 0 123
		scrape_samples_scraped 2 123
		scrape_response_size_bytes 0 123
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 2 123
		scrape_samples_limit 1 123
		scrape_series_added 0 123
		scrape_series_current 0 123
		scrape_series_limit 123 123
		scrape_series_limit_samples_dropped 0 123
		scrape_timeout_seconds 42 123
	`)
	// Scrape success with the given SeriesLimit.
	f(`
		foo{bar="baz"} 34.44
		bar{a="b",c="d"} -3e4
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		SeriesLimit:   123,
	}, `
		foo{bar="baz"} 34.44 123
		bar{a="b",c="d"} -3e4 123
		up 1 123
		scrape_samples_scraped 2 123
		scrape_response_size_bytes 49 123
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 2 123
		scrape_series_added 2 123
		scrape_series_current 2 123
		scrape_series_limit 123 123
		scrape_series_limit_samples_dropped 0 123
		scrape_timeout_seconds 42 123
	`)
	// Exceed SeriesLimit.
	f(`
		foo{bar="baz"} 34.44
		bar{a="b",c="d"} -3e4
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		SeriesLimit:   1,
	}, `
		foo{bar="baz"} 34.44 123
		up 1 123
		scrape_samples_scraped 2 123
		scrape_response_size_bytes 49 123
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 2 123
		scrape_series_added 2 123
		scrape_series_current 1 123
		scrape_series_limit 1 123
		scrape_series_limit_samples_dropped 1 123
		scrape_timeout_seconds 42 123
	`)
}

func TestAddRowToTimeseriesNoRelabeling(t *testing.T) {
	f := func(row string, cfg *ScrapeWork, dataExpected string) {
		t.Helper()
		sw := scrapeWork{
			Config: cfg,
		}
		var wc writeRequestCtx
		r := parsePromRow(row)
		sw.addRowToTimeseries(&wc, r, r.Timestamp, false)
		tss := wc.writeRequest.Timeseries
		tssExpected := parseData(dataExpected)
		if err := expectEqualTimeseries(tss, tssExpected); err != nil {
			t.Fatalf("%s\ngot\n%v\nwant\n%v", err, tss, tssExpected)
		}
	}

	// HonorLabels=false, empty Labels and ExternalLabels
	f(`metric 0 123`,
		&ScrapeWork{
			HonorLabels: false,
		},
		`metric 0 123`)
	f(`metric{a="f"} 0 123`,
		&ScrapeWork{
			HonorLabels: false,
		},
		`metric{a="f"} 0 123`)
	// HonorLabels=true, empty Labels and ExternalLabels
	f(`metric 0 123`,
		&ScrapeWork{
			HonorLabels: true,
		},
		`metric 0 123`)
	f(`metric{a="f"} 0 123`,
		&ScrapeWork{
			HonorLabels: true,
		},
		`metric{a="f"} 0 123`)
	// HonorLabels=false, non-empty Labels
	f(`metric 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f"} 0 123`)
	f(`metric{foo="bar"} 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f",foo="bar"} 0 123`)
	// HonorLabels=true, non-empty Labels
	f(`metric 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="f"} 0 123`)
	f(`metric{foo="bar"} 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="f",foo="bar"} 0 123`)
	// HonorLabels=false, non-empty ExternalLabels
	f(`metric 0 123`,
		&ScrapeWork{
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f"} 0 123`)
	f(`metric{foo="bar"} 0 123`,
		&ScrapeWork{
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f",foo="bar"} 0 123`)
	// HonorLabels=true, non-empty ExternalLabels
	f(`metric 0 123`,
		&ScrapeWork{
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="f"} 0 123`)
	f(`metric{foo="bar"} 0 123`,
		&ScrapeWork{
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="f",foo="bar"} 0 123`)
	// HonorLabels=false, non-empty Labels and ExternalLabels
	f(`metric 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"x": "y",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f",x="y"} 0 123`)
	f(`metric{foo="bar"} 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"x": "y",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f",foo="bar",x="y"} 0 123`)
	// HonorLabels=true, non-empty Labels and ExternalLabels
	f(`metric 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"x": "y",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="f",x="y"} 0 123`)
	f(`metric{foo="bar"} 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"x": "y",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="f",foo="bar",x="y"} 0 123`)
	// HonorLabels=false, clashing Labels and metric label
	f(`metric{a="b"} 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f",exported_a="b"} 0 123`)
	// HonorLabels=true, clashing Labels and metric label
	f(`metric{a="b"} 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="b"} 0 123`)
	// HonorLabels=false, clashing ExternalLabels and metric label
	f(`metric{a="b"} 0 123`,
		&ScrapeWork{
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f",exported_a="b"} 0 123`)
	// HonorLabels=true, clashing ExternalLabels and metric label
	f(`metric{a="b"} 0 123`,
		&ScrapeWork{
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="b"} 0 123`)
	// HonorLabels=false, clashing Labels and ExternalLAbels
	f(`metric 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "e",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f",exported_a="e"} 0 123`)
	f(`metric{foo="bar"} 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "e",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: false,
		},
		`metric{a="f",foo="bar",exported_a="e"} 0 123`)
	// HonorLabels=true, clashing Labels and ExternalLAbels
	f(`metric 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "e",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="e"} 0 123`)
	f(`metric{foo="bar"} 0 123`,
		&ScrapeWork{
			Labels: promutils.NewLabelsFromMap(map[string]string{
				"a": "e",
			}),
			ExternalLabels: promutils.NewLabelsFromMap(map[string]string{
				"a": "f",
			}),
			HonorLabels: true,
		},
		`metric{a="e",foo="bar"} 0 123`)
}

func TestSendStaleSeries(t *testing.T) {
	f := func(lastScrape, currScrape string, staleMarksExpected int) {
		t.Helper()
		var sw scrapeWork
		sw.Config = &ScrapeWork{
			NoStaleMarkers: false,
		}
		common.StartUnmarshalWorkers()
		defer common.StopUnmarshalWorkers()

		var staleMarks int
		sw.PushData = func(_ *auth.Token, wr *prompbmarshal.WriteRequest) {
			staleMarks += len(wr.Timeseries)
		}
		sw.sendStaleSeries(lastScrape, currScrape, 0, false)
		if staleMarks != staleMarksExpected {
			t.Fatalf("unexpected number of stale marks; got %d; want %d", staleMarks, staleMarksExpected)
		}
	}
	generateScrape := func(n int) string {
		w := strings.Builder{}
		for i := 0; i < n; i++ {
			w.WriteString(fmt.Sprintf("foo_%d 1\n", i))
		}
		return w.String()
	}

	f("", "", 0)
	f(generateScrape(10), generateScrape(10), 0)
	f(generateScrape(10), "", 10)
	f("", generateScrape(10), 0)
	f(generateScrape(10), generateScrape(3), 7)
	f(generateScrape(3), generateScrape(10), 0)
	f(generateScrape(20000), generateScrape(10), 19990)
}

func parsePromRow(data string) *parser.Row {
	var rows parser.Rows
	errLogger := func(s string) {
		panic(fmt.Errorf("unexpected error when unmarshaling Prometheus rows: %s", s))
	}
	rows.UnmarshalWithErrLogger(data, errLogger)
	if len(rows.Rows) != 1 {
		panic(fmt.Errorf("unexpected number of rows parsed from %q; got %d; want %d", data, len(rows.Rows), 1))
	}
	return &rows.Rows[0]
}

func parseData(data string) []prompbmarshal.TimeSeries {
	return prompbmarshal.MustParsePromMetrics(data, 0)
}

func expectEqualTimeseries(tss, tssExpected []prompbmarshal.TimeSeries) error {
	m, err := timeseriesToMap(tss)
	if err != nil {
		return fmt.Errorf("invalid generated timeseries: %w", err)
	}
	mExpected, err := timeseriesToMap(tssExpected)
	if err != nil {
		return fmt.Errorf("invalid expected timeseries: %w", err)
	}
	if len(m) != len(mExpected) {
		return fmt.Errorf("unexpected time series len; got %d; want %d", len(m), len(mExpected))
	}
	for k, tsExpected := range mExpected {
		ts := m[k]
		if ts != tsExpected {
			return fmt.Errorf("unexpected timeseries %q;\ngot\n%s\nwant\n%s", k, ts, tsExpected)
		}
	}
	return nil
}

func timeseriesToMap(tss []prompbmarshal.TimeSeries) (map[string]string, error) {
	m := make(map[string]string, len(tss))
	for i := range tss {
		ts := &tss[i]
		if len(ts.Labels) == 0 {
			return nil, fmt.Errorf("unexpected empty labels for timeseries #%d; timeseries: %#v", i, ts)
		}
		if len(ts.Samples) != 1 {
			return nil, fmt.Errorf("unexpected number of samples for timeseries #%d; got %d; want %d", i, len(ts.Samples), 1)
		}
		if ts.Labels[0].Name != "__name__" {
			return nil, fmt.Errorf("unexpected first name for timeseries #%d; got %q; want %q", i, ts.Labels[0].Name, "__name__")
		}
		if ts.Labels[0].Value == "scrape_duration_seconds" {
			// Reset scrape_duration_seconds value to 0, since it is non-deterministic
			ts.Samples[0].Value = 0
		}
		m[ts.Labels[0].Value] = timeseriesToString(ts)
	}
	return m, nil
}

func timeseriesToString(ts *prompbmarshal.TimeSeries) string {
	promrelabel.SortLabels(ts.Labels)
	var sb strings.Builder
	fmt.Fprintf(&sb, "{")
	for i, label := range ts.Labels {
		fmt.Fprintf(&sb, "%s=%q", label.Name, label.Value)
		if i+1 < len(ts.Labels) {
			fmt.Fprintf(&sb, ",")
		}
	}
	fmt.Fprintf(&sb, "} ")
	if len(ts.Samples) != 1 {
		panic(fmt.Errorf("expecting a single sample; got %d samples", len(ts.Samples)))
	}
	s := ts.Samples[0]
	fmt.Fprintf(&sb, "%g %d", s.Value, s.Timestamp)
	return sb.String()
}

func mustParseRelabelConfigs(config string) *promrelabel.ParsedConfigs {
	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(config))
	if err != nil {
		panic(fmt.Errorf("cannot parse %q: %w", config, err))
	}
	return pcs
}
