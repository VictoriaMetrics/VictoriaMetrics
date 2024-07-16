package vm

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestTimeSeriesWrite(t *testing.T) {
	f := func(ts *TimeSeries, resultExpected string) {
		t.Helper()

		var b bytes.Buffer
		_, err := ts.write(&b)
		if err != nil {
			t.Fatalf("error in TimeSeries.write: %s", err)
		}
		result := strings.TrimSpace(b.String())
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	// one datapoint
	f(&TimeSeries{
		Name: "foo",
		LabelPairs: []LabelPair{
			{
				Name:  "key",
				Value: "val",
			},
		},
		Timestamps: []int64{1577877162200},
		Values:     []float64{1},
	}, `{"metric":{"__name__":"foo","key":"val"},"timestamps":[1577877162200],"values":[1]}`)

	// multiple samples
	f(&TimeSeries{
		Name: "foo",
		LabelPairs: []LabelPair{
			{
				Name:  "key",
				Value: "val",
			},
		},
		Timestamps: []int64{1577877162200, 15778771622400, 15778771622600},
		Values:     []float64{1, 1.6263, 32.123},
	}, `{"metric":{"__name__":"foo","key":"val"},"timestamps":[1577877162200,15778771622400,15778771622600],"values":[1,1.6263,32.123]}`)

	// no samples
	f(&TimeSeries{
		Name: "foo",
		LabelPairs: []LabelPair{
			{
				Name:  "key",
				Value: "val",
			},
		},
	}, ``)

	// inf values
	f(&TimeSeries{
		Name: "foo",
		LabelPairs: []LabelPair{
			{
				Name:  "key",
				Value: "val",
			},
		},
		Timestamps: []int64{1577877162200, 1577877162200, 1577877162200},
		Values:     []float64{0, math.Inf(-1), math.Inf(1)},
	}, `{"metric":{"__name__":"foo","key":"val"},"timestamps":[1577877162200,1577877162200,1577877162200],"values":[0,-Inf,+Inf]}`)
}
