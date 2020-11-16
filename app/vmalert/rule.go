package main

import (
	"context"
	"errors"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// Rule represents alerting or recording rule
// that has unique ID, can be Executed and
// updated with other Rule.
type Rule interface {
	// Returns unique ID that may be used for
	// identifying this Rule among others.
	ID() uint64
	// Exec executes the rule with given context
	// and Querier. If returnSeries is true, Exec
	// may return TimeSeries as result of execution
	Exec(ctx context.Context, q datasource.Querier, returnSeries bool) ([]prompbmarshal.TimeSeries, error)
	// UpdateWith performs modification of current Rule
	// with fields of the given Rule.
	UpdateWith(Rule) error
	// Close performs the shutdown procedures for rule
	// such as metrics unregister
	Close()
}

var errDuplicate = errors.New("result contains metrics with the same labelset after applying rule labels")
