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

	// Every series gets blockRef with addr.offset equal to its index at series.
	// expected contains addr.offsets of patches returned for each lookup.
	f := func(series []storage.MetricName, isPatchExpected []bool, lookup []storage.MetricName, expected [][][]uint64) {
		t.Helper()
		suc := newSeriesUpdateContext("rev", len(series))
		for i := range series {
			isPatch, err := suc.tryAddUpdate(series[i].Marshal(nil), blockRef{
				addr: tmpBlockAddr{offset: uint64(i)},
			})
			if err != nil {
				t.Fatalf("unexpected error at series #%d: %s", i, err)
			}
			if isPatch != isPatchExpected[i] {
				t.Fatalf("unexpected isPatch at series #%d; got %v; want %v", i, isPatch, isPatchExpected[i])
			}
		}
		for i := range lookup {
			var got [][]uint64
			for _, brs := range suc.getUpdatesByMetricName(string(lookup[i].Marshal(nil))) {
				offsets := make([]uint64, 0, len(brs))
				for _, br := range brs {
					offsets = append(offsets, br.addr.offset)
				}
				got = append(got, offsets)
			}
			if !reflect.DeepEqual(got, expected[i]) {
				t.Fatalf("unexpected patches for lookup %s;\ngot\n%v\nwant\n%v", lookup[i].String(), got, expected[i])
			}
		}
	}

	// no series
	f(nil, nil, []storage.MetricName{
		newMetricName("foo"),
	}, [][][]uint64{nil})

	// series without revision label
	f([]storage.MetricName{
		newMetricName("foo", "job", "a"),
		newMetricName("bar"),
	}, []bool{false, false}, []storage.MetricName{
		newMetricName("foo", "job", "a"),
	}, [][][]uint64{nil})

	// single patch
	f([]storage.MetricName{
		newMetricName("foo", "job", "a"),
		newMetricName("foo", "job", "a", "rev", "1"),
	}, []bool{false, true}, []storage.MetricName{
		newMetricName("foo", "job", "a"),
	}, [][][]uint64{{{1}}})

	// patches are sorted by revision and blocks with the same revision are grouped
	f([]storage.MetricName{
		newMetricName("foo", "rev", "10"),
		newMetricName("foo", "rev", "10"),
		newMetricName("foo", "rev", "2"),
		newMetricName("foo"),
		newMetricName("foo", "rev", "-5"),
		newMetricName("foo", "rev", "2"),
	}, []bool{true, true, true, false, true, true}, []storage.MetricName{
		newMetricName("foo"),
	}, [][][]uint64{{{4}, {2, 5}, {0, 1}}})

	// patches are returned per series
	f([]storage.MetricName{
		newMetricName("foo", "rev", "1"),
		newMetricName("bar", "rev", "1"),
		newMetricName("foo", "job", "b", "rev", "2"),
		newMetricName("foo", "rev", "0"),
	}, []bool{true, true, true, true}, []storage.MetricName{
		newMetricName("bar"),
		newMetricName("foo"),
		newMetricName("foo", "job", "b"),
	}, [][][]uint64{{{1}}, {{3}, {0}}, {{2}}})

	// lookup of unknown series
	f([]storage.MetricName{
		newMetricName("foo", "rev", "1"),
	}, []bool{true}, []storage.MetricName{
		newMetricName("bar"),
	}, [][][]uint64{nil})
}
