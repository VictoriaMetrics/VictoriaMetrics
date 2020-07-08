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
)

// AlertingRule is basic alert entity
type AlertingRule struct {
	RuleID      uint64
	Name        string
	Expr        string
	For         time.Duration
	Labels      map[string]string
	Annotations map[string]string
	GroupID     uint64

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
}

func newAlertingRule(gID uint64, cfg config.Rule) *AlertingRule {
	return &AlertingRule{
		RuleID:      cfg.ID,
		Name:        cfg.Alert,
		Expr:        cfg.Expr,
		For:         cfg.For,
		Labels:      cfg.Labels,
		Annotations: cfg.Annotations,
		GroupID:     gID,
		alerts:      make(map[uint64]*notifier.Alert),
	}
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

// Exec executes AlertingRule expression via the given Querier.
// Based on the Querier results AlertingRule maintains notifier.Alerts
func (ar *AlertingRule) Exec(ctx context.Context, q datasource.Querier, series bool) ([]prompbmarshal.TimeSeries, error) {
	qMetrics, err := q.Query(ctx, ar.Expr)
	ar.mu.Lock()
	defer ar.mu.Unlock()

	ar.lastExecError = err
	ar.lastExecTime = time.Now()
	if err != nil {
		return nil, fmt.Errorf("failed to execute query %q: %w", ar.Expr, err)
	}

	for h, a := range ar.alerts {
		// cleanup inactive alerts from previous Exec
		if a.State == notifier.StateInactive {
			delete(ar.alerts, h)
		}
	}

	updated := make(map[uint64]struct{})
	// update list of active alerts
	for _, m := range qMetrics {
		h := hash(m)
		updated[h] = struct{}{}
		if a, ok := ar.alerts[h]; ok {
			if a.Value != m.Value {
				// update Value field with latest value
				a.Value = m.Value
				// and re-exec template since Value can be used
				// in templates
				err = ar.template(a)
				if err != nil {
					return nil, err
				}
			}
			continue
		}
		a, err := ar.newAlert(m, ar.lastExecTime)
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
	if series {
		return ar.toTimeSeries(ar.lastExecTime), nil
	}
	return nil, nil
}

func (ar *AlertingRule) toTimeSeries(timestamp time.Time) []prompbmarshal.TimeSeries {
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

func (ar *AlertingRule) newAlert(m datasource.Metric, start time.Time) (*notifier.Alert, error) {
	a := &notifier.Alert{
		GroupID: ar.GroupID,
		Name:    ar.Name,
		Labels:  map[string]string{},
		Value:   m.Value,
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
	return a, ar.template(a)
}

func (ar *AlertingRule) template(a *notifier.Alert) error {
	// 1. template rule labels with data labels
	rLabels, err := a.ExecTemplate(ar.Labels)
	if err != nil {
		return err
	}

	// 2. merge data labels and rule labels
	// metric labels may be overridden by
	// rule labels
	for k, v := range rLabels {
		a.Labels[k] = v
	}

	// 3. template merged labels
	a.Labels, err = a.ExecTemplate(a.Labels)
	if err != nil {
		return err
	}

	a.Annotations, err = a.ExecTemplate(ar.Annotations)
	return err
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
		Name:        ar.Name,
		Expression:  ar.Expr,
		For:         ar.For.String(),
		LastError:   lastErr,
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
	return &APIAlert{
		// encode as strings to avoid rounding
		ID:      fmt.Sprintf("%d", a.ID),
		GroupID: fmt.Sprintf("%d", a.GroupID),

		Name:        a.Name,
		Expression:  ar.Expr,
		Labels:      a.Labels,
		Annotations: a.Annotations,
		State:       a.State.String(),
		ActiveAt:    a.Start,
		Value:       strconv.FormatFloat(a.Value, 'e', -1, 64),
	}
}

const (
	// AlertMetricName is the metric name for synthetic alert timeseries.
	alertMetricName = "ALERTS"
	// AlertForStateMetricName is the metric name for 'for' state of alert.
	alertForStateMetricName = "ALERTS_FOR_STATE"

	// AlertNameLabel is the label name indicating the name of an alert.
	alertNameLabel = "alertname"
	// AlertStateLabel is the label name indicating the state of an alert.
	alertStateLabel = "alertstate"
)

// alertToTimeSeries converts the given alert with the given timestamp to timeseries
func (ar *AlertingRule) alertToTimeSeries(a *notifier.Alert, timestamp time.Time) []prompbmarshal.TimeSeries {
	var tss []prompbmarshal.TimeSeries
	tss = append(tss, alertToTimeSeries(ar.Name, a, timestamp))
	if ar.For > 0 {
		tss = append(tss, alertForToTimeSeries(ar.Name, a, timestamp))
	}
	return tss
}

func alertToTimeSeries(name string, a *notifier.Alert, timestamp time.Time) prompbmarshal.TimeSeries {
	labels := make(map[string]string)
	for k, v := range a.Labels {
		labels[k] = v
	}
	labels["__name__"] = alertMetricName
	labels[alertNameLabel] = name
	labels[alertStateLabel] = a.State.String()
	return newTimeSeries(1, labels, timestamp)
}

// alertForToTimeSeries returns a timeseries that represents
// state of active alerts, where value is time when alert become active
func alertForToTimeSeries(name string, a *notifier.Alert, timestamp time.Time) prompbmarshal.TimeSeries {
	labels := make(map[string]string)
	for k, v := range a.Labels {
		labels[k] = v
	}
	labels["__name__"] = alertForStateMetricName
	labels[alertNameLabel] = name
	return newTimeSeries(float64(a.Start.Unix()), labels, timestamp)
}

// Restore restores the state of active alerts basing on previously written timeseries.
// Restore restores only Start field. Field State will be always Pending and supposed
// to be updated on next Exec, as well as Value field.
// Only rules with For > 0 will be restored.
func (ar *AlertingRule) Restore(ctx context.Context, q datasource.Querier, lookback time.Duration) error {
	if q == nil {
		return fmt.Errorf("querier is nil")
	}
	// Get the last datapoint in range via MetricsQL `last_over_time`.
	// We don't use plain PromQL since Prometheus doesn't support
	// remote write protocol which is used for state persistence in vmalert.
	expr := fmt.Sprintf("last_over_time(%s{alertname=%q}[%ds])",
		alertForStateMetricName, ar.Name, int(lookback.Seconds()))
	qMetrics, err := q.Query(ctx, expr)
	if err != nil {
		return err
	}

	for _, m := range qMetrics {
		labels := m.Labels
		m.Labels = make([]datasource.Label, 0)
		// drop all extra labels, so hash key will
		// be identical to timeseries received in Exec
		for _, l := range labels {
			if l.Name == alertNameLabel {
				continue
			}
			// drop all overridden labels
			if _, ok := ar.Labels[l.Name]; ok {
				continue
			}
			m.Labels = append(m.Labels, l)
		}

		a, err := ar.newAlert(m, time.Unix(int64(m.Value), 0))
		if err != nil {
			return fmt.Errorf("failed to create alert: %w", err)
		}
		a.ID = hash(m)
		a.State = notifier.StatePending
		ar.alerts[a.ID] = a
		logger.Infof("alert %q(%d) restored to state at %v", a.Name, a.ID, a.Start)
	}
	return nil
}
