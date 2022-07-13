package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// AlertingRule is basic alert entity
type AlertingRule struct {
	Type         datasource.Type
	RuleID       uint64
	Name         string
	Expr         string
	For          time.Duration
	Labels       map[string]string
	Annotations  map[string]string
	GroupID      uint64
	GroupName    string
	EvalInterval time.Duration

	q datasource.Querier

	// guard status fields
	mu sync.RWMutex
	// stores list of active alerts
	alerts map[uint64]*notifier.Alert
	// stores last moment of time Exec was called
	lastExecTime time.Time
	// stores the duration of the last Exec call
	lastExecDuration time.Duration
	// stores last error that happened in Exec func
	// resets on every successful Exec
	// may be used as Health state
	lastExecError error
	// stores the number of samples returned during
	// the last evaluation
	lastExecSamples int

	metrics *alertingRuleMetrics
}

type alertingRuleMetrics struct {
	errors  *utils.Gauge
	pending *utils.Gauge
	active  *utils.Gauge
	samples *utils.Gauge
}

func newAlertingRule(qb datasource.QuerierBuilder, group *Group, cfg config.Rule) *AlertingRule {
	ar := &AlertingRule{
		Type:         group.Type,
		RuleID:       cfg.ID,
		Name:         cfg.Alert,
		Expr:         cfg.Expr,
		For:          cfg.For.Duration(),
		Labels:       cfg.Labels,
		Annotations:  cfg.Annotations,
		GroupID:      group.ID(),
		GroupName:    group.Name,
		EvalInterval: group.Interval,
		q: qb.BuildWithParams(datasource.QuerierParams{
			DataSourceType:     &group.Type,
			EvaluationInterval: group.Interval,
			QueryParams:        group.Params,
		}),
		alerts:  make(map[uint64]*notifier.Alert),
		metrics: &alertingRuleMetrics{},
	}

	labels := fmt.Sprintf(`alertname=%q, group=%q, id="%d"`, ar.Name, group.Name, ar.ID())
	ar.metrics.pending = utils.GetOrCreateGauge(fmt.Sprintf(`vmalert_alerts_pending{%s}`, labels),
		func() float64 {
			ar.mu.RLock()
			defer ar.mu.RUnlock()
			var num int
			for _, a := range ar.alerts {
				if a.State == notifier.StatePending {
					num++
				}
			}
			return float64(num)
		})
	ar.metrics.active = utils.GetOrCreateGauge(fmt.Sprintf(`vmalert_alerts_firing{%s}`, labels),
		func() float64 {
			ar.mu.RLock()
			defer ar.mu.RUnlock()
			var num int
			for _, a := range ar.alerts {
				if a.State == notifier.StateFiring {
					num++
				}
			}
			return float64(num)
		})
	ar.metrics.errors = utils.GetOrCreateGauge(fmt.Sprintf(`vmalert_alerting_rules_error{%s}`, labels),
		func() float64 {
			ar.mu.RLock()
			defer ar.mu.RUnlock()
			if ar.lastExecError == nil {
				return 0
			}
			return 1
		})
	ar.metrics.samples = utils.GetOrCreateGauge(fmt.Sprintf(`vmalert_alerting_rules_last_evaluation_samples{%s}`, labels),
		func() float64 {
			ar.mu.RLock()
			defer ar.mu.RUnlock()
			return float64(ar.lastExecSamples)
		})
	return ar
}

// Close unregisters rule metrics
func (ar *AlertingRule) Close() {
	ar.metrics.active.Unregister()
	ar.metrics.pending.Unregister()
	ar.metrics.errors.Unregister()
	ar.metrics.samples.Unregister()
}

// String implements Stringer interface
func (ar *AlertingRule) String() string {
	return ar.Name
}

// ID returns unique Rule ID
// within the parent Group.
func (ar *AlertingRule) ID() uint64 {
	return ar.RuleID
}

type labelSet struct {
	// origin labels from series
	// used for templating
	origin map[string]string
	// processed labels with additional data
	// used as Alert labels
	processed map[string]string
}

