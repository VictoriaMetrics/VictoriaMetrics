package rule

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
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
	Type          config.Type
	RuleID        uint64
	Name          string
	Expr          string
	For           time.Duration
	KeepFiringFor time.Duration
	Labels        map[string]string
	Annotations   map[string]string
	GroupID       uint64
	GroupName     string
	File          string
	EvalInterval  time.Duration
	Debug         bool

	q datasource.Querier

	alertsMu sync.RWMutex
	// stores list of active alerts
	alerts map[uint64]*notifier.Alert

	// state stores recent state changes
	// during evaluations
	state *ruleState

	metrics *alertingRuleMetrics
}

type alertingRuleMetrics struct {
	errors        *utils.Counter
	pending       *utils.Gauge
	active        *utils.Gauge
	samples       *utils.Gauge
	seriesFetched *utils.Gauge
}

// NewAlertingRule creates a new AlertingRule
func NewAlertingRule(qb datasource.QuerierBuilder, group *Group, cfg config.Rule) *AlertingRule {
	ar := &AlertingRule{
		Type:          group.Type,
		RuleID:        cfg.ID,
		Name:          cfg.Alert,
		Expr:          cfg.Expr,
		For:           cfg.For.Duration(),
		KeepFiringFor: cfg.KeepFiringFor.Duration(),
		Labels:        cfg.Labels,
		Annotations:   cfg.Annotations,
		GroupID:       group.ID(),
		GroupName:     group.Name,
		File:          group.File,
		EvalInterval:  group.Interval,
		Debug:         cfg.Debug,
		q: qb.BuildWithParams(datasource.QuerierParams{
			DataSourceType:     group.Type.String(),
			EvaluationInterval: group.Interval,
			QueryParams:        group.Params,
			Headers:            group.Headers,
			Debug:              cfg.Debug,
		}),
		alerts:  make(map[uint64]*notifier.Alert),
		metrics: &alertingRuleMetrics{},
	}

	entrySize := *ruleUpdateEntriesLimit
	if cfg.UpdateEntriesLimit != nil {
		entrySize = *cfg.UpdateEntriesLimit
	}
	if entrySize < 1 {
		entrySize = 1
	}
	ar.state = &ruleState{
		entries: make([]StateEntry, entrySize),
	}

	labels := fmt.Sprintf(`alertname=%q, group=%q, file=%q, id="%d"`, ar.Name, group.Name, group.File, ar.ID())
	ar.metrics.pending = utils.GetOrCreateGauge(fmt.Sprintf(`vmalert_alerts_pending{%s}`, labels),
		func() float64 {
			ar.alertsMu.RLock()
			defer ar.alertsMu.RUnlock()
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
			ar.alertsMu.RLock()
			defer ar.alertsMu.RUnlock()
			var num int
			for _, a := range ar.alerts {
				if a.State == notifier.StateFiring {
					num++
				}
			}
			return float64(num)
		})
	ar.metrics.errors = utils.GetOrCreateCounter(fmt.Sprintf(`vmalert_alerting_rules_errors_total{%s}`, labels))
	ar.metrics.samples = utils.GetOrCreateGauge(fmt.Sprintf(`vmalert_alerting_rules_last_evaluation_samples{%s}`, labels),
		func() float64 {
			e := ar.state.getLast()
			return float64(e.Samples)
		})
	ar.metrics.seriesFetched = utils.GetOrCreateGauge(fmt.Sprintf(`vmalert_alerting_rules_last_evaluation_series_fetched{%s}`, labels),
		func() float64 {
			e := ar.state.getLast()
			if e.SeriesFetched == nil {
				// means seriesFetched is unsupported
				return -1
			}
			seriesFetched := float64(*e.SeriesFetched)
			if seriesFetched == 0 && e.Samples > 0 {
				// `alert: 0.95` will fetch no series
				// but will get one time series in response.
				seriesFetched = float64(e.Samples)
			}
			return seriesFetched
		})
	return ar
}

