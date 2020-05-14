package main

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metricsql"
)

// Rule is basic alert entity
type Rule struct {
	Name        string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         time.Duration     `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`

	group Group

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

func (r *Rule) id() string {
	return r.Name
}

// Validate validates rule
func (r *Rule) Validate() error {
	if r.Name == "" {
		return errors.New("rule name can not be empty")
	}
	if r.Expr == "" {
		return fmt.Errorf("expression for rule %q can't be empty", r.Name)
	}
	if _, err := metricsql.Parse(r.Expr); err != nil {
		return fmt.Errorf("invalid expression for rule %q: %w", r.Name, err)
	}
	return nil
}

// Exec executes Rule expression via the given Querier.
// Based on the Querier results Rule maintains notifier.Alerts
func (r *Rule) Exec(ctx context.Context, q datasource.Querier) error {
	qMetrics, err := q.Query(ctx, r.Expr)
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastExecError = err
	r.lastExecTime = time.Now()
	if err != nil {
		return fmt.Errorf("failed to execute query %q: %s", r.Expr, err)
	}

	for h, a := range r.alerts {
		// cleanup inactive alerts from previous Eval
		if a.State == notifier.StateInactive {
			delete(r.alerts, h)
		}
	}

	updated := make(map[uint64]struct{})
	// update list of active alerts
	for _, m := range qMetrics {
		h := hash(m)
		updated[h] = struct{}{}
		if a, ok := r.alerts[h]; ok {
			if a.Value != m.Value {
				// update Value field with latest value
				a.Value = m.Value
				// and re-exec template since Value can be used
				// in templates
				err = r.template(a)
				if err != nil {
					return err
				}
			}
			continue
		}
		a, err := r.newAlert(m)
		if err != nil {
			r.lastExecError = err
			return fmt.Errorf("failed to create alert: %s", err)
		}
		a.ID = h
		a.State = notifier.StatePending
		r.alerts[h] = a
	}

	for h, a := range r.alerts {
		// if alert wasn't updated in this iteration
		// means it is resolved already
		if _, ok := updated[h]; !ok {
			a.State = notifier.StateInactive
			// set endTime to last execution time
			// so it can be sent by notifier on next step
			a.End = r.lastExecTime
			continue
		}
		if a.State == notifier.StatePending && time.Since(a.Start) >= r.For {
			a.State = notifier.StateFiring
			alertsFired.Inc()
		}
		if a.State == notifier.StateFiring {
			a.End = r.lastExecTime.Add(3* r.group.Interval)
		}
	}
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

func (r *Rule) newAlert(m datasource.Metric) (*notifier.Alert, error) {
	a := &notifier.Alert{
		GroupID: r.group.ID(),
		Name:    r.Name,
		Labels:  map[string]string{},
		Value:   m.Value,
		Start:   time.Now(),
		// TODO: support End time
	}
	for _, l := range m.Labels {
		// drop __name__ to be consistent with Prometheus alerting
		if l.Name == "__name__" {
			continue
		}
		a.Labels[l.Name] = l.Value
	}
	return a, r.template(a)
}

func (r *Rule) template(a *notifier.Alert) error {
	// 1. template rule labels with data labels
	rLabels, err := a.ExecTemplate(r.Labels)
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

	a.Annotations, err = a.ExecTemplate(r.Annotations)
	return err
}

// AlertAPI generates APIAlert object from alert by its id(hash)
func (r *Rule) AlertAPI(id uint64) *APIAlert {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.alerts[id]
	if !ok {
		return nil
	}
	return r.newAlertAPI(*a)
}

// AlertsAPI generates list of APIAlert objects from existing alerts
func (r *Rule) AlertsAPI() []*APIAlert {
	var alerts []*APIAlert
	r.mu.RLock()
	for _, a := range r.alerts {
		alerts = append(alerts, r.newAlertAPI(*a))
	}
	r.mu.RUnlock()
	return alerts
}

func (r *Rule) newAlertAPI(a notifier.Alert) *APIAlert {
	return &APIAlert{
		// encode as strings to avoid rounding
		ID:      fmt.Sprintf("%d", a.ID),
		GroupID: fmt.Sprintf("%d", a.GroupID),

		Name:        a.Name,
		Expression:  r.Expr,
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

// AlertToTimeSeries converts the given alert with the given timestamp to timeseries
func (r *Rule) AlertToTimeSeries(a *notifier.Alert, timestamp time.Time) []prompbmarshal.TimeSeries {
	var tss []prompbmarshal.TimeSeries
	tss = append(tss, alertToTimeSeries(r.Name, a, timestamp))
	if r.For > 0 {
		tss = append(tss, alertForToTimeSeries(r.Name, a, timestamp))
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

func newTimeSeries(value float64, labels map[string]string, timestamp time.Time) prompbmarshal.TimeSeries {
	ts := prompbmarshal.TimeSeries{}
	ts.Samples = append(ts.Samples, prompbmarshal.Sample{
		Value:     value,
		Timestamp: timestamp.UnixNano() / 1e6,
	})
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ts.Labels = append(ts.Labels, prompbmarshal.Label{
			Name:  key,
			Value: labels[key],
		})
	}
	return ts
}

// Restore restores the state of active alerts basing on previously written timeseries.
// Restore restores only Start field. Field State will be always Pending and supposed
// to be updated on next Eval, as well as Value field.
func (r *Rule) Restore(ctx context.Context, q datasource.Querier, lookback time.Duration) error {
	if q == nil {
		return fmt.Errorf("querier is nil")
	}

	// Get the last datapoint in range via MetricsQL `last_over_time`.
	// We don't use plain PromQL since Prometheus doesn't support
	// remote write protocol which is used for state persistence in vmalert.
	expr := fmt.Sprintf("last_over_time(%s{alertname=%q}[%ds])",
		alertForStateMetricName, r.Name, int(lookback.Seconds()))
	qMetrics, err := q.Query(ctx, expr)
	if err != nil {
		return err
	}

	for _, m := range qMetrics {
		labels := m.Labels
		m.Labels = make([]datasource.Label, 0)
		// drop all extra labels, so hash key will
		// be identical to timeseries received in Eval
		for _, l := range labels {
			if l.Name == alertNameLabel {
				continue
			}
			// drop all overridden labels
			if _, ok := r.Labels[l.Name]; ok {
				continue
			}
			m.Labels = append(m.Labels, l)
		}

		a, err := r.newAlert(m)
		if err != nil {
			return fmt.Errorf("failed to create alert: %s", err)
		}
		a.ID = hash(m)
		a.State = notifier.StatePending
		a.Start = time.Unix(int64(m.Value), 0)
		r.alerts[a.ID] = a
		logger.Infof("alert %q(%d) restored to state at %v", a.Name, a.ID, a.Start)
	}
	return nil
}