// toLabels converts labels from given Metric
// to labelSet which contains original and processed labels.
func (ar *AlertingRule) toLabels(m datasource.Metric, qFn templates.QueryFn) (*labelSet, error) {
	ls := &labelSet{
		origin:    make(map[string]string, len(m.Labels)),
		processed: make(map[string]string),
	}
	for _, l := range m.Labels {
		ls.origin[l.Name] = l.Value
		// drop __name__ to be consistent with Prometheus alerting
		if l.Name == "__name__" {
			continue
		}
		ls.processed[l.Name] = l.Value
	}

	extraLabels, err := notifier.ExecTemplate(qFn, ar.Labels, notifier.AlertTplData{
		Labels: ls.origin,
		Value:  m.Values[0],
		Expr:   ar.Expr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to expand labels: %s", err)
	}
	for k, v := range extraLabels {
		ls.processed[k] = v
	}

	// set additional labels to identify group and rule name
	if ar.Name != "" {
		ls.processed[alertNameLabel] = ar.Name
	}
	if !*disableAlertGroupLabel && ar.GroupName != "" {
		ls.processed[alertGroupNameLabel] = ar.GroupName
	}
	return ls, nil
}

// ExecRange executes alerting rule on the given time range similarly to Exec.
// It doesn't update internal states of the Rule and meant to be used just
// to get time series for backfilling.
// It returns ALERT and ALERT_FOR_STATE time series as result.
func (ar *AlertingRule) ExecRange(ctx context.Context, start, end time.Time) ([]prompbmarshal.TimeSeries, error) {
	series, err := ar.q.QueryRange(ctx, ar.Expr, start, end)
	if err != nil {
		return nil, err
	}
	var result []prompbmarshal.TimeSeries
	qFn := func(query string) ([]datasource.Metric, error) {
		return nil, fmt.Errorf("`query` template isn't supported in replay mode")
	}
	for _, s := range series {
		a, err := ar.newAlert(s, nil, time.Time{}, qFn) // initial alert
		if err != nil {
			return nil, fmt.Errorf("failed to create alert: %s", err)
		}
		if ar.For == 0 { // if alert is instant
			a.State = notifier.StateFiring
			for i := range s.Values {
				result = append(result, ar.alertToTimeSeries(a, s.Timestamps[i])...)
			}
			continue
		}

		// if alert with For > 0
		prevT := time.Time{}
		for i := range s.Values {
			at := time.Unix(s.Timestamps[i], 0)
			if at.Sub(prevT) > ar.EvalInterval {
				// reset to Pending if there are gaps > EvalInterval between DPs
				a.State = notifier.StatePending
				a.ActiveAt = at
			} else if at.Sub(a.ActiveAt) >= ar.For {
				a.State = notifier.StateFiring
				a.Start = at
			}
			prevT = at
			result = append(result, ar.alertToTimeSeries(a, s.Timestamps[i])...)
		}
	}
	return result, nil
}

// resolvedRetention is the duration for which a resolved alert instance
// is kept in memory state and consequently repeatedly sent to the AlertManager.
const resolvedRetention = 15 * time.Minute

// Exec executes AlertingRule expression via the given Querier.
// Based on the Querier results AlertingRule maintains notifier.Alerts
func (ar *AlertingRule) Exec(ctx context.Context, ts time.Time, limit int) ([]prompbmarshal.TimeSeries, error) {
	start := time.Now()
	qMetrics, err := ar.q.Query(ctx, ar.Expr, ts)
	ar.mu.Lock()
	defer ar.mu.Unlock()

	ar.lastExecTime = start
	ar.lastExecDuration = time.Since(start)
	ar.lastExecError = err
	ar.lastExecSamples = len(qMetrics)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query %q: %w", ar.Expr, err)
	}

	for h, a := range ar.alerts {
		// cleanup inactive alerts from previous Exec
		if a.State == notifier.StateInactive && ts.Sub(a.ResolvedAt) > resolvedRetention {
			delete(ar.alerts, h)
		}
	}

	qFn := func(query string) ([]datasource.Metric, error) { return ar.q.Query(ctx, query, ts) }
	updated := make(map[uint64]struct{})
	// update list of active alerts
	for _, m := range qMetrics {
		ls, err := ar.toLabels(m, qFn)
		if err != nil {
			return nil, fmt.Errorf("failed to expand labels: %s", err)
		}
		h := hash(ls.processed)
		if _, ok := updated[h]; ok {
			// duplicate may be caused by extra labels
			// conflicting with the metric labels
			ar.lastExecError = fmt.Errorf("labels %v: %w", ls.processed, errDuplicate)
			return nil, ar.lastExecError
		}
		updated[h] = struct{}{}
		if a, ok := ar.alerts[h]; ok {
			if a.State == notifier.StateInactive {
				// alert could be in inactive state for resolvedRetention
				// so when we again receive metrics for it - we switch it
				// back to notifier.StatePending
				a.State = notifier.StatePending
				a.ActiveAt = ts
			}
			if a.Value != m.Values[0] {
				// update Value field with latest value
				a.Value = m.Values[0]
				// and re-exec template since Value can be used
				// in annotations
				a.Annotations, err = a.ExecTemplate(qFn, ls.origin, ar.Annotations)
				if err != nil {
					return nil, err
				}
			}
			continue
		}
		a, err := ar.newAlert(m, ls, ar.lastExecTime, qFn)
		if err != nil {
			ar.lastExecError = err
			return nil, fmt.Errorf("failed to create alert: %w", err)
		}
		a.ID = h
		a.State = notifier.StatePending
		a.ActiveAt = ts
		ar.alerts[h] = a
	}
	var numActivePending int
	for h, a := range ar.alerts {
		// if alert wasn't updated in this iteration
		// means it is resolved already
		if _, ok := updated[h]; !ok {
			if a.State == notifier.StatePending {
				// alert was in Pending state - it is not
				// active anymore
				delete(ar.alerts, h)
				continue
			}
			if a.State == notifier.StateFiring {
				a.State = notifier.StateInactive
				a.ResolvedAt = ts
			}
			continue
		}
		numActivePending++
		if a.State == notifier.StatePending && ts.Sub(a.ActiveAt) >= ar.For {
			a.State = notifier.StateFiring
			a.Start = ts
			alertsFired.Inc()
		}
	}
	if limit > 0 && numActivePending > limit {
		ar.alerts = map[uint64]*notifier.Alert{}
		return nil, fmt.Errorf("exec exceeded limit of %d with %d alerts", limit, numActivePending)
	}
	return ar.toTimeSeries(ts.Unix()), nil
}

