package main

import (
	"context"
	"errors"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"time"
)

// Rule represents alerting or recording rule
// that has unique ID, can be Executed and
// updated with other Rule.
type Rule interface {
	// ID returns unique ID that may be used for
	// identifying this Rule among others.
	ID() uint64
	// Exec executes the rule with given context
	Exec(ctx context.Context) ([]prompbmarshal.TimeSeries, error)
	// ExecRange executes the rule on the given time range
	ExecRange(ctx context.Context, start, end time.Time) ([]prompbmarshal.TimeSeries, error)
	// UpdateWith performs modification of current Rule
	// with fields of the given Rule.
	UpdateWith(Rule) error
	// Close performs the shutdown procedures for rule
	// such as metrics unregister
	Close()
}

var errDuplicate = errors.New("result contains metrics with the same labelset after applying rule labels")
