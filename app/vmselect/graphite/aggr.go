package graphite

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/valyala/histogram"
)

var aggrFuncs = map[string]aggrFunc{
	"average":  aggrAvg,
	"avg":      aggrAvg,
	"avg_zero": aggrAvgZero,
	"median":   aggrMedian,
	"sum":      aggrSum,
	"total":    aggrSum,
	"min":      aggrMin,
	"max":      aggrMax,
	"diff":     aggrDiff,
	"pow":      aggrPow,
	"stddev":   aggrStddev,
	"count":    aggrCount,
	"range":    aggrRange,
	"rangeOf":  aggrRange,
	"multiply": aggrMultiply,
	"first":    aggrFirst,
	"last":     aggrLast,
	"current":  aggrLast,
}

func getAggrFunc(funcName string) (aggrFunc, error) {
	s := strings.TrimSuffix(funcName, "Series")
	aggrFunc := aggrFuncs[s]
	if aggrFunc == nil {
		return nil, fmt.Errorf("unsupported aggregate function %q", funcName)
	}
	return aggrFunc, nil
}

type aggrFunc func(values []float64) float64

func (af aggrFunc) apply(xFilesFactor float64, values []float64) float64 {
	if aggrCount(values) >= float64(len(values))*xFilesFactor {
		return af(values)
	}
	return nan
}

func aggrAvg(values []float64) float64 {
	pos := getFirstNonNaNPos(values)
	if pos < 0 {
		return nan
	}
	sum := values[pos]
	count := 1
	for _, v := range values[pos+1:] {
		if !math.IsNaN(v) {
			sum += v
			count++
		}
	}
	return sum / float64(count)
}

func aggrAvgZero(values []float64) float64 {
	if len(values) == 0 {
		return nan
	}
	sum := float64(0)
	for _, v := range values {
		if !math.IsNaN(v) {
			sum += v
		}
	}
	return sum / float64(len(values))
}

var aggrMedian = newAggrFuncPercentile(50)

func aggrSum(values []float64) float64 {
	pos := getFirstNonNaNPos(values)
	if pos < 0 {
		return nan
	}
	sum := values[pos]
	for _, v := range values[pos+1:] {
		if !math.IsNaN(v) {
			sum += v
		}
	}
	return sum
}

func aggrMin(values []float64) float64 {
	pos := getFirstNonNaNPos(values)
	if pos < 0 {
		return nan
	}
	min := values[pos]
	for _, v := range values[pos+1:] {
		if !math.IsNaN(v) && v < min {
			min = v
		}
	}
	return min
}

func aggrMax(values []float64) float64 {
	pos := getFirstNonNaNPos(values)
	if pos < 0 {
		return nan
	}
	max := values[pos]
	for _, v := range values[pos+1:] {
		if !math.IsNaN(v) && v > max {
			max = v
		}
	}
	return max
}

func aggrDiff(values []float64) float64 {
	pos := getFirstNonNaNPos(values)
	if pos < 0 {
		return nan
	}
	sum := float64(0)
	for _, v := range values[pos+1:] {
		if !math.IsNaN(v) {
			sum += v
		}
	}
	return values[pos] - sum
}

func aggrPow(values []float64) float64 {
	pos := getFirstNonNaNPos(values)
	if pos < 0 {
		return nan
	}
	pow := values[pos]
	for _, v := range values[pos+1:] {
		if !math.IsNaN(v) {
			pow = math.Pow(pow, v)
		}
	}
	return pow
}

func aggrStddev(values []float64) float64 {
	avg := aggrAvg(values)
	if math.IsNaN(avg) {
		return nan
	}
	sum := float64(0)
	count := 0
	for _, v := range values {
		if !math.IsNaN(v) {
			d := avg - v
			sum += d * d
			count++
		}
	}
	return math.Sqrt(sum / float64(count))
}

func aggrCount(values []float64) float64 {
	count := 0
	for _, v := range values {
		if !math.IsNaN(v) {
			count++
		}
	}
	return float64(count)
}

func aggrRange(values []float64) float64 {
	min := aggrMin(values)
	if math.IsNaN(min) {
		return nan
	}
	max := aggrMax(values)
	return max - min
}

func aggrMultiply(values []float64) float64 {
	pos := getFirstNonNaNPos(values)
	if pos < 0 {
		return nan
	}
	p := values[pos]
	for _, v := range values[pos+1:] {
		if !math.IsNaN(v) {
			p *= v
		}
	}
	return p
}

func aggrFirst(values []float64) float64 {
	pos := getFirstNonNaNPos(values)
	if pos < 0 {
		return nan
	}
	return values[pos]
}

func aggrLast(values []float64) float64 {
	for i := len(values) - 1; i >= 0; i-- {
		v := values[i]
		if !math.IsNaN(v) {
			return v
		}
	}
	return nan
}

func getFirstNonNaNPos(values []float64) int {
	for i, v := range values {
		if !math.IsNaN(v) {
			return i
		}
	}
	return -1
}

var nan = math.NaN()

func newAggrFuncPercentile(n float64) aggrFunc {
	f := func(values []float64) float64 {
		h := getHistogram()
		for _, v := range values {
			if !math.IsNaN(v) {
				h.Update(v)
			}
		}
		p := h.Quantile(n / 100)
		putHistogram(h)
		return p
	}
	return f
}

func getHistogram() *histogram.Fast {
	return histogramPool.Get().(*histogram.Fast)
}

func putHistogram(h *histogram.Fast) {
	h.Reset()
	histogramPool.Put(h)
}

var histogramPool = &sync.Pool{
	New: func() interface{} {
		return histogram.NewFast()
	},
}