func (ar *AlertingRule) toTimeSeries(timestamp int64) []prompbmarshal.TimeSeries {
	var tss []prompbmarshal.TimeSeries
	for _, a := range ar.alerts {
		if a.State == notifier.StateInactive {
			continue
		}
		ts := ar.alertToTimeSeries(a, timestamp)
		tss = append(tss, ts...)
	}
	return tss
}

// UpdateWith copies all significant fields.
// alerts state isn't copied since
// it should be updated in next 2 Execs
func (ar *AlertingRule) UpdateWith(r Rule) error {
	nr, ok := r.(*AlertingRule)
	if !ok {
		return fmt.Errorf("BUG: attempt to update alerting rule with wrong type %#v", r)
	}
	ar.Expr = nr.Expr
	ar.For = nr.For
	ar.Labels = nr.Labels
	ar.Annotations = nr.Annotations
	ar.EvalInterval = nr.EvalInterval
	ar.q = nr.q
	return nil
}

// TODO: consider hashing algorithm in VM
func hash(labels map[string]string) uint64 {
	hash := fnv.New64a()
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		// drop __name__ to be consistent with Prometheus alerting
		if k == "__name__" {
			continue
		}
		name, value := k, labels[k]
		hash.Write([]byte(name))
		hash.Write([]byte(value))
		hash.Write([]byte("\xff"))
	}
	return hash.Sum64()
}

