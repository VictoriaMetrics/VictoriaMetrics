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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
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
	errors  *gauge
	pending *gauge
	active  *gauge
	samples *gauge
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
	ar.metrics.pending = getOrCreateGauge(fmt.Sprintf(`vmalert_alerts_pending{%s}`, labels),
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
	ar.metrics.active = getOrCreateGauge(fmt.Sprintf(`vmalert_alerts_firing{%s}`, labels),
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
	ar.metrics.errors = getOrCreateGauge(fmt.Sprintf(`vmalert_alerting_rules_error{%s}`, labels),
		func() float64 {
			ar.mu.RLock()
			defer ar.mu.RUnlock()
			if ar.lastExecError == nil {
				return 0
			}
			return 1
		})
	ar.metrics.samples = getOrCreateGauge(fmt.Sprintf(`vmalert_alerting_rules_last_evaluation_samples{%s}`, labels),
		func() float64 {
			ar.mu.RLock()
			defer ar.mu.RUnlock()
			return float64(ar.lastExecSamples)
		})
	return ar
}

// Close unregisters rule metrics
func (ar *AlertingRule) Close() {
	metrics.UnregisterMetric(ar.metrics.active.name)
	metrics.UnregisterMetric(ar.metrics.pending.name)
	metrics.UnregisterMetric(ar.metrics.errors.name)
	metrics.UnregisterMetric(ar.metrics.samples.name)
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
		// set additional labels to identify group and rule name
		if ar.Name != "" {
			s.SetLabel(alertNameLabel, ar.Name)
		}
		if !*disableAlertGroupLabel && ar.GroupName != "" {
			s.SetLabel(alertGroupNameLabel, ar.GroupName)
		}
		// extra labels could contain templates, so we expand them first
		labels, err := expandLabels(s, qFn, ar)
		if err != nil {
			return nil, fmt.Errorf("failed to expand labels: %s", err)
		}
		for k, v := range labels {
			// apply extra labels to datasource
			// so the hash key will be consistent on restore
			s.SetLabel(k, v)
		}
		a, err := ar.newAlert(s, time.Time{}, qFn) // initial alert
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
				a.Start = at
			} else if at.Sub(a.Start) >= ar.For {
				a.State = notifier.StateFiring
			}
			prevT = at
			result = append(result, ar.alertToTimeSeries(a, s.Timestamps[i])...)
		}
	}
	return result, nil
}

// Exec executes AlertingRule expression via the given Querier.
// Based on the Querier results AlertingRule maintains notifier.Alerts
func (ar *AlertingRule) Exec(ctx context.Context) ([]prompbmarshal.TimeSeries, error) {
	qMetrics, err := ar.q.Query(ctx, ar.Expr)
	ar.mu.Lock()
	defer ar.mu.Unlock()

	ar.lastExecError = err
	ar.lastExecTime = time.Now()
	ar.lastExecSamples = len(qMetrics)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query %q: %w", ar.Expr, err)
	}

	for h, a := range ar.alerts {
		// cleanup inactive alerts from previous Exec
		if a.State == notifier.StateInactive {
			delete(ar.alerts, h)
		}
	}

	qFn := func(query string) ([]datasource.Metric, error) { return ar.q.Query(ctx, query) }
	updated := make(map[uint64]struct{})
	// update list of active alerts
	for _, m := range qMetrics {
		// set additional labels to identify group and rule name
		if ar.Name != "" {
			m.SetLabel(alertNameLabel, ar.Name)
		}
		if !*disableAlertGroupLabel && ar.GroupName != "" {
			m.SetLabel(alertGroupNameLabel, ar.GroupName)
		}
		// extra labels could contain templates, so we expand them first
		labels, err := expandLabels(m, qFn, ar)
		if err != nil {
			return nil, fmt.Errorf("failed to expand labels: %s", err)
		}
		for k, v := range labels {
			// apply extra labels to datasource
			// so the hash key will be consistent on restore
			m.SetLabel(k, v)
		}
		h := hash(m)
		if _, ok := updated[h]; ok {
			// duplicate may be caused by extra labels
			// conflicting with the metric labels
			return nil, fmt.Errorf("labels %v: %w", m.Labels, errDuplicate)
		}
		updated[h] = struct{}{}
		if a, ok := ar.alerts[h]; ok {
			if a.Value != m.Values[0] {
				// update Value field with latest value
				a.Value = m.Values[0]
				// and re-exec template since Value can be used
				// in annotations
				a.Annotations, err = a.ExecTemplate(qFn, ar.Annotations)
				if err != nil {
					return nil, err
				}
			}
			continue
		}
		a, err := ar.newAlert(m, ar.lastExecTime, qFn)
		if err != nil {
			ar.lastExecError = err
			return nil, fmt.Errorf("failed to create alert: %w", err)
		}
		a.ID = h
		a.State = notifier.StatePending
		ar.alerts[h] = a
	}

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
			a.State = notifier.StateInactive
			continue
		}
		if a.State == notifier.StatePending && time.Since(a.Start) >= ar.For {
			a.State = notifier.StateFiring
			alertsFired.Inc()
		}
	}
	return ar.toTimeSeries(ar.lastExecTime.Unix()), nil
}

