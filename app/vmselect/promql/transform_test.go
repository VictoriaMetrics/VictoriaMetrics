package promql

import (
	"reflect"
	"testing"

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
	tests := []struct {
		name string
		tss  []*timeseries
		want []*timeseries
	}{

		{
			name: "nil timeseries",
			tss:  nil,
			want: []*timeseries{},
		},
		{
			name: "empty timeseries",
			tss:  []*timeseries{},
			want: []*timeseries{},
		},
		// This test is panic. Panic appears if xsPrev.end is Inf, and we skip appending to the slice
		// if !math.IsInf(xsPrev.end, 1) {
		//			xssNew = append(xssNew, x{
		//				endStr: "+Inf",
		//				end:    math.Inf(1),
		//				ts:     copyTS(xsPrev.ts, "+Inf"),
		//			})
		//		}
		// and it is not depend on value of timeseries.Value
		{
			name: "with infinite end time and values is nil",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("0...+Inf")},
						},
					},
					Values:     nil,
					Timestamps: nil,
				},
			},
			want: []*timeseries{},
		},
		// Panic as well
		{
			name: "with infinite end time and values is nil",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("0...+Inf")},
						},
					},
					Values:     []float64{0},
					Timestamps: nil,
				},
			},
			want: []*timeseries{},
		},
		{
			name: "with zeroes on both ranges",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("0...0")},
						},
					},
					Values:     nil,
					Timestamps: nil,
				},
			},
			want: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("+Inf")},
						},
					},
					Values:     []float64(nil),
					Timestamps: []int64(nil),
					denyReuse:  true,
				},
			},
		},
		{
			name: "with zeroes on both ranges and zero value",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("0...1")},
						},
					},
					Values:     []float64{0},
					Timestamps: nil,
				},
			},
			want: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("+Inf")},
						},
					},
					Values:     []float64{0},
					Timestamps: []int64(nil),
					denyReuse:  true,
				},
			},
		},
		{
			name: "range with values",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("1.000e+00...1.136e+00")},
						},
					},
					Values:     []float64{123},
					Timestamps: nil,
				},
			},
			want: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("1.000e+00")},
						},
					},
					Values:     []float64{0},
					Timestamps: []int64(nil),
					denyReuse:  true,
				},
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("1.136e+00")},
						},
					},
					Values:     []float64{123},
					Timestamps: []int64(nil),
					denyReuse:  false,
				},
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("+Inf")},
						},
					},
					Values:     []float64{123},
					Timestamps: []int64(nil),
					denyReuse:  true,
				},
			},
		},
		{
			name: "range with empty first value",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("...1.136e+00")},
						},
					},
					Values:     []float64{123},
					Timestamps: nil,
				},
			},
			want: []*timeseries{},
		},
		{
			name: "range with minus Inf",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("-Inf...0")},
						},
					},
					Values:     []float64{123},
					Timestamps: nil,
				},
			},
			want: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("-Inf")},
						},
					},
					Values:     []float64{0},
					Timestamps: []int64(nil),
					denyReuse:  true,
				},
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("0")},
						},
					},
					Values:     []float64{123},
					Timestamps: []int64(nil),
					denyReuse:  false,
				},
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("+Inf")},
						},
					},
					Values:     []float64{123},
					Timestamps: []int64(nil),
					denyReuse:  true,
				},
			},
		},
		{
			name: "range with empty first value",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("...1.136e+00")},
						},
					},
					Values:     []float64{123},
					Timestamps: nil,
				},
			},
			want: []*timeseries{},
		},
		{
			name: "range with minus Inf and zero value",
			tss: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("vmrange"), Value: []byte("-Inf...0")},
						},
					},
					Values:     []float64{0},
					Timestamps: nil,
				},
			},
			want: []*timeseries{
				&timeseries{
					MetricName: storage.MetricName{
						MetricGroup: []byte("new_metrics"),
						Tags: []storage.Tag{
							{Key: []byte("le"), Value: []byte("+Inf")},
						},
					},
					Values:     []float64{0},
					Timestamps: []int64(nil),
					denyReuse:  true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vmrangeBucketsToLE(tt.tss); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("vmrangeBucketsToLE() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
