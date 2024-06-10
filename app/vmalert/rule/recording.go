package rule

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// RecordingRule is a Rule that supposed
// to evaluate configured Expression and
// return TimeSeries as result.
type RecordingRule struct {
	Type      config.Type
	RuleID    uint64
	Name      string
	Expr      string
	Labels    map[string]string
	GroupID   uint64
	GroupName string
	File      string

	q datasource.Querier

	// state stores recent state changes
	// during evaluations
	state *ruleState

	metrics *recordingRuleMetrics
}

type recordingRuleMetrics struct {
	errors  *utils.Counter
	samples *utils.Gauge
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

// NewRecordingRule creates a new RecordingRule
func NewRecordingRule(qb datasource.QuerierBuilder, group *Group, cfg config.Rule) *RecordingRule {
	rr := &RecordingRule{
		Type:      group.Type,
		RuleID:    cfg.ID,
		Name:      cfg.Record,
		Expr:      cfg.Expr,
		Labels:    cfg.Labels,
		GroupID:   group.ID(),
		GroupName: group.Name,
		File:      group.File,
		metrics:   &recordingRuleMetrics{},
		q: qb.BuildWithParams(datasource.QuerierParams{
			DataSourceType:     group.Type.String(),
			EvaluationInterval: group.Interval,
			QueryParams:        group.Params,
			Headers:            group.Headers,
		}),
	}

	entrySize := *ruleUpdateEntriesLimit
	if cfg.UpdateEntriesLimit != nil {
		entrySize = *cfg.UpdateEntriesLimit
	}
	if entrySize < 1 {
		entrySize = 1
	}
	rr.state = &ruleState{
		entries: make([]StateEntry, entrySize),
	}

	labels := fmt.Sprintf(`recording=%q, group=%q, file=%q, id="%d"`, rr.Name, group.Name, group.File, rr.ID())
	rr.metrics.errors = utils.GetOrCreateCounter(fmt.Sprintf(`vmalert_recording_rules_errors_total{%s}`, labels))
	rr.metrics.samples = utils.GetOrCreateGauge(fmt.Sprintf(`vmalert_recording_rules_last_evaluation_samples{%s}`, labels),
		func() float64 {
			e := rr.state.getLast()
			return float64(e.Samples)
		})
	return rr
}

// close unregisters rule metrics
func (rr *RecordingRule) close() {
	rr.metrics.errors.Unregister()
	rr.metrics.samples.Unregister()
}

// execRange executes recording rule on the given time range similarly to Exec.
// It doesn't update internal states of the Rule and meant to be used just
// to get time series for backfilling.
func (rr *RecordingRule) execRange(ctx context.Context, start, end time.Time) ([]prompbmarshal.TimeSeries, error) {
	res, err := rr.q.QueryRange(ctx, rr.Expr, start, end)
	if err != nil {
		return nil, err
	}
	duplicates := make(map[string]struct{}, len(res.Data))
	var tss []prompbmarshal.TimeSeries
	for _, s := range res.Data {
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

// exec executes RecordingRule expression via the given Querier.
func (rr *RecordingRule) exec(ctx context.Context, ts time.Time, limit int) ([]prompbmarshal.TimeSeries, error) {
	start := time.Now()
	res, req, err := rr.q.Query(ctx, rr.Expr, ts)
	curState := StateEntry{
		Time:          start,
		At:            ts,
		Duration:      time.Since(start),
		Samples:       len(res.Data),
		SeriesFetched: res.SeriesFetched,
		Curl:          requestToCurl(req),
	}

	defer func() {
		rr.state.add(curState)
		if curState.Err != nil {
			rr.metrics.errors.Inc()
		}
	}()

	if err != nil {
		curState.Err = fmt.Errorf("failed to execute query %q: %w", rr.Expr, err)
		return nil, curState.Err
	}

	qMetrics := res.Data
	numSeries := len(qMetrics)
	if limit > 0 && numSeries > limit {
		curState.Err = fmt.Errorf("exec exceeded limit of %d with %d series", limit, numSeries)
		return nil, curState.Err
	}

	duplicates := make(map[string]struct{}, len(qMetrics))
	var tss []prompbmarshal.TimeSeries
	for _, r := range qMetrics {
		ts := rr.toTimeSeries(r)
		key := stringifyLabels(ts)
		if _, ok := duplicates[key]; ok {
			curState.Err = fmt.Errorf("original metric %v; resulting labels %q: %w", r, key, errDuplicate)
			return nil, curState.Err
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
		if _, ok := labels[k]; ok && labels[k] != v {
			labels[fmt.Sprintf("exported_%s", k)] = labels[k]
		}
		labels[k] = v
	}
	return newTimeSeries(m.Values, m.Timestamps, labels)
}

// updateWith copies all significant fields.
func (rr *RecordingRule) updateWith(r Rule) error {
	nr, ok := r.(*RecordingRule)
	if !ok {
		return fmt.Errorf("BUG: attempt to update recording rule with wrong type %#v", r)
	}
	rr.Expr = nr.Expr
	rr.Labels = nr.Labels
	rr.q = nr.q
	return nil
}
