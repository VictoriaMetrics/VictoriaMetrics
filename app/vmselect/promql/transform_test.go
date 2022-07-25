package promql

import (
	"log"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestFixBrokenBuckets(t *testing.T) {
	f := func(values, expectedResult []float64) {
		t.Helper()
		xss := make([]leTimeseries, len(values))
		for i, v := range values {
			xss[i].ts = &timeseries{
				Values: []float64{v},
			}
		}
		fixBrokenBuckets(0, xss)
		result := make([]float64, len(values))
		for i, xs := range xss {
			result[i] = xs.ts.Values[0]
		}
		if !reflect.DeepEqual(result, expectedResult) {
			t.Fatalf("unexpected result for values=%v\ngot\n%v\nwant\n%v", values, result, expectedResult)
		}
	}
	f(nil, []float64{})
	f([]float64{1}, []float64{1})
	f([]float64{1, 2}, []float64{1, 2})
	f([]float64{2, 1}, []float64{1, 1})
	f([]float64{1, 2, 3, nan, nan}, []float64{1, 2, 3, 3, 3})
	f([]float64{5, 1, 2, 3, nan}, []float64{1, 1, 2, 3, 3})
	f([]float64{1, 5, 2, nan, 6, 3}, []float64{1, 2, 2, 3, 3, 3})
	f([]float64{5, 10, 4, 3}, []float64{3, 3, 3, 3})
}

func Test_vmrangeBucketsToLE(t *testing.T) {
	f := func(metrics string, expected []*timeseries) {
		t.Helper()

		var rows prometheus.Rows
		rows.UnmarshalWithErrLogger(metrics, func(errStr string) {
			t.Fatalf("unexpected error when parsing %s: %s", metrics, errStr)
		})
		var tss []*timeseries
		for _, row := range rows.Rows {
			log.Printf("ROW => %#v", row)
			var tags []storage.Tag
			for _, tag := range row.Tags {
				tags = append(tags, storage.Tag{
					Key:   []byte(tag.Key),
					Value: []byte(tag.Value),
				})
			}
			var ts timeseries
			ts.MetricName.MetricGroup = []byte(row.Metric)
			ts.MetricName.Tags = tags
			ts.Timestamps = append(ts.Timestamps, row.Timestamp)
			ts.Values = append(ts.Values, row.Value)
			tss = append(tss, &ts)
		}
		if got := vmrangeBucketsToLE(tss); !reflect.DeepEqual(got, expected) {
			t.Errorf("vmrangeBucketsToLE() = %#v, want %#v", got, expected)
		}
	}

	f(`vm_rows_read_per_query_bucket{vmrange="4.084e+02...4.642e+02"} 2`, []*timeseries{
		&timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte("vm_rows_read_per_query_bucket"),
				Tags: []storage.Tag{
					{Key: []byte("le"), Value: []byte("4.084e+02")},
				},
			},
			Values:     []float64{0},
			Timestamps: []int64{0},
			denyReuse:  true,
		},
		&timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte("vm_rows_read_per_query_bucket"),
				Tags: []storage.Tag{
					{Key: []byte("le"), Value: []byte("4.642e+02")},
				},
			},
			Values:     []float64{2},
			Timestamps: []int64{0},
			denyReuse:  false,
		},
		&timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte("vm_rows_read_per_query_bucket"),
				Tags: []storage.Tag{
					{Key: []byte("le"), Value: []byte("+Inf")},
				},
			},
			Values:     []float64{2},
			Timestamps: []int64{0},
			denyReuse:  true,
		},
	})
	// This test is panic
	f(`vm_rows_read_per_query_bucket{vmrange="0...+Inf"} 0`, []*timeseries{})
	f(`vm_rows_read_per_query_bucket{vmrange="-Inf...0"} 0`, []*timeseries{
		&timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte("vm_rows_read_per_query_bucket"),
				Tags: []storage.Tag{
					{Key: []byte("le"), Value: []byte("+Inf")},
				},
			},
			Values:     []float64{0},
			Timestamps: []int64{0},
			denyReuse:  true,
		},
	})
	f(`vm_rows_read_per_query_bucket{vmrange="0...0"} 0`, []*timeseries{
		&timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte("vm_rows_read_per_query_bucket"),
				Tags: []storage.Tag{
					{Key: []byte("le"), Value: []byte("+Inf")},
				},
			},
			Values:     []float64{0},
			Timestamps: []int64{0},
			denyReuse:  true,
		},
	})
	f(`vm_rows_read_per_query_bucket{vmrange="-Inf...+Inf"} 0`, []*timeseries{})
	f(`vm_rows_read_per_query_bucket{vmrange="-Inf...+Inf"} 1`, []*timeseries{
		&timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte("vm_rows_read_per_query_bucket"),
				Tags: []storage.Tag{
					{Key: []byte("le"), Value: []byte("-Inf")},
				},
			},
			Values:     []float64{0},
			Timestamps: []int64{0},
			denyReuse:  true,
		},
		&timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte("vm_rows_read_per_query_bucket"),
				Tags: []storage.Tag{
					{Key: []byte("le"), Value: []byte("+Inf")},
				},
			},
			Values:     []float64{1},
			Timestamps: []int64{0},
			denyReuse:  false,
		},
	})
	f(`vm_rows_read_per_query_bucket{vmrange="4.084e+02...4.642e+02"} 0`, []*timeseries{
		&timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte("vm_rows_read_per_query_bucket"),
				Tags: []storage.Tag{
					{Key: []byte("le"), Value: []byte("+Inf")},
				},
			},
			Values:     []float64{0},
			Timestamps: []int64{0},
			denyReuse:  true,
		},
	})
}
