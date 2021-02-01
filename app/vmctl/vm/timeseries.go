package vm

import (
	"fmt"
	"io"
)

// TimeSeries represents a time series.
type TimeSeries struct {
	Name       string
	LabelPairs []LabelPair
	Timestamps []int64
	Values     []float64
}

// LabelPair represents a label
type LabelPair struct {
	Name  string
	Value string
}

// String returns user-readable ts.
func (ts TimeSeries) String() string {
	s := ts.Name
	if len(ts.LabelPairs) < 1 {
		return s
	}
	var labels string
	for i, lp := range ts.LabelPairs {
		labels += fmt.Sprintf("%s=%q", lp.Name, lp.Value)
		if i < len(ts.LabelPairs)-1 {
			labels += ","
		}
	}
	return fmt.Sprintf("%s{%s}", s, labels)
}

// cWriter used to avoid error checking
// while doing Write calls.
// cWriter caches the first error if any
// and discards all sequential write calls
type cWriter struct {
	w   io.Writer
	n   int
	err error
}

func (cw *cWriter) printf(format string, args ...interface{}) {
	if cw.err != nil {
		return
	}
	n, err := fmt.Fprintf(cw.w, format, args...)
	cw.n += n
	cw.err = err
}

//"{"metric":{"__name__":"cpu_usage_guest","arch":"x64","hostname":"host_19",},"timestamps":[1567296000000,1567296010000],"values":[1567296000000,66]}
func (ts *TimeSeries) write(w io.Writer) (int, error) {
	pointsCount := len(ts.Timestamps)
	if pointsCount == 0 {
		return 0, nil
	}

	cw := &cWriter{w: w}
	cw.printf(`{"metric":{"__name__":%q`, ts.Name)
	if len(ts.LabelPairs) > 0 {
		for _, lp := range ts.LabelPairs {
			cw.printf(",%q:%q", lp.Name, lp.Value)
		}
	}

	cw.printf(`},"timestamps":[`)
	for i := 0; i < pointsCount-1; i++ {
		cw.printf(`%d,`, ts.Timestamps[i])
	}
	cw.printf(`%d],"values":[`, ts.Timestamps[pointsCount-1])
	for i := 0; i < pointsCount-1; i++ {
		cw.printf(`%v,`, ts.Values[i])
	}
	cw.printf("%v]}\n", ts.Values[pointsCount-1])
	return cw.n, cw.err
}
