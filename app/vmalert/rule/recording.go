package rule

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/vmalertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
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
	Debug     bool

	q datasource.Querier

	// state stores recent state changes
	// during evaluations
	state *ruleState

	lastEvaluation map[string]struct{}

	metrics *recordingRuleMetrics
}

type recordingRuleMetrics struct {
	errors  *vmalertutil.Counter
	samples *vmalertutil.Gauge
}

func newRecordingRuleMetrics(set *metrics.Set, rr *RecordingRule) *recordingRuleMetrics {
	rmr := &recordingRuleMetrics{}

	labels := fmt.Sprintf(`recording=%q, group=%q, file=%q, id="%d"`, rr.Name, rr.GroupName, rr.File, rr.ID())
	rmr.errors = vmalertutil.NewCounter(set, fmt.Sprintf(`vmalert_recording_rules_errors_total{%s}`, labels))
	rmr.samples = vmalertutil.NewGauge(set, fmt.Sprintf(`vmalert_recording_rules_last_evaluation_samples{%s}`, labels),
		func() float64 {
			e := rr.state.getLast()
			return float64(e.Samples)
		})

	return rmr
}

func (m *recordingRuleMetrics) close() {
	if m == nil {
		return
	}
	m.errors.Unregister()
	m.samples.Unregister()
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
		GroupID:   group.GetID(),
		GroupName: group.Name,
		File:      group.File,
		Debug:     group.Debug,
		q: qb.BuildWithParams(datasource.QuerierParams{
			DataSourceType:            group.Type.String(),
			ApplyIntervalAsTimeFilter: setIntervalAsTimeFilter(group.Type.String(), cfg.Expr),
			EvaluationInterval:        group.Interval,
			QueryParams:               group.Params,
			Headers:                   group.Headers,
		}),
	}
	if cfg.Debug != nil {
		rr.Debug = *cfg.Debug
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
	return rr
}

func (rr *RecordingRule) registerMetrics(set *metrics.Set) {
	rr.metrics = newRecordingRuleMetrics(set, rr)
}

// close unregisters rule metrics
func (rr *RecordingRule) unregisterMetrics() {
	rr.metrics.close()
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
		key := stringifyLabels(ts.Labels)
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

	rr.logDebugf(ts, "query returned %d samples (elapsed: %s, isPartial: %t)", curState.Samples, curState.Duration, isPartialResponse(res))

	qMetrics := res.Data
	numSeries := len(qMetrics)
	if limit > 0 && numSeries > limit {
		curState.Err = fmt.Errorf("exec exceeded limit of %d with %d series", limit, numSeries)
		return nil, curState.Err
	}

	curEvaluation := make(map[string]struct{}, len(qMetrics))
	lastEvaluation := rr.lastEvaluation
	var tss []prompbmarshal.TimeSeries
	for _, r := range qMetrics {
		ts := rr.toTimeSeries(r)
		key := stringifyLabels(ts.Labels)
		if _, ok := curEvaluation[key]; ok {
			curState.Err = fmt.Errorf("original metric %v; resulting labels %q: %w", r, key, errDuplicate)
			return nil, curState.Err
		}
		curEvaluation[key] = struct{}{}
		delete(lastEvaluation, key)
		tss = append(tss, ts)
	}
	// check for stale time series
	for k := range lastEvaluation {
		tss = append(tss, prompbmarshal.TimeSeries{
			Labels: stringToLabels(k),
			Samples: []prompbmarshal.Sample{
				{Value: decimal.StaleNaN, Timestamp: ts.UnixNano() / 1e6},
			}})
	}
	rr.lastEvaluation = curEvaluation
	return tss, nil
}

func (rr *RecordingRule) logDebugf(at time.Time, format string, args ...any) {
	if !rr.Debug {
		return
	}
	prefix := fmt.Sprintf("DEBUG recording rule %q, %q:%q (%d) at %v: ",
		rr.File, rr.GroupName, rr.Name, rr.RuleID, at.Format(time.RFC3339))

	msg := fmt.Sprintf(format, args...)
	logger.Infof("%s", prefix+msg)
}

func stringToLabels(s string) []prompbmarshal.Label {
	labels := strings.Split(s, ",")
	rLabels := make([]prompbmarshal.Label, 0, len(labels))
	for i := range labels {
		if label := strings.Split(labels[i], "="); len(label) == 2 {
			rLabels = append(rLabels, prompbmarshal.Label{
				Name:  label[0],
				Value: label[1],
			})
		}
	}
	return rLabels
}

func stringifyLabels(labels []prompbmarshal.Label) string {
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
	if preN := promrelabel.GetLabelByName(m.Labels, "__name__"); preN != nil {
		preN.Value = rr.Name
	} else {
		m.Labels = append(m.Labels, prompbmarshal.Label{
			Name:  "__name__",
			Value: rr.Name,
		})
	}
	for k := range rr.Labels {
		prevLabel := promrelabel.GetLabelByName(m.Labels, k)
		if prevLabel != nil && prevLabel.Value != rr.Labels[k] {
			// Rename the prevLabel to "exported_" + label.Name
			prevLabel.Name = fmt.Sprintf("exported_%s", prevLabel.Name)
		}
		m.Labels = append(m.Labels, prompbmarshal.Label{
			Name:  k,
			Value: rr.Labels[k],
		})
	}
	ts := newTimeSeries(m.Values, m.Timestamps, m.Labels)
	return ts
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

// setIntervalAsTimeFilter returns true if given LogsQL has a time filter.
func setIntervalAsTimeFilter(dType, expr string) bool {
	if dType != "vlogs" {
		return false
	}
	q, err := logstorage.ParseStatsQuery(expr, 0)
	if err != nil {
		logger.Panicf("BUG: the LogsQL query must be valid here; got error: %s; query=[%s]", err, expr)
	}
	return !q.HasGlobalTimeFilter()
}
