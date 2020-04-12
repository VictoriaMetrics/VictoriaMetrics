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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/metricsql"
	"github.com/VictoriaMetrics/metrics"
)

// Group grouping array of alert
type Group struct {
	Name  string
	Rules []*Rule
}

// Rule is basic alert entity
type Rule struct {
	Name        string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         time.Duration     `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`

	group *Group

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
		if _, ok := r.alerts[h]; ok {
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

// Send sends the active alerts via given
// notifier.Notifier.
// See for reference https://prometheus.io/docs/alerting/clients/
// TODO: add tests for endAt value
func (r *Rule) Send(_ context.Context, ap notifier.Notifier) error {
	// copy alerts to new list to avoid locks
	var alertsCopy []notifier.Alert
	r.mu.Lock()
	for _, a := range r.alerts {
		if a.State == notifier.StatePending {
			continue
		}
		// it is safe to dereference instead of deep-copy
		// because only simple types may be changed during rule.Exec
		alertsCopy = append(alertsCopy, *a)
	}
	r.mu.Unlock()

	if len(alertsCopy) < 1 {
		return nil
	}
	alertsSent.Add(len(alertsCopy))
	return ap.Send(alertsCopy)
}

var (
	alertsFired = metrics.NewCounter(`vmalert_alerts_fired_total`)
	alertsSent  = metrics.NewCounter(`vmalert_alerts_sent_total`)
)

// TODO: consider hashing algorithm in VM
func hash(m datasource.Metric) uint64 {
	hash := fnv.New64a()
	labels := m.Labels
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

func (r *Rule) newAlert(m datasource.Metric) (*notifier.Alert, error) {
	a := &notifier.Alert{
		Group:  r.group.Name,
		Name:   r.Name,
		Labels: map[string]string{},
		Value:  m.Value,
		Start:  time.Now(),
		// TODO: support End time
	}
	for _, l := range m.Labels {
		a.Labels[l.Name] = l.Value
	}
	// metric labels may be overridden by
	// rule labels
	for k, v := range r.Labels {
		a.Labels[k] = v
	}
	var err error
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
		ID:          a.ID,
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
