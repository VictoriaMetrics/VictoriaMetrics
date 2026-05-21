package prometheus

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestFederate(t *testing.T) {
	f := func(rs *netstorage.Result, expectedResult string, scheme EscapingScheme) {
		t.Helper()
		result := Federate(rs, scheme)
		if result != expectedResult {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, expectedResult)
		}
	}

	f(&netstorage.Result{}, ``, UnderscoreEscaping)

	f(&netstorage.Result{
		MetricName: storage.MetricName{
			MetricGroup: []byte("foo"),
			Tags: []storage.Tag{
				{
					Key:   []byte("a"),
					Value: []byte("b"),
				},
				{
					Key:   []byte("qqq"),
					Value: []byte("\\"),
				},
				{
					Key: []byte("abc"),
					// Verify that < isn't encoded. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5431
					Value: []byte("a<b\"\\c"),
				},
			},
		},
		Values:     []float64{1.23},
		Timestamps: []int64{123},
	}, `foo{a="b",qqq="\\",abc="a<b\"\\c"} 1.23 123`+"\n", UnderscoreEscaping)

	f(&netstorage.Result{
		MetricName: storage.MetricName{
			MetricGroup: []byte("f.o.o"),
			Tags: []storage.Tag{
				{
					Key:   []byte("a"),
					Value: []byte("b"),
				},
				{
					Key:   []byte("q.q.q"),
					Value: []byte("\\"),
				},
				{
					Key: []byte("abc"),
					// Verify that < isn't encoded. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5431
					Value: []byte("a<b\"\\c"),
				},
			},
		},
		Values:     []float64{1.23},
		Timestamps: []int64{123},
	}, `f_o_o{a="b",q_q_q="\\",abc="a<b\"\\c"} 1.23 123`+"\n", UnderscoreEscaping)

	f(&netstorage.Result{
		MetricName: storage.MetricName{
			MetricGroup: []byte("f.o.o"),
			Tags: []storage.Tag{
				{
					Key:   []byte("a"),
					Value: []byte("b"),
				},
				{
					Key:   []byte("q.q.q"),
					Value: []byte("\\"),
				},
				{
					Key: []byte("abc"),
					// Verify that < isn't encoded. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5431
					Value: []byte("a<b\"\\c"),
				},
			},
		},
		Values:     []float64{1.23},
		Timestamps: []int64{123},
	}, `{"f.o.o",a="b","q.q.q"="\\",abc="a<b\"\\c"} 1.23 123`+"\n", NoEscaping)

	f(&netstorage.Result{
		MetricName: storage.MetricName{
			MetricGroup: []byte("f.ö.o"),
			Tags: []storage.Tag{
				{
					Key:   []byte("a"),
					Value: []byte("b"),
				},
				{
					Key:   []byte("q.ö.q"),
					Value: []byte("\\"),
				},
				{
					Key: []byte("abc"),
					// Verify that < isn't encoded. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5431
					Value: []byte("a<b\"\\c"),
				},
			},
		},
		Values:     []float64{1.23},
		Timestamps: []int64{123},
	}, `{"f.ö.o",a="b","q.ö.q"="\\",abc="a<b\"\\c"} 1.23 123`+"\n", NoEscaping)
}
