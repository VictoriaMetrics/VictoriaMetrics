package searchutils

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestGetTimeSuccess(t *testing.T) {
	f := func(s string, timestampExpected int64) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		// Verify defaultValue
		ts, err := GetTime(r, "foo", 123456)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from GetTime(%q): %s", s, err)
		}
		if ts != 123000 {
			t.Fatalf("unexpected default value for GetTime(%q); got %d; want %d", s, ts, 123000)
		}

		// Verify timestampExpected
		ts, err = GetTime(r, "s", 123)
		if err != nil {
			t.Fatalf("unexpected error in GetTime(%q): %s", s, err)
		}
		if ts != timestampExpected {
			t.Fatalf("unexpected timestamp for GetTime(%q); got %d; want %d", s, ts, timestampExpected)
		}
	}

	f("2019-07-07T20:01:02Z", 1562529662000)
	f("2019-07-07T20:47:40+03:00", 1562521660000)
	f("-292273086-05-16T16:47:06Z", minTimeMsecs)
	f("292277025-08-18T07:12:54.999999999Z", maxTimeMsecs)
	f("1562529662.324", 1562529662324)
	f("-9223372036.854", minTimeMsecs)
	f("-9223372036.855", minTimeMsecs)
	f("9223372036.855", maxTimeMsecs)
}

func TestGetTimeError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		// Verify defaultValue
		ts, err := GetTime(r, "foo", 123456)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from GetTime(%q): %s", s, err)
		}
		if ts != 123000 {
			t.Fatalf("unexpected default value for GetTime(%q); got %d; want %d", s, ts, 123000)
		}

		// Verify timestampExpected
		_, err = GetTime(r, "s", 123)
		if err == nil {
			t.Fatalf("expecting non-nil error in GetTime(%q)", s)
		}
	}

	f("foo")
	f("2019-07-07T20:01:02Zisdf")
	f("2019-07-07T20:47:40+03:00123")
	f("-292273086-05-16T16:47:07Z")
	f("292277025-08-18T07:12:54.999999998Z")
}

func TestGetExtraTagFilters(t *testing.T) {
	httpReqWithForm := func(qs string) *http.Request {
		q, err := url.ParseQuery(qs)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		return &http.Request{
			Form: q,
		}
	}
	f := func(t *testing.T, r *http.Request, want []string, wantErr bool) {
		t.Helper()
		result, err := GetExtraTagFilters(r)
		if (err != nil) != wantErr {
			t.Fatalf("unexpected error: %v", err)
		}
		got := tagFilterssToStrings(result)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unxpected result for GetExtraTagFilters\ngot:  %s\nwant: %s", got, want)
		}
	}
	f(t, httpReqWithForm("extra_label=label=value"),
		[]string{`{label="value"}`},
		false,
	)
	f(t, httpReqWithForm("extra_label=job=vmagent&extra_label=dc=gce"),
		[]string{`{job="vmagent",dc="gce"}`},
		false,
	)
	f(t, httpReqWithForm(`extra_filters={foo="bar"}`),
		[]string{`{foo="bar"}`},
		false,
	)
	f(t, httpReqWithForm(`extra_filters={foo="bar"}&extra_filters[]={baz!~"aa",x=~"y"}`),
		[]string{
			`{foo="bar"}`,
			`{baz!~"aa",x=~"y"}`,
		},
		false,
	)
	f(t, httpReqWithForm(`extra_label=job=vmagent&extra_label=dc=gce&extra_filters={foo="bar"}`),
		[]string{`{foo="bar",job="vmagent",dc="gce"}`},
		false,
	)
	f(t, httpReqWithForm(`extra_label=job=vmagent&extra_label=dc=gce&extra_filters[]={foo="bar"}&extra_filters[]={x=~"y|z",a="b"}`),
		[]string{
			`{foo="bar",job="vmagent",dc="gce"}`,
			`{x=~"y|z",a="b",job="vmagent",dc="gce"}`,
		},
		false,
	)
	f(t, httpReqWithForm("extra_label=bad_filter"),
		nil,
		true,
	)
	f(t, httpReqWithForm(`extra_filters={bad_filter}`),
		nil,
		true,
	)
	f(t, httpReqWithForm(`extra_filters[]={bad_filter}`),
		nil,
		true,
	)
	f(t, httpReqWithForm(""),
		nil,
		false,
	)
}

func TestParseMetricSelectorSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		tfs, err := ParseMetricSelector(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if tfs == nil {
			t.Fatalf("expecting non-nil tfs when parsing %q", s)
		}
	}
	f("foo")
	f(":foo")
	f("  :fo:bar.baz")
	f(`a{}`)
	f(`{foo="bar"}`)
	f(`{:f:oo=~"bar.+"}`)
	f(`foo {bar != "baz"}`)
	f(` foo { bar !~ "^ddd(x+)$", a="ss", __name__="sffd"}  `)
	f(`(foo)`)
	f(`\п\р\и\в\е\т{\ы="111"}`)
}

func TestParseMetricSelectorError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		tfs, err := ParseMetricSelector(s)
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing %q", s)
		}
		if tfs != nil {
			t.Fatalf("expecting nil tfs when parsing %q", s)
		}
	}
	f("")
	f(`{}`)
	f(`foo bar`)
	f(`foo+bar`)
	f(`sum(bar)`)
	f(`x{y}`)
	f(`x{y+z}`)
	f(`foo[5m]`)
	f(`foo offset 5m`)
}

func TestJoinTagFilterss(t *testing.T) {
	f := func(t *testing.T, src, etfs [][]storage.TagFilter, want []string) {
		t.Helper()
		result := JoinTagFilterss(src, etfs)
		got := tagFilterssToStrings(result)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unxpected result for JoinTagFilterss\ngot:  %s\nwant: %v", got, want)
		}
	}
	// Single tag filter
	f(t, [][]storage.TagFilter{
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
	}, nil, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`,
	})
	// Miltiple tag filters
	f(t, [][]storage.TagFilter{
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
		mustParseMetricSelector(`{k5=~"v5"}`),
	}, nil, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`,
		`{k5=~"v5"}`,
	})
	// Single extra filter
	f(t, nil, [][]storage.TagFilter{
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
	}, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`,
	})
	// Multiple extra filters
	f(t, nil, [][]storage.TagFilter{
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
		mustParseMetricSelector(`{k5=~"v5"}`),
	}, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`,
		`{k5=~"v5"}`,
	})
	// Single tag filter and a single extra filter
	f(t, [][]storage.TagFilter{
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
	}, [][]storage.TagFilter{
		mustParseMetricSelector(`{k5=~"v5"}`),
	}, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k5=~"v5"}`,
	})
	// Multiple tag filters and a single extra filter
	f(t, [][]storage.TagFilter{
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
		mustParseMetricSelector(`{k5=~"v5"}`),
	}, [][]storage.TagFilter{
		mustParseMetricSelector(`{k6=~"v6"}`),
	}, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k6=~"v6"}`,
		`{k5=~"v5",k6=~"v6"}`,
	})
	// Single tag filter and multiple extra filters
	f(t, [][]storage.TagFilter{
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
	}, [][]storage.TagFilter{
		mustParseMetricSelector(`{k5=~"v5"}`),
		mustParseMetricSelector(`{k6=~"v6"}`),
	}, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k5=~"v5"}`,
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k6=~"v6"}`,
	})
	// Multiple tag filters and multiple extra filters
	f(t, [][]storage.TagFilter{
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
		mustParseMetricSelector(`{k5=~"v5"}`),
	}, [][]storage.TagFilter{
		mustParseMetricSelector(`{k6=~"v6"}`),
		mustParseMetricSelector(`{k7=~"v7"}`),
	}, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k6=~"v6"}`,
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k7=~"v7"}`,
		`{k5=~"v5",k6=~"v6"}`,
		`{k5=~"v5",k7=~"v7"}`,
	})
}

func mustParseMetricSelector(s string) []storage.TagFilter {
	tf, err := ParseMetricSelector(s)
	if err != nil {
		panic(fmt.Errorf("cannot parse %q: %w", s, err))
	}
	return tf
}

func tagFilterssToStrings(tfss [][]storage.TagFilter) []string {
	var a []string
	for _, tfs := range tfss {
		a = append(a, tagFiltersToString(tfs))
	}
	return a
}

func tagFiltersToString(tfs []storage.TagFilter) string {
	b := []byte("{")
	for i, tf := range tfs {
		b = append(b, tf.Key...)
		if tf.IsNegative {
			if tf.IsRegexp {
				b = append(b, "!~"...)
			} else {
				b = append(b, "!="...)
			}
		} else {
			if tf.IsRegexp {
				b = append(b, "=~"...)
			} else {
				b = append(b, "="...)
			}
		}
		b = strconv.AppendQuote(b, string(tf.Value))
		if i+1 < len(tfs) {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return string(b)
}
