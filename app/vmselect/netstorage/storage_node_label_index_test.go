package netstorage

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
)

func TestParseStorageNodeLabelIndexFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if _, err := parseStorageNodeLabelIndex(s); err == nil {
			t.Fatalf("expecting non-nil error for -storageNodeLabelIndex=%q", s)
		}
	}

	// missing `=`
	f(`region`)
	f(`region=eu-1^^eu-2`)

	// empty label name
	f(`=eu-1`)

	// empty label value
	f(`region=`)
	f(`region=eu-1^^`)
	f(`^^region=eu-1`)

	// duplicate label pair
	f(`region=eu-1^^region=eu-1`)
}

func TestStorageNodeLabelIndexMayMatch(t *testing.T) {
	f := func(index, query string, resultExpected bool) {
		t.Helper()
		li, err := parseStorageNodeLabelIndex(index)
		if err != nil {
			t.Fatalf("cannot parse -storageNodeLabelIndex=%q: %s", index, err)
		}
		tfss, err := searchutil.ParseMetricSelector(query)
		if err != nil {
			t.Fatalf("cannot parse query %q: %s", query, err)
		}
		result := li.mayMatch(newQueryFilters(tfss))
		if result != resultExpected {
			t.Fatalf("unexpected result for -storageNodeLabelIndex=%s and query %s; got %v; want %v", index, query, result, resultExpected)
		}
	}

	// the query has no filters for the indexed label
	f(`region=eu-1`, `up`, true)
	f(`region=eu-1`, `up{job="foo"}`, true)

	// `=` query filter
	f(`region=eu-1`, `up{region="eu-1"}`, true)
	f(`region=eu-1`, `up{region="eu-2"}`, false)
	f(`region=eu-1`, `up{region=""}`, false)

	// `!=` query filter
	f(`region=eu-1`, `up{region!="eu-2"}`, true)
	f(`region=eu-1`, `up{region!="eu-1"}`, false)

	// `=~` query filter
	f(`region=eu-1`, `up{region=~"eu-.*"}`, true)
	f(`region=eu-1`, `up{region=~"eu-1|us-1"}`, true)
	f(`region=eu-1`, `up{region=~"us-.*"}`, false)
	f(`region=eu-1`, `up{region=~"eu"}`, false)

	// `!~` query filter
	f(`region=eu-1`, `up{region!~"us-.*"}`, true)
	f(`region=eu-1`, `up{region!~"eu-.*"}`, false)

	// multiple query filters for the indexed label
	f(`region=eu-1`, `up{region=~"eu-.*",region!="eu-1"}`, false)
	f(`region=eu-1`, `up{region=~"eu-.*",region!="eu-2"}`, true)

	// metric name
	f(`__name__=up`, `up`, true)
	f(`__name__=up`, `down`, false)

	// multiple values for the indexed label
	f(`region=us-east^^region=us-west`, `up{region="us-west"}`, true)
	f(`region=us-east^^region=us-west`, `up{region=~"eu-.*"}`, false)
	f(`region=us-east^^region=us-west`, `up{region!~"us-.*"}`, false)
	f(`region=us-east^^region=us-west`, `up{region!="us-east"}`, true)
	f(`region=us-east^^region=us-west`, `up{region=~"us-.*",region!="us-east"}`, true)
	f(`region=us-east^^region=us-west`, `up{region=~"us-.*",region!~"us-(east|west)"}`, false)

	// multiple indexed labels
	f(`region=us-1^^department=security`, `up`, true)
	f(`region=us-1^^department=security`, `up{region="us-1"}`, true)
	f(`region=us-1^^department=security`, `up{department="security"}`, true)
	f(`region=us-1^^department=security`, `up{region="us-1",department="security"}`, true)
	f(`region=us-1^^department=security`, `up{region="eu-1"}`, false)
	f(`region=us-1^^department=security`, `up{department="hr"}`, false)
	f(`region=us-1^^department=security`, `up{region="us-1",department="hr"}`, false)
	f(`region=us-1^^region=us-2^^department=security`, `up{region="us-2",department=~"sec.*"}`, true)

	// `or` at the query
	f(`region=eu-1`, `{region="us-1" or region="eu-1"}`, true)
	f(`region=eu-1`, `{region="us-1" or region="us-2"}`, false)
	f(`region=eu-1`, `{region="us-1" or job="foo"}`, true)
	f(`region=us-1^^department=security`, `{region="eu-1" or department="hr"}`, false)
	f(`region=us-1^^department=security`, `{region="eu-1" or department="security"}`, true)
}