func expandLabels(m datasource.Metric, q notifier.QueryFn, ar *AlertingRule) (map[string]string, error) {
	metricLabels := make(map[string]string)
	for _, l := range m.Labels {
		metricLabels[l.Name] = l.Value
	}
	tpl := notifier.AlertTplData{
		Labels: metricLabels,
		Value:  m.Values[0],
		Expr:   ar.Expr,
	}
	return notifier.ExecTemplate(q, ar.Labels, tpl)
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
func hash(m datasource.Metric) uint64 {
	hash := fnv.New64a()
	labels := m.Labels
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})
	for _, l := range labels {
		// drop __name__ to be consistent with Prometheus alerting
		if l.Name == "__name__" {
			continue
		}
		hash.Write([]byte(l.Name))
		hash.Write([]byte(l.Value))
		hash.Write([]byte("\xff"))
	}
	return hash.Sum64()
}

func (ar *AlertingRule) newAlert(m datasource.Metric, start time.Time, qFn notifier.QueryFn) (*notifier.Alert, error) {
	a := &notifier.Alert{
		GroupID: ar.GroupID,
		Name:    ar.Name,
		Labels:  map[string]string{},
		Value:   m.Values[0],
		Start:   start,
		Expr:    ar.Expr,
	}
	for _, l := range m.Labels {
		// drop __name__ to be consistent with Prometheus alerting
		if l.Name == "__name__" {
			continue
		}
		a.Labels[l.Name] = l.Value
	}
	var err error
	a.Annotations, err = a.ExecTemplate(qFn, ar.Annotations)
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

// RuleAPI returns Rule representation in form
// of APIAlertingRule
func (ar *AlertingRule) RuleAPI() APIAlertingRule {
	var lastErr string
	if ar.lastExecError != nil {
		lastErr = ar.lastExecError.Error()
	}
	return APIAlertingRule{
		// encode as strings to avoid rounding
		ID:          fmt.Sprintf("%d", ar.ID()),
		GroupID:     fmt.Sprintf("%d", ar.GroupID),
		Type:        ar.Type.String(),
		Name:        ar.Name,
		Expression:  ar.Expr,
		For:         ar.For.String(),
		LastError:   lastErr,
		LastSamples: ar.lastExecSamples,
		LastExec:    ar.lastExecTime,
		Labels:      ar.Labels,
		Annotations: ar.Annotations,
	}
}

// AlertsAPI generates list of APIAlert objects from existing alerts
func (ar *AlertingRule) AlertsAPI() []*APIAlert {
	var alerts []*APIAlert
	ar.mu.RLock()
	for _, a := range ar.alerts {
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
		ActiveAt:    a.Start,
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

// alertToTimeSeries converts the given alert with the given timestamp to timeseries
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
	return newTimeSeries([]float64{float64(a.Start.Unix())}, []int64{timestamp}, labels)
}

// Restore restores the state of active alerts basing on previously written time series.
// Restore restores only Start field. Field State will be always Pending and supposed
// to be updated on next Exec, as well as Value field.
// Only rules with For > 0 will be restored.
func (ar *AlertingRule) Restore(ctx context.Context, q datasource.Querier, lookback time.Duration, labels map[string]string) error {
	if q == nil {
		return fmt.Errorf("querier is nil")
	}

	qFn := func(query string) ([]datasource.Metric, error) { return ar.q.Query(ctx, query) }

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
	qMetrics, err := q.Query(ctx, expr)
	if err != nil {
		return err
	}

	for _, m := range qMetrics {
		a, err := ar.newAlert(m, time.Unix(int64(m.Values[0]), 0), qFn)
		if err != nil {
			return fmt.Errorf("failed to create alert: %w", err)
		}
		a.ID = hash(m)
		a.State = notifier.StatePending
		a.Restored = true
		ar.alerts[a.ID] = a
		logger.Infof("alert %q (%d) restored to state at %v", a.Name, a.ID, a.Start)
	}
	return nil
}
