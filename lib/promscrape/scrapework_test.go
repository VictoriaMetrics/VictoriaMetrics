package promscrape

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

func TestPromLabelsString(t *testing.T) {
	f := func(labels []prompbmarshal.Label, resultExpected string) {
		t.Helper()
		result := promLabelsString(labels)
		if result != resultExpected {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, resultExpected)
		}
	}
	f([]prompbmarshal.Label{}, "{}")
	f([]prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
	}, `{foo="bar"}`)
	f([]prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "a",
			Value: `"b"`,
		},
	}, `{foo="bar",a="\"b\""}`)
}

func TestScrapeWorkScrapeInternalFailure(t *testing.T) {
	dataExpected := `
		up 0 123
		scrape_samples_scraped 0 123
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
	sw.ReadData = func(dst []byte) ([]byte, error) {
		readDataCalls++
		return dst, fmt.Errorf("error when reading data")
	}

	pushDataCalls := 0
	var pushDataErr error
	sw.PushData = func(wr *prompbmarshal.WriteRequest) {
		if err := expectEqualTimeseries(wr.Timeseries, timeseriesExpected); err != nil {
			pushDataErr = fmt.Errorf("unexpected data pushed: %w\ngot\n%#v\nwant\n%#v", err, wr.Timeseries, timeseriesExpected)
		}
		pushDataCalls++
	}

	timestamp := int64(123000)
	if err := sw.scrapeInternal(timestamp, timestamp); err == nil {
		t.Fatalf("expecting non-nil error")
	}
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
		sw.ReadData = func(dst []byte) ([]byte, error) {
			readDataCalls++
			dst = append(dst, data...)
			return dst, nil
		}

		pushDataCalls := 0
		var pushDataErr error
		sw.PushData = func(wr *prompbmarshal.WriteRequest) {
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
		if err := sw.scrapeInternal(timestamp, timestamp); err != nil {
			if !strings.Contains(err.Error(), "sample_limit") {
				t.Fatalf("unexpected error: %s", err)
			}
		}
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
		Labels: []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "x",
			},
		},
	}, `
		foo{bar="baz",foo="x"} 34.45 3
		abc{foo="x"} -2 123
		up{foo="x"} 1 123
		scrape_samples_scraped{foo="x"} 2 123
		scrape_duration_seconds{foo="x"} 0 123
		scrape_samples_post_metric_relabeling{foo="x"} 2 123
		scrape_series_added{foo="x"} 2 123
		scrape_timeout_seconds{foo="x"} 42 123
	`)
	f(`
		foo{job="orig",bar="baz"} 34.45
		bar{y="2",job="aa",a="b",job="bb",x="1"} -3e4 2345
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   false,
		Labels: []prompbmarshal.Label{
			{
				Name:  "job",
				Value: "override",
			},
		},
	}, `
		foo{exported_job="orig",job="override",bar="baz"} 34.45 123
		bar{exported_job="aa",job="override",x="1",a="b",y="2"} -3e4 123
		up{job="override"} 1 123
		scrape_samples_scraped{job="override"} 2 123
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
		Labels: []prompbmarshal.Label{
			{
				Name:  "instance",
				Value: "foobar",
			},
			{
				Name:  "job",
				Value: "xxx",
			},
		},
	}, `
		no_instance{job="some_job",label="val1"} 5555 123
		test_with_instance{instance="some_instance",job="some_job",label="val2"} 1555 123
		up{instance="foobar",job="xxx"} 1 123
		scrape_samples_scraped{instance="foobar",job="xxx"} 2 123
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
		Labels: []prompbmarshal.Label{
			{
				Name:  "instance",
				Value: "foobar",
			},
			{
				Name:  "job",
				Value: "xxx",
			},
		},
	}, `
		no_instance{exported_job="some_job",instance="foobar",job="xxx",label="val1"} 5555 123
		test_with_instance{exported_instance="some_instance",exported_job="some_job",instance="foobar",job="xxx",label="val2"} 1555 123
		up{instance="foobar",job="xxx"} 1 123
		scrape_samples_scraped{instance="foobar",job="xxx"} 2 123
		scrape_duration_seconds{instance="foobar",job="xxx"} 0 123
		scrape_samples_post_metric_relabeling{instance="foobar",job="xxx"} 2 123
		scrape_series_added{instance="foobar",job="xxx"} 2 123
		scrape_timeout_seconds{instance="foobar",job="xxx"} 42 123
	`)
	f(`
		foo{job="orig",bar="baz"} 34.45
		bar{job="aa",a="b",job="bb"} -3e4 2345
	`, &ScrapeWork{
		ScrapeTimeout: time.Second * 42,
		HonorLabels:   true,
		Labels: []prompbmarshal.Label{
			{
				Name:  "job",
				Value: "override",
			},
		},
	}, `
		foo{job="orig",bar="baz"} 34.45 123
		bar{job="aa",a="b"} -3e4 123
		up{job="override"} 1 123
		scrape_samples_scraped{job="override"} 2 123
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
		Labels: []prompbmarshal.Label{
			{
				Name:  "job",
				Value: "xx",
			},
			{
				Name:  "__address__",
				Value: "foo.com",
			},
		},
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
		Labels: []prompbmarshal.Label{
			{
				Name:  "job",
				Value: "xx",
			},
			{
				Name:  "instance",
				Value: "foo.com",
			},
		},
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
		scrape_duration_seconds{job="xx",instance="foo.com"} 0 123
		scrape_samples_post_metric_relabeling{job="xx",instance="foo.com"} 1 123
		scrape_series_added{job="xx",instance="foo.com"} 4 123
		scrape_timeout_seconds{job="xx",instance="foo.com"} 42 123
	`)
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
		scrape_duration_seconds 0 123
		scrape_samples_post_metric_relabeling 2 123
		scrape_series_added 0 123
		scrape_timeout_seconds 42 123
	`)
}

func parseData(data string) []prompbmarshal.TimeSeries {
	var rows parser.Rows
	errLogger := func(s string) {
		panic(fmt.Errorf("unexpected error when unmarshaling Prometheus rows: %s", s))
	}
	rows.UnmarshalWithErrLogger(data, errLogger)
	var tss []prompbmarshal.TimeSeries
	for _, r := range rows.Rows {
		labels := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: r.Metric,
			},
		}
		for _, tag := range r.Tags {
			labels = append(labels, prompbmarshal.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		var ts prompbmarshal.TimeSeries
		ts.Labels = labels
		ts.Samples = []prompbmarshal.Sample{
			{
				Value:     r.Value,
				Timestamp: r.Timestamp,
			},
		}
		tss = append(tss, ts)
	}
	return tss
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
	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(config), false)
	if err != nil {
		panic(fmt.Errorf("cannot parse %q: %w", config, err))
	}
	return pcs
}
