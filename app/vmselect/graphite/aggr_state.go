package graphite

import (
	"fmt"
	"math"
	"strings"

	"github.com/valyala/histogram"
)

var aggrStateFuncs = map[string]func(int) aggrState{
	"average":  newAggrStateAvg,
	"avg":      newAggrStateAvg,
	"avg_zero": newAggrStateAvgZero,
	"median":   newAggrStateMedian,
	"sum":      newAggrStateSum,
	"total":    newAggrStateSum,
	"min":      newAggrStateMin,
	"max":      newAggrStateMax,
	"diff":     newAggrStateDiff,
	"pow":      newAggrStatePow,
	"stddev":   newAggrStateStddev,
	"count":    newAggrStateCount,
	"range":    newAggrStateRange,
	"rangeOf":  newAggrStateRange,
	"multiply": newAggrStateMultiply,
	"first":    newAggrStateFirst,
	"last":     newAggrStateLast,
	"current":  newAggrStateLast,
}

type aggrState interface {
	Update(values []float64)
	Finalize(xFilesFactor float64) []float64
}

func newAggrState(pointsLen int, funcName string) (aggrState, error) {
	s := strings.TrimSuffix(funcName, "Series")
	asf := aggrStateFuncs[s]
	if asf == nil {
		return nil, fmt.Errorf("unsupported aggregate function %q", funcName)
	}
	return asf(pointsLen), nil
}

type aggrStateAvg struct {
	pointsLen   int
	sums        []float64
	counts      []int
	seriesTotal int
}

func newAggrStateAvg(pointsLen int) aggrState {
	return &aggrStateAvg{
		pointsLen: pointsLen,
		sums:      make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateAvg) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	sums := as.sums
	counts := as.counts
	for i, v := range values {
		if !math.IsNaN(v) {
			sums[i] += v
			counts[i]++
		}
	}
	as.seriesTotal++
}

func (as *aggrStateAvg) Finalize(xFilesFactor float64) []float64 {
	sums := as.sums
	counts := as.counts
	values := make([]float64, as.pointsLen)
	xff := int(xFilesFactor * float64(as.seriesTotal))
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = sums[i] / float64(count)
		}
		values[i] = v
	}
	return values
}

type aggrStateAvgZero struct {
	pointsLen   int
	sums        []float64
	seriesTotal int
}

func newAggrStateAvgZero(pointsLen int) aggrState {
	return &aggrStateAvgZero{
		pointsLen: pointsLen,
		sums:      make([]float64, pointsLen),
	}
}

func (as *aggrStateAvgZero) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	sums := as.sums
	for i, v := range values {
		if !math.IsNaN(v) {
			sums[i] += v
		}
	}
	as.seriesTotal++
}

func (as *aggrStateAvgZero) Finalize(_ float64) []float64 {
	sums := as.sums
	values := make([]float64, as.pointsLen)
	count := float64(as.seriesTotal)
	for i, sum := range sums {
		v := nan
		if count > 0 {
			v = sum / count
		}
		values[i] = v
	}
	return values
}

func newAggrStateMedian(pointsLen int) aggrState {
	return newAggrStatePercentile(pointsLen, 50)
}

type aggrStatePercentile struct {
	phi         float64
	pointsLen   int
	hs          []*histogram.Fast
	counts      []int
	seriesTotal int
}

func newAggrStatePercentile(pointsLen int, n float64) aggrState {
	hs := make([]*histogram.Fast, pointsLen)
	for i := 0; i < pointsLen; i++ {
		hs[i] = histogram.NewFast()
	}
	return &aggrStatePercentile{
		phi:       n / 100,
		pointsLen: pointsLen,
		hs:        hs,
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStatePercentile) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	hs := as.hs
	counts := as.counts
	for i, v := range values {
		if !math.IsNaN(v) {
			hs[i].Update(v)
			counts[i]++
		}
	}
	as.seriesTotal++
}

