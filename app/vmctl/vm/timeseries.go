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

func (cw *cWriter) printf(format string, args ...any) {
	if cw.err != nil {
		return
	}
	n, err := fmt.Fprintf(cw.w, format, args...)
	cw.n += n
	cw.err = err
}

// "{"metric":{"__name__":"cpu_usage_guest","arch":"x64","hostname":"host_19",},"timestamps":[1567296000000,1567296010000],"values":[1567296000000,66]}
func (ts *TimeSeries) write(w io.Writer) (int, error) {
	timestamps := ts.Timestamps
	values := ts.Values
	cw := &cWriter{w: w}
	for len(timestamps) > 0 {
		// Split long lines with more than 10K samples into multiple JSON lines.
		// This should limit memory usage at VictoriaMetrics during data ingestion,
		// since it allocates memory for the whole JSON line and processes it in one go.
		batchSize := min(10000, len(timestamps))
		timestampsBatch := timestamps[:batchSize]
		valuesBatch := values[:batchSize]
		timestamps = timestamps[batchSize:]
		values = values[batchSize:]

		cw.printf(`{"metric":{"__name__":%q`, ts.Name)
		for _, lp := range ts.LabelPairs {
			cw.printf(",%q:%q", lp.Name, lp.Value)
		}

		pointsCount := len(timestampsBatch)
		cw.printf(`},"timestamps":[`)
		for i := 0; i < pointsCount-1; i++ {
			cw.printf(`%d,`, timestampsBatch[i])
		}
		cw.printf(`%d],"values":[`, timestampsBatch[pointsCount-1])
		for i := 0; i < pointsCount-1; i++ {
			cw.printf(`%v,`, valuesBatch[i])
		}
		cw.printf("%v]}\n", valuesBatch[pointsCount-1])
	}
	return cw.n, cw.err
}
