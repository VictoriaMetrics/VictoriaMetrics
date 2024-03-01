package promql

import (
	"os"
	"reflect"
	"testing"
	"unsafe"

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

		// Check 8-byte alignment.
		// This prevents from SIGBUS error on arm architectures.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/3927
		for _, ts := range tss2 {
			if len(ts.Values) == 0 {
				continue
			}

			// check float64 alignment
			addr := uintptr(unsafe.Pointer(&ts.Values[0]))
			if mod := addr % unsafe.Alignof(ts.Values[0]); mod != 0 {
				t.Fatalf("mis-aligned; &ts.Values[0]=%p; mod=%d", &ts.Values[0], mod)
			}
			// check int64 alignment
			addr = uintptr(unsafe.Pointer(&ts.Timestamps[0]))
			if mod := addr % unsafe.Alignof(ts.Timestamps[0]); mod != 0 {
				t.Fatalf("mis-aligned; &ts.Timestamps[0]=%p; mod=%d", &ts.Timestamps[0], mod)
			}
		}
	}

	// Single series
	f([]*timeseries{{
		MetricName: storage.MetricName{
			MetricGroup: []byte{},
		},
		Values:     []float64{},
		Timestamps: []int64{},
		denyReuse:  true,
	}})
	f([]*timeseries{{
		MetricName: storage.MetricName{
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