// close unregisters rule metrics
func (ar *AlertingRule) close() {
	ar.metrics.active.Unregister()
	ar.metrics.pending.Unregister()
	ar.metrics.errors.Unregister()
	ar.metrics.samples.Unregister()
	ar.metrics.seriesFetched.Unregister()
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

// GetAlerts returns active alerts of rule
func (ar *AlertingRule) GetAlerts() []*notifier.Alert {
	ar.alertsMu.RLock()
	defer ar.alertsMu.RUnlock()
	var alerts []*notifier.Alert
	for _, a := range ar.alerts {
		alerts = append(alerts, a)
	}
	return alerts
}

// GetAlert returns alert if id exists
func (ar *AlertingRule) GetAlert(id uint64) *notifier.Alert {
	ar.alertsMu.RLock()
	defer ar.alertsMu.RUnlock()
	if ar.alerts == nil {
		return nil
	}
	return ar.alerts[id]
}

func (ar *AlertingRule) logDebugf(at time.Time, a *notifier.Alert, format string, args ...interface{}) {
	if !ar.Debug {
		return
	}
	prefix := fmt.Sprintf("DEBUG rule %q:%q (%d) at %v: ",
		ar.GroupName, ar.Name, ar.RuleID, at.Format(time.RFC3339))

	if a != nil {
		labelKeys := make([]string, len(a.Labels))
		var i int
		for k := range a.Labels {
			labelKeys[i] = k
			i++
		}
		sort.Strings(labelKeys)
		labels := make([]string, len(labelKeys))
		for i, l := range labelKeys {
			labels[i] = fmt.Sprintf("%s=%q", l, a.Labels[l])
		}
		labelsStr := strings.Join(labels, ",")
		prefix += fmt.Sprintf("alert %d {%s} ", a.ID, labelsStr)
	}
	msg := fmt.Sprintf(format, args...)
	logger.Infof("%s", prefix+msg)
}

// updateWith copies all significant fields.
// alerts state isn't copied since
// it should be updated in next 2 Execs
func (ar *AlertingRule) updateWith(r Rule) error {
	nr, ok := r.(*AlertingRule)
	if !ok {
		return fmt.Errorf("BUG: attempt to update alerting rule with wrong type %#v", r)
	}
	ar.Expr = nr.Expr
	ar.For = nr.For
	ar.KeepFiringFor = nr.KeepFiringFor
	ar.Labels = nr.Labels
	ar.Annotations = nr.Annotations
	ar.EvalInterval = nr.EvalInterval
	ar.Debug = nr.Debug
	ar.q = nr.q
	ar.state = nr.state
	return nil
}

type labelSet struct {
	// origin labels extracted from received time series
	// plus extra labels (group labels, service labels like alertNameLabel).
	// in case of conflicts, origin labels from time series preferred.
	// used for templating annotations
	origin map[string]string
	// processed labels includes origin labels
	// plus extra labels (group labels, service labels like alertNameLabel).
	// in case of key conflicts, origin labels are renamed with prefix `exported_` and extra labels are preferred.
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5161
	// used as labels attached to notifier.Alert and ALERTS series written to remote storage.
	processed map[string]string
}

// add adds a value v with key k to origin and processed label sets.
// On k conflicts in processed set, the passed v is preferred.
// On k conflicts in origin set, the original value is preferred and copied
// to processed with `exported_%k` key. The copy happens only if passed v isn't equal to origin[k] value.
func (ls *labelSet) add(k, v string) {
	ls.processed[k] = v
	ov, ok := ls.origin[k]
	if !ok {
		ls.origin[k] = v
		return
	}
	if ov != v {
		// copy value only if v and ov are different
		key := fmt.Sprintf("exported_%s", k)
		ls.processed[key] = ov
	}
}

// toLabels converts labels from given Metric
// to labelSet which contains original and processed labels.
func (ar *AlertingRule) toLabels(m datasource.Metric, qFn templates.QueryFn) (*labelSet, error) {
	ls := &labelSet{
		origin:    make(map[string]string),
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
		return nil, fmt.Errorf("failed to expand labels: %w", err)
	}
	for k, v := range extraLabels {
		ls.add(k, v)
	}
	// set additional labels to identify group and rule name
	if ar.Name != "" {
		ls.add(alertNameLabel, ar.Name)
	}
	if !*disableAlertGroupLabel && ar.GroupName != "" {
		ls.add(alertGroupNameLabel, ar.GroupName)
	}
	return ls, nil
}

// execRange executes alerting rule on the given time range similarly to exec.
// When making consecutive calls make sure to respect time linearity for start and end params,
// as this function modifies AlertingRule alerts state.
// It is not thread safe.
// It returns ALERT and ALERT_FOR_STATE time series as a result.
func (ar *AlertingRule) execRange(ctx context.Context, start, end time.Time) ([]prompbmarshal.TimeSeries, error) {
	res, err := ar.q.QueryRange(ctx, ar.Expr, start, end)
	if err != nil {
		return nil, err
	}
	var result []prompbmarshal.TimeSeries
	holdAlertState := make(map[uint64]*notifier.Alert)
	qFn := func(query string) ([]datasource.Metric, error) {
		return nil, fmt.Errorf("`query` template isn't supported in replay mode")
	}
	for _, s := range res.Data {
		ls, err := ar.toLabels(s, qFn)
		if err != nil {
			return nil, fmt.Errorf("failed to expand labels: %s", err)
		}
		h := hash(ls.processed)
		a, err := ar.newAlert(s, nil, time.Time{}, qFn) // initial alert
		if err != nil {
			return nil, fmt.Errorf("failed to create alert: %w", err)
		}

		// if alert is instant, For: 0
		if ar.For == 0 {
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
			// try to restore alert's state on the first iteration
			if at.Equal(start) {
				if _, ok := ar.alerts[h]; ok {
					a = ar.alerts[h]
					prevT = at
				}
			}
			if at.Sub(prevT) > ar.EvalInterval {
				// reset to Pending if there are gaps > EvalInterval between DPs
				a.State = notifier.StatePending
				a.ActiveAt = at
				a.Start = time.Time{}
			} else if at.Sub(a.ActiveAt) >= ar.For && a.State != notifier.StateFiring {
				a.State = notifier.StateFiring
				a.Start = at
			}
			prevT = at
			result = append(result, ar.alertToTimeSeries(a, s.Timestamps[i])...)

			// save alert's state on last iteration, so it can be used on the next execRange call
			if at.Equal(end) {
				holdAlertState[h] = a
			}
		}
	}
	ar.alerts = holdAlertState
	return result, nil
}

// resolvedRetention is the duration for which a resolved alert instance
// is kept in memory state and consequently repeatedly sent to the AlertManager.
const resolvedRetention = 15 * time.Minute

// exec executes AlertingRule expression via the given Querier.
// Based on the Querier results AlertingRule maintains notifier.Alerts
func (ar *AlertingRule) exec(ctx context.Context, ts time.Time, limit int) ([]prompbmarshal.TimeSeries, error) {
	start := time.Now()
	res, req, err := ar.q.Query(ctx, ar.Expr, ts)
	curState := StateEntry{
		Time:          start,
		At:            ts,
		Duration:      time.Since(start),
		Samples:       len(res.Data),
		SeriesFetched: res.SeriesFetched,
		Err:           err,
		Curl:          requestToCurl(req),
	}

	defer func() {
		ar.state.add(curState)
		if curState.Err != nil {
			ar.metrics.errors.Inc()
		}
	}()

	ar.alertsMu.Lock()
	defer ar.alertsMu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to execute query %q: %w", ar.Expr, err)
	}

	ar.logDebugf(ts, nil, "query returned %d samples (elapsed: %s)", curState.Samples, curState.Duration)

	for h, a := range ar.alerts {
		// cleanup inactive alerts from previous Exec
		if a.State == notifier.StateInactive && ts.Sub(a.ResolvedAt) > resolvedRetention {
			ar.logDebugf(ts, a, "deleted as inactive")
			delete(ar.alerts, h)
		}
	}

	qFn := func(query string) ([]datasource.Metric, error) {
		res, _, err := ar.q.Query(ctx, query, ts)
		return res.Data, err
	}
	updated := make(map[uint64]struct{})
	// update list of active alerts
	for _, m := range res.Data {
		ls, err := ar.toLabels(m, qFn)
		if err != nil {
			curState.Err = fmt.Errorf("failed to expand labels: %w", err)
			return nil, curState.Err
		}
		h := hash(ls.processed)
		if _, ok := updated[h]; ok {
			// duplicate may be caused the removal of `__name__` label
			curState.Err = fmt.Errorf("labels %v: %w", ls.processed, errDuplicate)
			return nil, curState.Err
		}
		updated[h] = struct{}{}
		if a, ok := ar.alerts[h]; ok {
			if a.State == notifier.StateInactive {
				// alert could be in inactive state for resolvedRetention
				// so when we again receive metrics for it - we switch it
				// back to notifier.StatePending
				a.State = notifier.StatePending
				a.ActiveAt = ts
				ar.logDebugf(ts, a, "INACTIVE => PENDING")
			}
			a.Value = m.Values[0]
			// re-exec template since Value or query can be used in annotations
			a.Annotations, err = a.ExecTemplate(qFn, ls.origin, ar.Annotations)
			if err != nil {
				return nil, err
			}
			a.KeepFiringSince = time.Time{}
			continue
		}
		a, err := ar.newAlert(m, ls, start, qFn)
		if err != nil {
			curState.Err = fmt.Errorf("failed to create alert: %w", err)
			return nil, curState.Err
		}
		a.ID = h
		a.State = notifier.StatePending
		a.ActiveAt = ts
		ar.alerts[h] = a
		ar.logDebugf(ts, a, "created in state PENDING")
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
				ar.logDebugf(ts, a, "PENDING => DELETED: is absent in current evaluation round")
				continue
			}
			// check if alert should keep StateFiring if rule has
			// `keep_firing_for` field
			if a.State == notifier.StateFiring {
				if ar.KeepFiringFor > 0 {
					if a.KeepFiringSince.IsZero() {
						a.KeepFiringSince = ts
					}
				}
				// alerts with ar.KeepFiringFor>0 may remain FIRING
				// even if their expression isn't true anymore
				if ts.Sub(a.KeepFiringSince) > ar.KeepFiringFor {
					a.State = notifier.StateInactive
					a.ResolvedAt = ts
					ar.logDebugf(ts, a, "FIRING => INACTIVE: is absent in current evaluation round")
					continue
				}
				ar.logDebugf(ts, a, "KEEP_FIRING: will keep firing for %fs since %v", ar.KeepFiringFor.Seconds(), a.KeepFiringSince)
			}
		}
		numActivePending++
		if a.State == notifier.StatePending && ts.Sub(a.ActiveAt) >= ar.For {
			a.State = notifier.StateFiring
			a.Start = ts
			alertsFired.Inc()
			ar.logDebugf(ts, a, "PENDING => FIRING: %s since becoming active at %v", ts.Sub(a.ActiveAt), a.ActiveAt)
		}
	}
	if limit > 0 && numActivePending > limit {
		ar.alerts = map[uint64]*notifier.Alert{}
		curState.Err = fmt.Errorf("exec exceeded limit of %d with %d alerts", limit, numActivePending)
		return nil, curState.Err
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
			return nil, fmt.Errorf("failed to expand labels: %w", err)
		}
	}
	a := &notifier.Alert{
		GroupID:  ar.GroupID,
		Name:     ar.Name,
		Labels:   ls.processed,
		Value:    m.Values[0],
		ActiveAt: start,
		Expr:     ar.Expr,
		For:      ar.For,
	}
	a.Annotations, err = a.ExecTemplate(qFn, ls.origin, ar.Annotations)
	return a, err
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