func (as *aggrStatePercentile) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	hs := as.hs
	for i, count := range as.counts {
		v := nan
		if count > 0 && count >= xff {
			v = hs[i].Quantile(as.phi)
		}
		values[i] = v
	}
	return values
}

type aggrStateSum struct {
	pointsLen   int
	sums        []float64
	counts      []int
	seriesTotal int
}

func newAggrStateSum(pointsLen int) aggrState {
	return &aggrStateSum{
		pointsLen: pointsLen,
		sums:      make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateSum) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	sums := as.sums
	counts := as.counts
	for i, v := range values {
		if !math.IsNaN(v) {
			sums[i] += v
			counts[i]++
		}
	}
	as.seriesTotal++
}

func (as *aggrStateSum) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	sums := as.sums
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = sums[i]
		}
		values[i] = v
	}
	return values
}

type aggrStateMin struct {
	pointsLen   int
	mins        []float64
	counts      []int
	seriesTotal int
}

func newAggrStateMin(pointsLen int) aggrState {
	return &aggrStateMin{
		pointsLen: pointsLen,
		mins:      make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateMin) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	mins := as.mins
	counts := as.counts
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		counts[i]++
		if counts[i] == 1 {
			mins[i] = v
		} else if v < mins[i] {
			mins[i] = v
		}
	}
	as.seriesTotal++
}

func (as *aggrStateMin) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	mins := as.mins
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = mins[i]
		}
		values[i] = v
	}
	return values
}

type aggrStateMax struct {
	pointsLen   int
	maxs        []float64
	counts      []int
	seriesTotal int
}

func newAggrStateMax(pointsLen int) aggrState {
	return &aggrStateMax{
		pointsLen: pointsLen,
		maxs:      make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateMax) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	maxs := as.maxs
	counts := as.counts
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		counts[i]++
		if counts[i] == 1 {
			maxs[i] = v
		} else if v > maxs[i] {
			maxs[i] = v
		}
	}
	as.seriesTotal++
}

func (as *aggrStateMax) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	maxs := as.maxs
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = maxs[i]
		}
		values[i] = v
	}
	return values
}

type aggrStateDiff struct {
	pointsLen   int
	vs          []float64
	counts      []int
	seriesTotal int
}

func newAggrStateDiff(pointsLen int) aggrState {
	return &aggrStateDiff{
		pointsLen: pointsLen,
		vs:        make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateDiff) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	vs := as.vs
	counts := as.counts
	for i, v := range values {
		if !math.IsNaN(v) {
			if counts[i] == 0 {
				vs[i] = v
			} else {
				vs[i] -= v
			}
			counts[i]++
		}
	}
	as.seriesTotal++
}

func (as *aggrStateDiff) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	vs := as.vs
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = vs[i]
		}
		values[i] = v
	}
	return values
}

type aggrStatePow struct {
	pointsLen   int
	vs          []float64
	counts      []int
	seriesTotal int
}

func newAggrStatePow(pointsLen int) aggrState {
	return &aggrStatePow{
		pointsLen: pointsLen,
		vs:        make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStatePow) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	vs := as.vs
	counts := as.counts
	for i, v := range values {
		if !math.IsNaN(v) {
			if counts[i] == 0 {
				vs[i] = v
			} else {
				vs[i] = math.Pow(vs[i], v)
			}
			counts[i]++
		}
	}
	as.seriesTotal++
}

func (as *aggrStatePow) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	vs := as.vs
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = vs[i]
		}
		values[i] = v
	}
	return values
}

type aggrStateStddev struct {
	pointsLen   int
	means       []float64
	m2s         []float64
	counts      []int
	seriesTotal int
}

func newAggrStateStddev(pointsLen int) aggrState {
	return &aggrStateStddev{
		pointsLen: pointsLen,
		means:     make([]float64, pointsLen),
		m2s:       make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateStddev) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	means := as.means
	m2s := as.m2s
	counts := as.counts
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		// See https://en.m.wikipedia.org/wiki/Algorithms_for_calculating_variance#Welford's_online_algorithm
		count := counts[i]
		mean := means[i]
		count++
		delta := v - mean
		mean += delta / float64(count)
		delta2 := v - mean
		means[i] = mean
		m2s[i] += delta * delta2
		counts[i] = count
	}
	as.seriesTotal++
}

