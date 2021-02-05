package vm

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestTimeSeries_Write(t *testing.T) {
	var testCases = []struct {
		name string
		ts   *TimeSeries
		exp  string
	}{
		{
			name: "one datapoint",
			ts: &TimeSeries{
				Name: "foo",
				LabelPairs: []LabelPair{
					{
						Name:  "key",
						Value: "val",
					},
				},
				Timestamps: []int64{1577877162200},
				Values:     []float64{1},
			},
			exp: `{"metric":{"__name__":"foo","key":"val"},"timestamps":[1577877162200],"values":[1]}`,
		},
		{
			name: "multiple samples",
			ts: &TimeSeries{
				Name: "foo",
				LabelPairs: []LabelPair{
					{
						Name:  "key",
						Value: "val",
					},
				},
				Timestamps: []int64{1577877162200, 15778771622400, 15778771622600},
				Values:     []float64{1, 1.6263, 32.123},
			},
			exp: `{"metric":{"__name__":"foo","key":"val"},"timestamps":[1577877162200,15778771622400,15778771622600],"values":[1,1.6263,32.123]}`,
		},
		{
			name: "no samples",
			ts: &TimeSeries{
				Name: "foo",
				LabelPairs: []LabelPair{
					{
						Name:  "key",
						Value: "val",
					},
				},
			},
			exp: ``,
		},
		{
			name: "inf values",
			ts: &TimeSeries{
				Name: "foo",
				LabelPairs: []LabelPair{
					{
						Name:  "key",
						Value: "val",
					},
				},
				Timestamps: []int64{1577877162200, 1577877162200, 1577877162200},
				Values:     []float64{0, math.Inf(-1), math.Inf(1)},
			},
			exp: `{"metric":{"__name__":"foo","key":"val"},"timestamps":[1577877162200,1577877162200,1577877162200],"values":[0,-Inf,+Inf]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := &bytes.Buffer{}
			_, err := tc.ts.write(b)
			if err != nil {
				t.Error(err)
			}
			got := strings.TrimSpace(b.String())
			if got != tc.exp {
				t.Fatalf("\ngot:  %q\nwant: %q", got, tc.exp)
			}
		})
	}
}
