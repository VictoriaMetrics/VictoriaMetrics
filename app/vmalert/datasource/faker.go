package datasource

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// FakeQuerier is a mock querier that return predefined results and error message
type FakeQuerier struct {
	sync.Mutex
	metrics []Metric
	err     error
}

// SetErr sets query error message
func (fq *FakeQuerier) SetErr(err error) {
	fq.Lock()
	fq.err = err
	fq.Unlock()
}

// Reset reset querier's error message and results
func (fq *FakeQuerier) Reset() {
	fq.Lock()
	fq.err = nil
	fq.metrics = fq.metrics[:0]
	fq.Unlock()
}

// Add appends metrics to querier result metrics
func (fq *FakeQuerier) Add(metrics ...Metric) {
	fq.Lock()
	fq.metrics = append(fq.metrics, metrics...)
	fq.Unlock()
}

// BuildWithParams return FakeQuerier itself
func (fq *FakeQuerier) BuildWithParams(_ QuerierParams) Querier {
	return fq
}

// QueryRange performs query
func (fq *FakeQuerier) QueryRange(ctx context.Context, q string, _, _ time.Time) (Result, error) {
	req, _, err := fq.Query(ctx, q, time.Now())
	return req, err
}

// Query returns metrics restored in querier
func (fq *FakeQuerier) Query(_ context.Context, _ string, _ time.Time) (Result, *http.Request, error) {
	fq.Lock()
	defer fq.Unlock()
	if fq.err != nil {
		return Result{}, nil, fq.err
	}
	cp := make([]Metric, len(fq.metrics))
	copy(cp, fq.metrics)
	req, _ := http.NewRequest(http.MethodPost, "foo.com", nil)
	return Result{Data: cp}, req, nil
}

// FakeQuerierWithRegistry can store different results for different query expr
type FakeQuerierWithRegistry struct {
	sync.Mutex
	registry map[string][]Metric
}

// Set stores query result for given key
func (fqr *FakeQuerierWithRegistry) Set(key string, metrics ...Metric) {
	fqr.Lock()
	if fqr.registry == nil {
		fqr.registry = make(map[string][]Metric)
	}
	fqr.registry[key] = metrics
	fqr.Unlock()
}

// Reset clean querier's results registry
func (fqr *FakeQuerierWithRegistry) Reset() {
	fqr.Lock()
	fqr.registry = nil
	fqr.Unlock()
}

// BuildWithParams returns itself
func (fqr *FakeQuerierWithRegistry) BuildWithParams(_ QuerierParams) Querier {
	return fqr
}

// QueryRange performs query
func (fqr *FakeQuerierWithRegistry) QueryRange(ctx context.Context, q string, _, _ time.Time) (Result, error) {
	req, _, err := fqr.Query(ctx, q, time.Now())
	return req, err
}

// Query returns metrics restored in querier registry
func (fqr *FakeQuerierWithRegistry) Query(_ context.Context, expr string, _ time.Time) (Result, *http.Request, error) {
	fqr.Lock()
	defer fqr.Unlock()

	req, _ := http.NewRequest(http.MethodPost, "foo.com", nil)
	metrics, ok := fqr.registry[expr]
	if !ok {
		return Result{}, req, nil
	}
	cp := make([]Metric, len(metrics))
	copy(cp, metrics)
	return Result{Data: cp}, req, nil
}

// FakeQuerierWithDelay mock querier with given delay duration
type FakeQuerierWithDelay struct {
	FakeQuerier
	Delay time.Duration
}

// Query returns query result after delay duration
func (fqd *FakeQuerierWithDelay) Query(ctx context.Context, expr string, ts time.Time) (Result, *http.Request, error) {
	timer := time.NewTimer(fqd.Delay)
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	return fqd.FakeQuerier.Query(ctx, expr, ts)
}

// BuildWithParams returns itself
func (fqd *FakeQuerierWithDelay) BuildWithParams(_ QuerierParams) Querier {
	return fqd
}