// restore restores the value of ActiveAt field for active alerts,
// based on previously written time series `alertForStateMetricName`.
// Only rules with For > 0 can be restored.
func (ar *AlertingRule) restore(ctx context.Context, q datasource.Querier, ts time.Time, lookback time.Duration) error {
	if ar.For < 1 {
		return nil
	}

	ar.alertsMu.Lock()
	defer ar.alertsMu.Unlock()

	if len(ar.alerts) < 1 {
		return nil
	}

	nameStr := fmt.Sprintf("%s=%q", alertNameLabel, ar.Name)
	if !*disableAlertGroupLabel {
		nameStr = fmt.Sprintf("%s=%q,%s=%q", alertGroupNameLabel, ar.GroupName, alertNameLabel, ar.Name)
	}
	var labelsFilter string
	for k, v := range ar.Labels {
		labelsFilter += fmt.Sprintf(",%s=%q", k, v)
	}
	expr := fmt.Sprintf("last_over_time(%s{%s%s}[%ds])",
		alertForStateMetricName, nameStr, labelsFilter, int(lookback.Seconds()))

	res, _, err := q.Query(ctx, expr, ts)
	if err != nil {
		return fmt.Errorf("failed to execute restore query %q: %w ", expr, err)
	}

	if len(res.Data) < 1 {
		ar.logDebugf(ts, nil, "no response was received from restore query")
		return nil
	}
	for _, series := range res.Data {
		series.DelLabel("__name__")
		labelSet := make(map[string]string, len(series.Labels))
		for _, v := range series.Labels {
			labelSet[v.Name] = v.Value
		}
		id := hash(labelSet)
		a, ok := ar.alerts[id]
		if !ok {
			continue
		}
		if a.Restored || a.State != notifier.StatePending {
			continue
		}
		a.ActiveAt = time.Unix(int64(series.Values[0]), 0)
		a.Restored = true
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
