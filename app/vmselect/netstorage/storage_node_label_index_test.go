package netstorage

import (
	"slices"
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
	f(`region=eu-1^^eu-2`)

	// empty pair
	f(`region=eu-1^^`)

	// empty label name
	f(`=eu-1`)

	// empty label value
	f(`region=`)
}

func TestStorageNodeLabelIndexMayMatchFilterss(t *testing.T) {
	f := func(index, query string, resultExpected bool) {
		t.Helper()
		snli, err := parseStorageNodeLabelIndex(index)
		if err != nil {
			t.Fatalf("cannot parse -storageNodeLabelIndex=%q: %s", index, err)
		}
		tfss, err := searchutil.ParseMetricSelector(query)
		if err != nil {
			t.Fatalf("cannot parse query %q: %s", query, err)
		}
		result := snli.mayMatchFilterss(newTagFilterMatchers(tfss))
		if result != resultExpected {
			t.Fatalf("unexpected result for -storageNodeLabelIndex=%s and query %s; got %v; want %v", index, query, result, resultExpected)
		}
	}

	// the query has no filters for the indexed label
	f(`region=eu-1`, `up{}`, true)
	f(`region=eu-1`, `up{job="foo"}`, true)

	// `=` query filter
	f(`region=eu-1`, `up{region="eu-1"}`, true)
	f(`region=eu-1`, `up{region="eu-2"}`, false)

	// `!=` query filter
	f(`region=eu-1`, `up{region!="eu-2"}`, true)
	f(`region=eu-1`, `up{region!="eu-1"}`, false)

	// `=~` query filter
	f(`region=eu-1`, `up{region=~"eu-.*"}`, true)
	f(`region=eu-1`, `up{region=~"us-.*"}`, false)
	f(`region=eu-1`, `up{region=~"eu"}`, false)

	// `!~` query filter
	f(`region=eu-1`, `up{region!~"us-.*"}`, true)
	f(`region=eu-1`, `up{region!~"eu-.*"}`, false)

	// the query filter matches only series without the indexed label
	f(`region=eu-1`, `up{region=""}`, false)

	// multiple query filters for the indexed label
	f(`region=eu-1`, `up{region=~"eu-.*",region!="eu-1"}`, false)
	f(`region=eu-1`, `up{region=~"eu-.*",region!="eu-2"}`, true)

	// multiple values for the indexed label
	f(`region=us-east^^region=us-west`, `up{region="us-west"}`, true)
	f(`region=us-east^^region=us-west`, `up{region!="us-east"}`, true)
	f(`region=us-east^^region=us-west`, `up{region!~"us-.*"}`, false)

	// query filters are checked separately, so no value has to match all of them
	f(`region=us-east^^region=eu-1`, `up{region=~"us-.*",region!="us-east"}`, true)

	// multiple indexed labels
	f(`region=us-1^^department=security`, `up{region="us-1",department="security"}`, true)
	f(`region=us-1^^department=security`, `up{department="hr"}`, false)
	f(`region=us-1^^department=security`, `up{region="us-1",department="hr"}`, false)

	// `or` in the query
	f(`region=eu-1`, `{region="us-1" or region="eu-1"}`, true)
	f(`region=eu-1`, `{region="us-1" or region="us-2"}`, false)
	f(`region=eu-1`, `{region="us-1" or job="foo"}`, true)
}

func TestGetStorageNodesForTagFilterss(t *testing.T) {
	f := func(labelIndexes []string, query string, addrsExpected []string) {
		t.Helper()
		Init([]string{"host1", "host2", "host3"}, labelIndexes)
		defer MustStop()

		tfss, err := searchutil.ParseMetricSelector(query)
		if err != nil {
			t.Fatalf("cannot parse query %q: %s", query, err)
		}
		var addrs []string
		for _, sn := range getStorageNodesForTagFilterss(nil, tfss) {
			addrs = append(addrs, sn.connPool.Addr())
		}
		if !slices.Equal(addrs, addrsExpected) {
			t.Fatalf("unexpected storage nodes for query %s; got %q; want %q", query, addrs, addrsExpected)
		}
	}

	all := []string{"host1:8401", "host2:8401", "host3:8401"}

	// no indexes
	f(nil, `up{region="eu-1"}`, all)
	f([]string{"", "", ""}, `up{region="eu-1"}`, all)

	// nodes without index are always queried
	f([]string{"region=eu-1", "", "region=us-1"}, `up{region="eu-1"}`, []string{"host1:8401", "host2:8401"})

	// some nodes are skipped
	f([]string{"region=eu-1", "region=eu-2", "region=us-1"}, `up{region=~"eu-.*"}`, []string{"host1:8401", "host2:8401"})

	// all the nodes are skipped
	f([]string{"region=eu-1", "region=eu-2", "region=us-1"}, `up{region="ap-1"}`, nil)
}

func TestGetStorageNodesForTagFilterssDiscoveredAddrs(t *testing.T) {
	f := func(addrs, addrsDiscovered []string) {
		t.Helper()
		snlis := mustParseStorageNodeLabelIndexes(addrs, []string{"region=eu-1", "region=us-1"})
		snb := initStorageNodes(addrsDiscovered, snlis)
		setStorageNodesBucket(snb)
		defer MustStop()

		tfss, err := searchutil.ParseMetricSelector(`up{region="eu-1"}`)
		if err != nil {
			t.Fatalf("cannot parse query: %s", err)
		}
		var addrsQueried []string
		for _, sn := range getStorageNodesForTagFilterss(nil, tfss) {
			addrsQueried = append(addrsQueried, sn.connPool.Addr())
		}
		if !slices.Equal(addrsQueried, []string{"host1:8401"}) {
			t.Fatalf("unexpected storage nodes for -storageNode=%q and discovered addrs %q; got %q; want [\"host1:8401\"]", addrs, addrsDiscovered, addrsQueried)
		}
	}

	// reordered addrs
	f([]string{"host1", "host2"}, []string{"host2", "host1"})

	// addrs with and without the default port
	f([]string{"host1", "host2"}, []string{"host1:8401", "host2:8401"})
	f([]string{"host1:8401", "host2:8401"}, []string{"host1", "host2"})

	// addrs with groups
	f([]string{"g1/host1", "g2/host2"}, []string{"g1/host1:8401", "g2/host2:8401"})
}
