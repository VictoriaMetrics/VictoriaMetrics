package tests

import (
	"context"
	"fmt"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote"
)

// PrometheusMockStorage is a mock implementation of the Prometheus remote read storage interface.
type PrometheusMockStorage struct {
	query *prompb.Query
	store []*prompb.TimeSeries
	b     labels.ScratchBuilder
}

// NewPrometheusMockStorage creates a new PrometheusMockStorage with the provided series.
func NewPrometheusMockStorage(series []*prompb.TimeSeries) *PrometheusMockStorage {
	return &PrometheusMockStorage{store: series}
}

// ReadMultiple implemnets the storage.ReadClient interface for reading time series data.
func (ms *PrometheusMockStorage) ReadMultiple(ctx context.Context, queries []*prompb.Query, sortSeries bool) (storage.SeriesSet, error) {
	if len(queries) != 1 {
		panic(fmt.Errorf("reading multiple queries isn't implemented"))
	}

	query := queries[0]
	return ms.Read(ctx, query, sortSeries)
}

// Read implements the storage.ReadClient interface for reading time series data.
func (ms *PrometheusMockStorage) Read(_ context.Context, query *prompb.Query, sortSeries bool) (storage.SeriesSet, error) {
	if ms.query != nil {
		return nil, fmt.Errorf("expected only one call to remote client got: %v", query)
	}
	ms.query = query

	matchers, err := remote.FromLabelMatchers(query.Matchers)
	if err != nil {
		return nil, err
	}

	q := &prompb.QueryResult{}
	for _, s := range ms.store {
		l := s.ToLabels(&ms.b, nil)
		var notMatch bool

		for _, m := range matchers {
			if v := l.Get(m.Name); v != "" {
				if !m.Matches(v) {
					notMatch = true
					break
				}
			}
		}

		if !notMatch {
			q.Timeseries = append(q.Timeseries, &prompb.TimeSeries{Labels: s.Labels, Samples: s.Samples})
		}
	}

	return remote.FromQueryResult(sortSeries, q), nil
}

// Reset resets the PrometheusMockStorage, clearing any stored query and series.
func (ms *PrometheusMockStorage) Reset() {
	ms.query = nil
}
