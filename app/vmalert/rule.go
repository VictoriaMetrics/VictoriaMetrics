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

// Group grouping array of alert
type Group struct {
	Name  string
	Rules []*Rule
}

// Restore restores alerts state for all group rules with For > 0
func (g *Group) Restore(ctx context.Context, q datasource.Querier, lookback time.Duration) error {
	for _, rule := range g.Rules {
		if rule.For == 0 {
			return nil
		}
		if err := rule.Restore(ctx, q, lookback); err != nil {
			return fmt.Errorf("error while restoring rule %q: %s", rule.Name, err)
		}
	}
	return nil
}


// Update group
func (g *Group)Update(newGroup Group)*Group{
	//check if old rule exists at new rules
	for _, newRule := range newGroup.Rules{
		for _, oldRule := range g.Rules{
			if newRule.Name == oldRule.Name{
	            //is lock nessesary?
				oldRule.mu.Lock()
				//we copy only rules related values
				//it`s safe to add additional fields to rule
				//struct
				oldRule.Annotations = newRule.Annotations
				oldRule.Labels = newRule.Labels
				oldRule.For = newRule.For
				newRule = oldRule
				oldRule.mu.Unlock()
			}
		}
	}
	//swap rules
	g.Rules = newGroup.Rules
	return g
}

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
			// update Value field with latest value
			a.Value = m.Value
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
			a.End = r.lastExecTime.Add(3 * *evaluationInterval)
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
		Group:  r.group.Name,
		Name:   r.Name,
		Labels: map[string]string{},
		Value:  m.Value,
		Start:  time.Now(),
		// TODO: support End time
	}

	// 1. use data labels
	for _, l := range m.Labels {
		// drop __name__ to be consistent with Prometheus alerting
		if l.Name == "__name__" {
			continue
		}
		a.Labels[l.Name] = l.Value
	}

	// 2. template rule labels with data labels
	rLabels, err := a.ExecTemplate(r.Labels)
	if err != nil {
		return a, err
	}

	// 3. merge data labels and rule labels
	// metric labels may be overridden by
	// rule labels
	for k, v := range rLabels {
		a.Labels[k] = v
	}

	// 4. template merged labels
	a.Labels, err = a.ExecTemplate(a.Labels)
	if err != nil {
		return a, err
	}

	a.Annotations, err = a.ExecTemplate(r.Annotations)
	return a, err
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
		// encode as string to avoid rounding
		ID:          fmt.Sprintf("%d", a.ID),
		Name:        a.Name,
		Group:       a.Group,
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
		logger.Infof("alert %q.%q restored to state at %v", a.Group, a.Name, a.Start)
	}
	return nil
}
