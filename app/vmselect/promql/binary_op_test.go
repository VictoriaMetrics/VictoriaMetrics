package promql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func parseBinaryOpCtxTestExpr(t *testing.T, q string) *metricsql.BinaryOpExpr {
	t.Helper()
	e, err := metricsql.Parse(q)
	if err != nil {
		t.Fatalf("cannot parse %q: %s", q, err)
	}
	be, ok := e.(*metricsql.BinaryOpExpr)
	if !ok {
		t.Fatalf("%q doesn't evaluate to BinaryOpExpr; got %T", q, e)
	}
	return be
}

func newBinaryOpCtxTestTS(metricGroup string, tags map[string]string, points int, value float64) *timeseries {
	mn := storage.MetricName{
		MetricGroup: []byte(metricGroup),
	}
	for k, v := range tags {
		mn.AddTag(k, v)
	}
	values := make([]float64, points)
	timestamps := make([]int64, points)
	for i := range values {
		values[i] = value
		timestamps[i] = int64(i)
	}
	return &timeseries{
		MetricName: mn,
		Timestamps: timestamps,
		Values:     values,
	}
}

// Binary operators must stop the evaluation once the query evaluation
// context is done. Otherwise expensive queries keep running for a long time
// after exceeding -search.maxQueryDuration or after the client goes away.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11929
func TestBinaryOpCanceledContext(t *testing.T) {
	const points = 100

	left := make([]*timeseries, 50)
	right := make([]*timeseries, 50)
	for i := range left {
		tag := fmt.Sprintf("%d", i)
		left[i] = newBinaryOpCtxTestTS("a", map[string]string{"foo": tag}, points, 1)
		right[i] = newBinaryOpCtxTestTS("b", map[string]string{"foo": tag}, points, 1)
	}

	f := func(q string, left, right []*timeseries) {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		bfa := &binaryOpFuncArg{
			be:    parseBinaryOpCtxTestExpr(t, q),
			left:  left,
			right: right,
			ctx:   ctx,
		}
		bf := getBinaryOpFunc(bfa.be.Op)
		start := time.Now()
		_, err := bf(bfa)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected error for %q with canceled context; got %v; want %v", q, err, context.Canceled)
		}
		if d := time.Since(start); d > 10*time.Second {
			t.Fatalf("evaluation of %q wasn't stopped promptly after context cancelation; it took %s", q, d)
		}
	}

	// Set operations.
	f("a or b", left, right)
	f("a unless b", left, right)
	f("a and b", left, right)
	f("a default b", left, right)
	f("a if b", left, right)
	f("a ifnot b", left, right)
	// Arithmetic and comparison operations over vector pairs.
	f("a + b", left, right)
	f("a == b", left, right)
	// Join modifiers go through groupJoin.
	f("a * on(foo) group_left b", left, right)
	f("a * on(foo) group_right b", left, right)
	// Union comparisons have their own loops.
	f("a == (1, 2, 3)", left, right)
	f("a != (1, 2, 3)", left, right)
}

// An expensive `or` over many series and points must stop soon after the
// deadline instead of running the quadratic merge loop to completion.
func TestBinaryOpOrDeadlineExceeded(t *testing.T) {
	const (
		leftN  = 2000
		rightN = 2000
		points = 1000
	)
	left := make([]*timeseries, leftN)
	for i := range left {
		left[i] = newBinaryOpCtxTestTS("a", nil, points, 1)
	}
	right := make([]*timeseries, rightN)
	for i := range right {
		right[i] = newBinaryOpCtxTestTS("b", nil, points, 1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	bfa := &binaryOpFuncArg{
		be:    parseBinaryOpCtxTestExpr(t, "a or b"),
		left:  left,
		right: right,
		ctx:   ctx,
	}
	bf := getBinaryOpFunc(bfa.be.Op)
	start := time.Now()
	_, err := bf(bfa)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error; got %v; want %v", err, context.DeadlineExceeded)
	}
	// The full merge takes orders of magnitude longer than the deadline
	// when the context isn't checked.
	if d := time.Since(start); d > time.Minute {
		t.Fatalf("evaluation wasn't stopped promptly after the deadline; it took %s", d)
	}
}

// Sanity check: the same operations keep working when the context is alive.
func TestBinaryOpAliveContext(t *testing.T) {
	const points = 10
	left := []*timeseries{newBinaryOpCtxTestTS("a", map[string]string{"foo": "x"}, points, 2)}
	right := []*timeseries{newBinaryOpCtxTestTS("b", map[string]string{"foo": "x"}, points, 3)}
	bfa := &binaryOpFuncArg{
		be:    parseBinaryOpCtxTestExpr(t, "a + b"),
		left:  left,
		right: right,
		ctx:   context.Background(),
	}
	bf := getBinaryOpFunc(bfa.be.Op)
	rv, err := bf(bfa)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(rv) != 1 {
		t.Fatalf("unexpected number of result series; got %d; want 1", len(rv))
	}
	if rv[0].Values[0] != 5 {
		t.Fatalf("unexpected result value; got %v; want 5", rv[0].Values[0])
	}
}