func (ar *AlertingRule) newAlert(m datasource.Metric, ls *labelSet, start time.Time, qFn templates.QueryFn) (*notifier.Alert, error) {
	var err error
	if ls == nil {
		ls, err = ar.toLabels(m, qFn)
		if err != nil {
			return nil, fmt.Errorf("failed to expand labels: %s", err)
		}
	}
	a := &notifier.Alert{
		GroupID:  ar.GroupID,
		Name:     ar.Name,
		Labels:   ls.processed,
		Value:    m.Values[0],
		ActiveAt: start,
		Expr:     ar.Expr,
	}
	a.Annotations, err = a.ExecTemplate(qFn, ls.origin, ar.Annotations)
	return a, err
}

// AlertAPI generates APIAlert object from alert by its id(hash)
func (ar *AlertingRule) AlertAPI(id uint64) *APIAlert {
	ar.mu.RLock()
	defer ar.mu.RUnlock()
	a, ok := ar.alerts[id]
	if !ok {
		return nil
	}
	return ar.newAlertAPI(*a)
}

// ToAPI returns Rule representation in form
// of APIRule
func (ar *AlertingRule) ToAPI() APIRule {
	r := APIRule{
		Type:           "alerting",
		DatasourceType: ar.Type.String(),
		Name:           ar.Name,
		Query:          ar.Expr,
		Duration:       ar.For.Seconds(),
		Labels:         ar.Labels,
		Annotations:    ar.Annotations,
		LastEvaluation: ar.lastExecTime,
		EvaluationTime: ar.lastExecDuration.Seconds(),
		Health:         "ok",
		State:          "inactive",
		Alerts:         ar.AlertsToAPI(),
		LastSamples:    ar.lastExecSamples,

		// encode as strings to avoid rounding in JSON
		ID:      fmt.Sprintf("%d", ar.ID()),
		GroupID: fmt.Sprintf("%d", ar.GroupID),
	}
	if ar.lastExecError != nil {
		r.LastError = ar.lastExecError.Error()
		r.Health = "err"
	}
	// satisfy APIRule.State logic
	if len(r.Alerts) > 0 {
		r.State = notifier.StatePending.String()
		stateFiring := notifier.StateFiring.String()
		for _, a := range r.Alerts {
			if a.State == stateFiring {
				r.State = stateFiring
				break
			}
		}
	}
	return r
}

// AlertsToAPI generates list of APIAlert objects from existing alerts
func (ar *AlertingRule) AlertsToAPI() []*APIAlert {
	var alerts []*APIAlert
	ar.mu.RLock()
	for _, a := range ar.alerts {
		if a.State == notifier.StateInactive {
			continue
		}
		alerts = append(alerts, ar.newAlertAPI(*a))
	}
	ar.mu.RUnlock()
	return alerts
}

func (ar *AlertingRule) newAlertAPI(a notifier.Alert) *APIAlert {
	aa := &APIAlert{
		// encode as strings to avoid rounding
		ID:      fmt.Sprintf("%d", a.ID),
		GroupID: fmt.Sprintf("%d", a.GroupID),
		RuleID:  fmt.Sprintf("%d", ar.RuleID),

		Name:        a.Name,
		Expression:  ar.Expr,
		Labels:      a.Labels,
		Annotations: a.Annotations,
		State:       a.State.String(),
		ActiveAt:    a.ActiveAt,
		Restored:    a.Restored,
		Value:       strconv.FormatFloat(a.Value, 'f', -1, 32),
	}
	if alertURLGeneratorFn != nil {
		aa.SourceLink = alertURLGeneratorFn(a)
	}
	return aa
}

const (
	// alertMetricName is the metric name for synthetic alert timeseries.
	alertMetricName = "ALERTS"
	// alertForStateMetricName is the metric name for 'for' state of alert.
	alertForStateMetricName = "ALERTS_FOR_STATE"

	// alertNameLabel is the label name indicating the name of an alert.
	alertNameLabel = "alertname"
	// alertStateLabel is the label name indicating the state of an alert.
	alertStateLabel = "alertstate"

	// alertGroupNameLabel defines the label name attached for generated time series.
	// attaching this label may be disabled via `-disableAlertgroupLabel` flag.
	alertGroupNameLabel = "alertgroup"
)

