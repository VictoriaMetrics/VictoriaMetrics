package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
)

// RecordingRule is a Rule that supposed
// to evaluate configured Expression and
// return TimeSeries as result.
type RecordingRule struct {
	Type    datasource.Type
	RuleID  uint64
	Name    string
	Expr    string
	Labels  map[string]string
	GroupID uint64

	q datasource.Querier

	// guard status fields
	mu sync.RWMutex
	// stores last moment of time Exec was called
	lastExecTime time.Time
	// stores last error that happened in Exec func
	// resets on every successful Exec
	// may be used as Health state
	lastExecError error
	// stores the number of samples returned during
	// the last evaluation
	lastExecSamples int

	metrics *recordingRuleMetrics
}

type recordingRuleMetrics struct {
	errors  *gauge
	samples *gauge
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

func newRecordingRule(qb datasource.QuerierBuilder, group *Group, cfg config.Rule) *RecordingRule {
	rr := &RecordingRule{
		Type:    group.Type,
		RuleID:  cfg.ID,
		Name:    cfg.Record,
		Expr:    cfg.Expr,
		Labels:  cfg.Labels,
		GroupID: group.ID(),
		metrics: &recordingRuleMetrics{},
		q: qb.BuildWithParams(datasource.QuerierParams{
			DataSourceType:     &group.Type,
			EvaluationInterval: group.Interval,
			QueryParams:        group.Params,
		}),
	}

	labels := fmt.Sprintf(`recording=%q, group=%q, id="%d"`, rr.Name, group.Name, rr.ID())
	rr.metrics.errors = getOrCreateGauge(fmt.Sprintf(`vmalert_recording_rules_error{%s}`, labels),
		func() float64 {
			rr.mu.RLock()
			defer rr.mu.RUnlock()
			if rr.lastExecError == nil {
				return 0
			}
			return 1
		})
	rr.metrics.samples = getOrCreateGauge(fmt.Sprintf(`vmalert_recording_rules_last_evaluation_samples{%s}`, labels),
		func() float64 {
			rr.mu.RLock()
			defer rr.mu.RUnlock()
			return float64(rr.lastExecSamples)
		})
	return rr
}

// Close unregisters rule metrics
func (rr *RecordingRule) Close() {
	metrics.UnregisterMetric(rr.metrics.errors.name)
	metrics.UnregisterMetric(rr.metrics.samples.name)
}

// ExecRange executes recording rule on the given time range similarly to Exec.
// It doesn't update internal states of the Rule and meant to be used just
// to get time series for backfilling.
func (rr *RecordingRule) ExecRange(ctx context.Context, start, end time.Time) ([]prompbmarshal.TimeSeries, error) {
	series, err := rr.q.QueryRange(ctx, rr.Expr, start, end)
	if err != nil {
		return nil, err
	}
	duplicates := make(map[string]struct{}, len(series))
	var tss []prompbmarshal.TimeSeries
	for _, s := range series {
		ts := rr.toTimeSeries(s)
		key := stringifyLabels(ts)
		if _, ok := duplicates[key]; ok {
			return nil, fmt.Errorf("original metric %v; resulting labels %q: %w", s.Labels, key, errDuplicate)
		}
		duplicates[key] = struct{}{}
		tss = append(tss, ts)
	}
	return tss, nil
}

// Exec executes RecordingRule expression via the given Querier.
func (rr *RecordingRule) Exec(ctx context.Context) ([]prompbmarshal.TimeSeries, error) {
	qMetrics, err := rr.q.Query(ctx, rr.Expr)
	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.lastExecTime = time.Now()
	rr.lastExecError = err
	rr.lastExecSamples = len(qMetrics)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query %q: %w", rr.Expr, err)
	}

	duplicates := make(map[string]struct{}, len(qMetrics))
	var tss []prompbmarshal.TimeSeries
	for _, r := range qMetrics {
		ts := rr.toTimeSeries(r)
		key := stringifyLabels(ts)
		if _, ok := duplicates[key]; ok {
			rr.lastExecError = errDuplicate
			return nil, fmt.Errorf("original metric %v; resulting labels %q: %w", r, key, errDuplicate)
		}
		duplicates[key] = struct{}{}
		tss = append(tss, ts)
	}
	return tss, nil
}

func stringifyLabels(ts prompbmarshal.TimeSeries) string {
	labels := ts.Labels
	if len(labels) > 1 {
		sort.Slice(labels, func(i, j int) bool {
			return labels[i].Name < labels[j].Name
		})
	}
	b := strings.Builder{}
	for i, l := range labels {
		b.WriteString(l.Name)
		b.WriteString("=")
		b.WriteString(l.Value)
		if i != len(labels)-1 {
			b.WriteString(",")
		}
	}
	return b.String()
}

func (rr *RecordingRule) toTimeSeries(m datasource.Metric) prompbmarshal.TimeSeries {
	labels := make(map[string]string)
	for _, l := range m.Labels {
		labels[l.Name] = l.Value
	}
	labels["__name__"] = rr.Name
	// override existing labels with configured ones
	for k, v := range rr.Labels {
		labels[k] = v
	}
	return newTimeSeries(m.Values, m.Timestamps, labels)
}

// UpdateWith copies all significant fields.
func (rr *RecordingRule) UpdateWith(r Rule) error {
	nr, ok := r.(*RecordingRule)
	if !ok {
		return fmt.Errorf("BUG: attempt to update recroding rule with wrong type %#v", r)
	}
	rr.Expr = nr.Expr
	rr.Labels = nr.Labels
	rr.q = nr.q
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
		ID:          fmt.Sprintf("%d", rr.ID()),
		GroupID:     fmt.Sprintf("%d", rr.GroupID),
		Name:        rr.Name,
		Type:        rr.Type.String(),
		Expression:  rr.Expr,
		LastError:   lastErr,
		LastSamples: rr.lastExecSamples,
		LastExec:    rr.lastExecTime,
		Labels:      rr.Labels,
	}
}
