package promql

import (
	"os"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestMain(m *testing.M) {
	n := m.Run()
	os.Exit(n)
}

func TestMarshalTimeseriesFast(t *testing.T) {
	f := func(tss []*timeseries) {
		t.Helper()
		data := marshalTimeseriesFast(nil, tss, 1e9, 10)
		tss2, err := unmarshalTimeseriesFast(data)
		if err != nil {
			t.Fatalf("cannot unmarshal timeseries: %s", err)
		}
		if !reflect.DeepEqual(tss, tss2) {
			t.Fatalf("unexpected timeseries unmarshaled\ngot\n%#v\nwant\n%#v", tss2[0], tss[0])
		}
	}

	// Single series
	f([]*timeseries{{
		MetricName: storage.MetricName{
			MetricGroup: []byte{},
		},
		denyReuse: true,
	}})
	f([]*timeseries{{
		MetricName: storage.MetricName{
			AccountID:   8934,
			ProjectID:   8984,
			MetricGroup: []byte("foobar"),
			Tags: []storage.Tag{
				{
					Key:   []byte("tag1"),
					Value: []byte("value1"),
				},
				{
					Key:   []byte("tag2"),
					Value: []byte("value2"),
				},
			},
		},
		Values:     []float64{1, 2, 3.234},
		Timestamps: []int64{10, 20, 30},
		denyReuse:  true,
	}})

	// Multiple series
	f([]*timeseries{
		{
			MetricName: storage.MetricName{
				AccountID:   898,
				ProjectID:   9899889,
				MetricGroup: []byte("foobar"),
				Tags: []storage.Tag{
					{
						Key:   []byte("tag1"),
						Value: []byte("value1"),
					},
					{
						Key:   []byte("tag2"),
						Value: []byte("value2"),
					},
				},
			},
			Values:     []float64{1, 2.34, -33},
			Timestamps: []int64{-10, 0, 10},
			denyReuse:  true,
		},
		{
			MetricName: storage.MetricName{
				MetricGroup: []byte("baz"),
				Tags: []storage.Tag{
					{
						Key:   []byte("tag12"),
						Value: []byte("value13"),
					},
				},
			},
			Values:     []float64{4, 1, 2.34},
			Timestamps: []int64{-10, 0, 10},
			denyReuse:  true,
		},
	})
}