// alertToTimeSeries converts the given alert with the given timestamp to time series
func (ar *AlertingRule) alertToTimeSeries(a *notifier.Alert, timestamp int64) []prompbmarshal.TimeSeries {
	var tss []prompbmarshal.TimeSeries
	tss = append(tss, alertToTimeSeries(a, timestamp))
	if ar.For > 0 {
		tss = append(tss, alertForToTimeSeries(a, timestamp))
	}
	return tss
}

func alertToTimeSeries(a *notifier.Alert, timestamp int64) prompbmarshal.TimeSeries {
	labels := make(map[string]string)
	for k, v := range a.Labels {
		labels[k] = v
	}
	labels["__name__"] = alertMetricName
	labels[alertStateLabel] = a.State.String()
	return newTimeSeries([]float64{1}, []int64{timestamp}, labels)
}

// alertForToTimeSeries returns a timeseries that represents
// state of active alerts, where value is time when alert become active
func alertForToTimeSeries(a *notifier.Alert, timestamp int64) prompbmarshal.TimeSeries {
	labels := make(map[string]string)
	for k, v := range a.Labels {
		labels[k] = v
	}
	labels["__name__"] = alertForStateMetricName
	return newTimeSeries([]float64{float64(a.ActiveAt.Unix())}, []int64{timestamp}, labels)
}

// Restore restores the state of active alerts basing on previously written time series.
// Restore restores only ActiveAt field. Field State will be always Pending and supposed
// to be updated on next Exec, as well as Value field.
// Only rules with For > 0 will be restored.
func (ar *AlertingRule) Restore(ctx context.Context, q datasource.Querier, lookback time.Duration, labels map[string]string) error {
	if q == nil {
		return fmt.Errorf("querier is nil")
	}

	ts := time.Now()
	qFn := func(query string) ([]datasource.Metric, error) { return ar.q.Query(ctx, query, ts) }

	// account for external labels in filter
	var labelsFilter string
	for k, v := range labels {
		labelsFilter += fmt.Sprintf(",%s=%q", k, v)
	}

	// Get the last data point in range via MetricsQL `last_over_time`.
	// We don't use plain PromQL since Prometheus doesn't support
	// remote write protocol which is used for state persistence in vmalert.
	expr := fmt.Sprintf("last_over_time(%s{alertname=%q%s}[%ds])",
		alertForStateMetricName, ar.Name, labelsFilter, int(lookback.Seconds()))
	qMetrics, err := q.Query(ctx, expr, ts)
	if err != nil {
		return err
	}

	for _, m := range qMetrics {
		ls := &labelSet{
			origin:    make(map[string]string, len(m.Labels)),
			processed: make(map[string]string, len(m.Labels)),
		}
		for _, l := range m.Labels {
			if l.Name == "__name__" {
				continue
			}
			ls.origin[l.Name] = l.Value
			ls.processed[l.Name] = l.Value
		}
		a, err := ar.newAlert(m, ls, time.Unix(int64(m.Values[0]), 0), qFn)
		if err != nil {
			return fmt.Errorf("failed to create alert: %w", err)
		}
		a.ID = hash(ls.processed)
		a.State = notifier.StatePending
		a.Restored = true
		ar.alerts[a.ID] = a
		logger.Infof("alert %q (%d) restored to state at %v", a.Name, a.ID, a.ActiveAt)
	}
	return nil
}

// alertsToSend walks through the current alerts of AlertingRule
// and returns only those which should be sent to notifier.
// Isn't concurrent safe.
func (ar *AlertingRule) alertsToSend(ts time.Time, resolveDuration, resendDelay time.Duration) []notifier.Alert {
	needsSending := func(a *notifier.Alert) bool {
		if a.State == notifier.StatePending {
			return false
		}
		if a.ResolvedAt.After(a.LastSent) {
			return true
		}
		return a.LastSent.Add(resendDelay).Before(ts)
	}

	var alerts []notifier.Alert
	for _, a := range ar.alerts {
		if !needsSending(a) {
			continue
		}
		a.End = ts.Add(resolveDuration)
		if a.State == notifier.StateInactive {
			a.End = a.ResolvedAt
		}
		a.LastSent = ts
		alerts = append(alerts, *a)
	}
	return alerts
}
