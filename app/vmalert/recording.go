package main

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
)

// RecordingRule is a Rule that supposed
// to evaluate configured Expression and
// return TimeSeries as result.
type RecordingRule struct {
	RuleID         uint64
	Name           string
	Expr           string
	Labels         map[string]string
	GroupID        uint64
	GroupAuthToken *auth.Token

	// guard status fields
	mu sync.RWMutex
	// stores last moment of time Exec was called
	lastExecTime time.Time
	// stores last error that happened in Exec func
	// resets on every successful Exec
	// may be used as Health state
	lastExecError error

	metrics *recordingRuleMetrics
}

type recordingRuleMetrics struct {
	errors *gauge
}

// String implements Stringer interface
func (rr *RecordingRule) String() string {
	return rr.Name
}

// ID returns unique Rule ID
// within the parent Group.
func (rr *RecordingRule) ID() uint64 {
	return rr.RuleID
}

func newRecordingRule(group *Group, cfg config.Rule) *RecordingRule {
	rr := &RecordingRule{
		RuleID:         cfg.ID,
		Name:           cfg.Record,
		Expr:           cfg.Expr,
		Labels:         cfg.Labels,
		GroupID:        group.ID(),
		GroupAuthToken: group.AuthToken,
		metrics:        &recordingRuleMetrics{},
	}
	labels := fmt.Sprintf(`recording=%q, group=%q, id="%d"`, rr.Name, group.Name, rr.ID())
	rr.metrics.errors = getOrCreateGauge(fmt.Sprintf(`vmalert_recording_rules_error{%s}`, labels),
		func() float64 {
			rr.mu.Lock()
			defer rr.mu.Unlock()
			if rr.lastExecError == nil {
				return 0
			}
			return 1
		})
	return rr
}

// Close unregisters rule metrics
func (rr *RecordingRule) Close() {
	metrics.UnregisterMetric(rr.metrics.errors.name)
}

var errDuplicate = errors.New("result contains metrics with the same labelset after applying rule labels")

// Exec executes RecordingRule expression via the given Querier.
func (rr *RecordingRule) Exec(ctx context.Context, q datasource.Querier, series bool) ([]prompbmarshal.TimeSeries, error) {
	if !series {
		return nil, nil
	}

	qMetrics, err := q.Query(ctx, rr.GroupAuthToken, rr.Expr)

	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.lastExecTime = time.Now()
	rr.lastExecError = err
	if err != nil {
		return nil, fmt.Errorf("failed to execute query %q: %w", rr.Expr, err)
	}

	duplicates := make(map[uint64]prompbmarshal.TimeSeries, len(qMetrics))
	var tss []prompbmarshal.TimeSeries
	for _, r := range qMetrics {
		ts := rr.toTimeSeries(r, rr.lastExecTime)
		h := hashTimeSeries(ts)
		if _, ok := duplicates[h]; ok {
			rr.lastExecError = errDuplicate
			return nil, errDuplicate
		}
		duplicates[h] = ts
		tss = append(tss, ts)
	}
	return tss, nil
}

func hashTimeSeries(ts prompbmarshal.TimeSeries) uint64 {
	hash := fnv.New64a()
	labels := ts.Labels
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})
	for _, l := range labels {
		hash.Write([]byte(l.Name))
		hash.Write([]byte(l.Value))
		hash.Write([]byte("\xff"))
	}
	return hash.Sum64()
}

func (rr *RecordingRule) toTimeSeries(m datasource.Metric, timestamp time.Time) prompbmarshal.TimeSeries {
	labels := make(map[string]string)
	for _, l := range m.Labels {
		labels[l.Name] = l.Value
	}
	labels["__name__"] = rr.Name
	// override existing labels with configured ones
	for k, v := range rr.Labels {
		labels[k] = v
	}
	return newTimeSeries(m.Value, labels, timestamp)
}

// UpdateWith copies all significant fields.
// alerts state isn't copied since
// it should be updated in next 2 Execs
func (rr *RecordingRule) UpdateWith(r Rule) error {
	nr, ok := r.(*RecordingRule)
	if !ok {
		return fmt.Errorf("BUG: attempt to update recroding rule with wrong type %#v", r)
	}
	rr.Expr = nr.Expr
	rr.Labels = nr.Labels
	return nil
}

// RuleAPI returns Rule representation in form
// of APIRecordingRule
func (rr *RecordingRule) RuleAPI() APIRecordingRule {
	var lastErr string
	if rr.lastExecError != nil {
		lastErr = rr.lastExecError.Error()
	}
	return APIRecordingRule{
		// encode as strings to avoid rounding
		ID:         fmt.Sprintf("%d", rr.ID()),
		GroupID:    fmt.Sprintf("%d", rr.GroupID),
		Name:       rr.Name,
		Expression: rr.Expr,
		LastError:  lastErr,
		LastExec:   rr.lastExecTime,
		Labels:     rr.Labels,
	}
}