func (as *aggrStateStddev) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	m2s := as.m2s
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = math.Sqrt(m2s[i] / float64(count))
		}
		values[i] = v
	}
	return values
}

type aggrStateCount struct {
	pointsLen   int
	counts      []int
	seriesTotal int
}

func newAggrStateCount(pointsLen int) aggrState {
	return &aggrStateCount{
		pointsLen: pointsLen,
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateCount) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	counts := as.counts
	for i, v := range values {
		if !math.IsNaN(v) {
			counts[i]++
		}
	}
	as.seriesTotal++
}

func (as *aggrStateCount) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = float64(count)
		}
		values[i] = v
	}
	return values
}

type aggrStateRange struct {
	pointsLen   int
	mins        []float64
	maxs        []float64
	counts      []int
	seriesTotal int
}

func newAggrStateRange(pointsLen int) aggrState {
	return &aggrStateRange{
		pointsLen: pointsLen,
		mins:      make([]float64, pointsLen),
		maxs:      make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateRange) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	mins := as.mins
	maxs := as.maxs
	counts := as.counts
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		counts[i]++
		if counts[i] == 1 {
			mins[i] = v
			maxs[i] = v
		} else if v < mins[i] {
			mins[i] = v
		} else if v > maxs[i] {
			maxs[i] = v
		}
	}
	as.seriesTotal++
}

func (as *aggrStateRange) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	mins := as.mins
	maxs := as.maxs
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = maxs[i] - mins[i]
		}
		values[i] = v
	}
	return values
}

type aggrStateMultiply struct {
	pointsLen   int
	ms          []float64
	counts      []int
	seriesTotal int
}

func newAggrStateMultiply(pointsLen int) aggrState {
	return &aggrStateMultiply{
		pointsLen: pointsLen,
		ms:        make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateMultiply) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	ms := as.ms
	counts := as.counts
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		counts[i]++
		if counts[i] == 1 {
			ms[i] = v
		} else {
			ms[i] *= v
		}
	}
	as.seriesTotal++
}

func (as *aggrStateMultiply) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	ms := as.ms
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = ms[i]
		}
		values[i] = v
	}
	return values
}

type aggrStateFirst struct {
	pointsLen   int
	vs          []float64
	counts      []int
	seriesTotal int
}

func newAggrStateFirst(pointsLen int) aggrState {
	return &aggrStateFirst{
		pointsLen: pointsLen,
		vs:        make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateFirst) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	vs := as.vs
	counts := as.counts
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		counts[i]++
		if counts[i] == 1 {
			vs[i] = v
		}
	}
	as.seriesTotal++
}

func (as *aggrStateFirst) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	vs := as.vs
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = vs[i]
		}
		values[i] = v
	}
	return values
}

type aggrStateLast struct {
	pointsLen   int
	vs          []float64
	counts      []int
	seriesTotal int
}

func newAggrStateLast(pointsLen int) aggrState {
	return &aggrStateLast{
		pointsLen: pointsLen,
		vs:        make([]float64, pointsLen),
		counts:    make([]int, pointsLen),
	}
}

func (as *aggrStateLast) Update(values []float64) {
	if len(values) != as.pointsLen {
		panic(fmt.Errorf("BUG: unexpected number of points in values; got %d; want %d", len(values), as.pointsLen))
	}
	vs := as.vs
	counts := as.counts
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		vs[i] = v
		counts[i]++
	}
	as.seriesTotal++
}

func (as *aggrStateLast) Finalize(xFilesFactor float64) []float64 {
	xff := int(xFilesFactor * float64(as.seriesTotal))
	values := make([]float64, as.pointsLen)
	vs := as.vs
	counts := as.counts
	for i, count := range counts {
		v := nan
		if count > 0 && count >= xff {
			v = vs[i]
		}
		values[i] = v
	}
	return values
}
