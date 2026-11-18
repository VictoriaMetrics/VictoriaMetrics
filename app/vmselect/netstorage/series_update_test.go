package netstorage

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestSeriesUpdateContext(t *testing.T) {
	newMetricName := func(metricGroup string, tags ...string) storage.MetricName {
		mn := storage.MetricName{
			MetricGroup: []byte(metricGroup),
		}
		for i := 0; i < len(tags); i += 2 {
			mn.AddTag(tags[i], tags[i+1])
		}
		return mn
	}

	type update struct {
		value int
	}

	// Every series gets blockRef with addr.offset equal to its index at series.
	// expected contains addr.offsets of updates returned for each lookup.
	f := func(series []storage.MetricName, isUpdateExpected []bool, lookup []storage.MetricName, expected [][][]int) {
		t.Helper()
		suc := newSeriesUpdateContext[update]("rev", len(series))
		for i := range series {
			isUpdate, err := suc.tryAddUpdate(series[i].Marshal(nil), update{i})
			if err != nil {
				t.Fatalf("unexpected error at series #%d: %s", i, err)
			}
			if isUpdate != isUpdateExpected[i] {
				t.Fatalf("unexpected isUpdate at series #%d; got %v; want %v", i, isUpdate, isUpdateExpected[i])
			}
		}
		for i := range lookup {
			var got [][]int
			for _, updates := range suc.getUpdatesByMetricName(string(lookup[i].Marshal(nil))) {
				values := make([]int, 0, len(updates))
				for _, update := range updates {
					values = append(values, update.value)
				}
				got = append(got, values)
			}
			if !reflect.DeepEqual(got, expected[i]) {
				t.Fatalf("unexpected updates for lookup %s;\ngot\n%v\nwant\n%v", lookup[i].String(), got, expected[i])
			}
		}
	}

	// no series
	f(nil, nil, []storage.MetricName{
		newMetricName("foo"),
	}, [][][]int{nil})

	// series without revision label
	f([]storage.MetricName{
		newMetricName("foo", "job", "a"),
		newMetricName("bar"),
	}, []bool{false, false}, []storage.MetricName{
		newMetricName("foo", "job", "a"),
	}, [][][]int{nil})

	// single update
	f([]storage.MetricName{
		newMetricName("foo", "job", "a"),
		newMetricName("foo", "job", "a", "rev", "1"),
	}, []bool{false, true}, []storage.MetricName{
		newMetricName("foo", "job", "a"),
	}, [][][]int{{{1}}})

	// updates are sorted by revision and updates with the same revision are grouped
	f([]storage.MetricName{
		newMetricName("foo", "rev", "10"),
		newMetricName("foo", "rev", "10"),
		newMetricName("foo", "rev", "2"),
		newMetricName("foo"),
		newMetricName("foo", "rev", "-5"),
		newMetricName("foo", "rev", "2"),
	}, []bool{true, true, true, false, true, true}, []storage.MetricName{
		newMetricName("foo"),
	}, [][][]int{{{4}, {2, 5}, {0, 1}}})

	// updates are returned per series
	f([]storage.MetricName{
		newMetricName("foo", "rev", "1"),
		newMetricName("bar", "rev", "1"),
		newMetricName("foo", "job", "b", "rev", "2"),
		newMetricName("foo", "rev", "0"),
	}, []bool{true, true, true, true}, []storage.MetricName{
		newMetricName("bar"),
		newMetricName("foo"),
		newMetricName("foo", "job", "b"),
	}, [][][]int{{{1}}, {{3}, {0}}, {{2}}})

	// lookup of unknown series
	f([]storage.MetricName{
		newMetricName("foo", "rev", "1"),
	}, []bool{true}, []storage.MetricName{
		newMetricName("bar"),
	}, [][][]int{nil})
}
