package searchutil

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestGetExtraTagFilters(t *testing.T) {
	httpReqWithForm := func(qs string) *http.Request {
		q, err := url.ParseQuery(qs)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		return &http.Request{
			Form: q,
			URL:  &url.URL{RawQuery: q.Encode()},
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
			t.Fatalf("unexpected result for GetExtraTagFilters\ngot:  %s\nwant: %s", got, want)
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
	// extra_filters and extra_filters[] are merged with "and"
	f(t, httpReqWithForm(`extra_filters={foo="bar"}&extra_filters[]={baz!~"aa",x=~"y"}`),
		[]string{`{foo="bar",baz!~"aa",x=~"y"}`},
		false,
	)
	f(t, httpReqWithForm(`extra_filters[]={foo="bar"}&extra_filters[]={baz="aa"}`),
		[]string{`{foo="bar",baz="aa"}`},
		false,
	)
	f(t, httpReqWithForm(`extra_filters={foo="bar"}&extra_filters={baz="aa"}`),
		[]string{`{foo="bar",baz="aa"}`},
		false,
	)
	// conflicting filters are merged with "and", so they select nothing instead of the union of the filters
	f(t, httpReqWithForm(`extra_filters[]={vm_account_id="7"}&extra_filters[]={vm_account_id="42"}`),
		[]string{`{vm_account_id="7",vm_account_id="42"}`},
		false,
	)
	// extra_label and extra_filters are merged with "and"
	f(t, httpReqWithForm(`extra_label=job=vmagent&extra_label=dc=gce&extra_filters={foo="bar"}`),
		[]string{`{job="vmagent",dc="gce",foo="bar"}`},
		false,
	)
	f(t, httpReqWithForm(`extra_label=job=vmagent&extra_label=dc=gce&extra_filters[]={foo="bar"}&extra_filters[]={x=~"y|z",a="b"}`),
		[]string{`{job="vmagent",dc="gce",foo="bar",x=~"y|z",a="b"}`},
		false,
	)
	// `or` filters inside a single extra_filters arg
	f(t, httpReqWithForm(`extra_filters[]={vm_account_id="7",vm_project_id="9" or vm_account_id="42"}`),
		[]string{
			`{vm_account_id="7",vm_project_id="9"}`,
			`{vm_account_id="42"}`,
		},
		false,
	)
	f(t, httpReqWithForm(`extra_label=job=vmagent&extra_filters[]={env="prod" or env="dev"}&extra_filters={team!="devops"}`),
		[]string{
			`{job="vmagent",team!="devops",env="prod"}`,
			`{job="vmagent",team!="devops",env="dev"}`,
		},
		false,
	)
	f(t, httpReqWithForm(`extra_filters={a="1" or a="2"}&extra_filters[]={b="1" or b="2"}`),
		[]string{
			`{a="1",b="1"}`,
			`{a="1",b="2"}`,
			`{a="2",b="1"}`,
			`{a="2",b="2"}`,
		},
		false,
	)
	// filter sets, which cannot match any time series, are removed
	f(t, httpReqWithForm(`extra_filters={env="prod" or env="dev"}&extra_filters={env="prod" or env="test"}`),
		[]string{`{env="prod",env="prod"}`},
		false,
	)
	f(t, httpReqWithForm(`extra_label=vm_account_id=1&extra_filters={vm_account_id="1" or vm_account_id="2"}`),
		[]string{`{vm_account_id="1",vm_account_id="1"}`},
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

	formValues, err := url.ParseQuery(`extra_label=env=prod&extra_label=job=vmsingle&extra_label=tenant=prod&extra_filters[]={foo="bar"}&extra_filters[]={tenant="prod"}`)
	if err != nil {
		t.Fatalf("BUG: cannot parse query: %s", err)
	}
	urlValues, err := url.ParseQuery(`extra_label=job=vmagent&extra_label=env=dev&extra_filters[]={tenant="dev"}`)
	if err != nil {
		t.Fatalf("BUG: cannot parse query: %s", err)
	}
	httpReqWithBothFormAndURLParams := &http.Request{
		Form: formValues,
		URL: &url.URL{
			RawQuery: urlValues.Encode(),
		},
	}
	f(t, httpReqWithBothFormAndURLParams,
		[]string{`{job="vmagent",env="dev",tenant="dev"}`},
		false)

	// empty values are ignored
	f(t, httpReqWithForm("extra_label=&extra_filters=&extra_filters[]="),
		nil,
		false,
	)
	f(t, httpReqWithForm(`extra_label=&extra_label=job=vmagent&extra_filters=&extra_filters[]={foo="bar"}`),
		[]string{`{job="vmagent",foo="bar"}`},
		false,
	)
	f(t, httpReqWithForm("extra_label=job=vmagent&extra_filters=&extra_filters[]="),
		[]string{`{job="vmagent"}`},
		false,
	)

	// empty URL query args take precedence over post form args
	httpReqWithEmptyURLParams := &http.Request{
		Form: formValues,
		URL: &url.URL{
			RawQuery: "extra_label=&extra_filters=&extra_filters[]=",
		},
	}
	f(t, httpReqWithEmptyURLParams,
		nil,
		false)
}

func TestRemoveImpossibleTagFilterss(t *testing.T) {
	f := func(s string, want []string) {
		t.Helper()
		tfss, err := ParseMetricSelector(s)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", s, err)
		}
		got := tagFilterssToStrings(removeImpossibleTagFilterss(tfss))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected result for %s\ngot:  %s\nwant: %s", s, got, want)
		}
	}

	// possible filters are kept
	f(`{a="1"}`, []string{`{a="1"}`})
	f(`{a="1",b="2"}`, []string{`{a="1",b="2"}`})
	f(`{a="1",a!="2"}`, []string{`{a="1",a!="2"}`})
	f(`{a!="1",a!="2"}`, []string{`{a!="1",a!="2"}`})

	// regexp filters aren't checked
	f(`{a="1",a=~"2"}`, []string{`{a="1",a=~"2"}`})
	f(`{a="1",a!~"1"}`, []string{`{a="1",a!~"1"}`})

	// impossible filter sets are removed
	f(`{a="1",a="2" or b="1"}`, []string{`{b="1"}`})
	f(`{a="1",a!="1" or b="1"}`, []string{`{b="1"}`})
	f(`{a!="1",a="1" or b="1"}`, []string{`{b="1"}`})
	f(`{a="1",b="1",a="2" or a="1",b="2"}`, []string{`{a="1",b="2"}`})

	// the first set is kept if all the sets are impossible, since an empty result means "match all"
	f(`{a="1",a="2" or b="1",b!="1"}`, []string{`{a="1",a="2"}`})
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
	f(` { bar !~ "^ddd(x+)$", a="ss", __name__="sffd"}  `)
	f(` { bar !~ "^ddd(x+)$", a="ss", "foo"}  `)
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
			t.Fatalf("unexpected result for JoinTagFilterss\ngot:  %s\nwant: %v", got, want)
		}
	}
	// Single tag filter
	f(t, joinTagFilters(
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
	), nil, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`,
	})
	// Multiple tag filters
	f(t, joinTagFilters(
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
		mustParseMetricSelector(`{k5=~"v5"}`),
	), nil, []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`,
		`{k5=~"v5"}`,
	})
	// Single extra filter
	f(t, nil, joinTagFilters(
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
	), []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`,
	})
	// Multiple extra filters
	f(t, nil, joinTagFilters(
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
		mustParseMetricSelector(`{k5=~"v5"}`),
	), []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`,
		`{k5=~"v5"}`,
	})
	// Single tag filter and a single extra filter
	f(t, joinTagFilters(
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
	), joinTagFilters(
		mustParseMetricSelector(`{k5=~"v5"}`),
	), []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k5=~"v5"}`,
	})
	// Multiple tag filters and a single extra filter
	f(t, joinTagFilters(
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
		mustParseMetricSelector(`{k5=~"v5"}`),
	), joinTagFilters(
		mustParseMetricSelector(`{k6=~"v6"}`),
	), []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k6=~"v6"}`,
		`{k5=~"v5",k6=~"v6"}`,
	})
	// Single tag filter and multiple extra filters
	f(t, joinTagFilters(
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
	), joinTagFilters(
		mustParseMetricSelector(`{k5=~"v5"}`),
		mustParseMetricSelector(`{k6=~"v6"}`),
	), []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k5=~"v5"}`,
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k6=~"v6"}`,
	})
	// Multiple tag filters and multiple extra filters
	f(t, joinTagFilters(
		mustParseMetricSelector(`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4"}`),
		mustParseMetricSelector(`{k5=~"v5"}`),
	), joinTagFilters(
		mustParseMetricSelector(`{k6=~"v6"}`),
		mustParseMetricSelector(`{k7=~"v7"}`),
	), []string{
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k6=~"v6"}`,
		`{k1="v1",k2=~"v2",k3!="v3",k4!~"v4",k7=~"v7"}`,
		`{k5=~"v5",k6=~"v6"}`,
		`{k5=~"v5",k7=~"v7"}`,
	})
}

func joinTagFilters(args ...[][]storage.TagFilter) [][]storage.TagFilter {
	result := append([][]storage.TagFilter{}, args[0]...)
	for _, tfss := range args[1:] {
		result = append(result, tfss...)
	}
	return result
}

func mustParseMetricSelector(s string) [][]storage.TagFilter {
	tfss, err := ParseMetricSelector(s)
	if err != nil {
		panic(fmt.Errorf("cannot parse %q: %w", s, err))
	}
	return tfss
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

func TestGetContextDeadline(t *testing.T) {
	f := func(ctx context.Context, exp deadline) {
		t.Helper()
		dv := ctx.Value(deadlineKey)
		gotDeadline, ok := dv.(deadline)
		if !ok {
			t.Fatalf("unexpected missing Deadline at context")
		}
		if gotDeadline.deadlineTimestamp() != exp.deadlineTimestamp() {
			t.Fatalf("expected deadline %d; got %d instead", exp.deadlineTimestamp(), gotDeadline.deadlineTimestamp())
		}
	}

	start := time.Now()
	expDeadline := func(d time.Duration) deadline {
		return newDeadline(start, d, "")
	}
	unwrapContext := func(ctx context.Context, _ func()) context.Context {
		return ctx
	}
	r, _ := http.NewRequest("GET", "", nil)
	f(unwrapContext(GetContextForExport(r, start)), expDeadline(*maxExportDuration))
	f(unwrapContext(GetContextForLabelsAPI(r, start)), expDeadline(*maxLabelsAPIDuration))
	f(unwrapContext(GetContextForStatusRequest(r, start)), expDeadline(*maxStatusRequestDuration))
	f(unwrapContext(GetContextForQuery(r, start)), expDeadline(*maxQueryDuration))

	r, _ = http.NewRequest("GET", "http://foo?timeout=1s", nil)
	f(unwrapContext(GetContextForExport(r, start)), expDeadline(time.Second))
	f(unwrapContext(GetContextForLabelsAPI(r, start)), expDeadline(time.Second))
	f(unwrapContext(GetContextForStatusRequest(r, start)), expDeadline(time.Second))
	f(unwrapContext(GetContextForQuery(r, start)), expDeadline(time.Second))
}
