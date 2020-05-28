package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// Group is an entity for grouping rules
type Group struct {
	Name     string
	Interval time.Duration `yaml:"interval"`
	File     string
	Rules    []*Rule
	doneCh     chan struct{}
	finishedCh chan struct{}
	// channel accepts new Group obj
	// which supposed to update current group
	updateCh chan Group
}

// ID return unique group ID that consists of
// rules file and group name
func (g *Group) ID() uint64 {
	hash := fnv.New64a()
	hash.Write([]byte(g.File))
	hash.Write([]byte("\xff"))
	hash.Write([]byte(g.Name))
	return hash.Sum64()
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

// updateWith updates existing group with
// passed group object.
// Not thread-safe.
func (g *Group) updateWith(newGroup Group) {
	g.rulesMu.Lock()
	g.Interval = newGroup.Interval

	rulesRegistry := make(map[string]*Rule)
	for _, nr := range newGroup.Rules {
		rulesRegistry[nr.id()] = nr
	}

	for i, or := range g.Rules {
		nr, ok := rulesRegistry[or.id()]
		if !ok {
			// old rule is not present in the new list
			// so we mark it for removing
			g.Rules[i] = nil
			continue
		}

		// copy all significant fields.
		// alerts state isn't copied since
		// it should be updated in next 2 Execs
		or.For = nr.For
		or.Expr = nr.Expr
		or.Labels = nr.Labels
		or.Annotations = nr.Annotations
		delete(rulesRegistry, nr.id())
	}

	var newRules []*Rule
	for _, r := range g.Rules {
		if r == nil {
			// skip nil rules
			continue
		}
		newRules = append(newRules, r)
	}
	// add the rest of rules from registry
	for _, nr := range rulesRegistry {
		newRules = append(newRules, nr)
	}
	g.Rules = newRules
}

var (
	iterationTotal    = metrics.NewCounter(`vmalert_iteration_total`)
	iterationDuration = metrics.NewSummary(`vmalert_iteration_duration_seconds`)

	execTotal    = metrics.NewCounter(`vmalert_execution_total`)
	execErrors   = metrics.NewCounter(`vmalert_execution_errors_total`)
	execDuration = metrics.NewSummary(`vmalert_execution_duration_seconds`)

	alertsFired      = metrics.NewCounter(`vmalert_alerts_fired_total`)
	alertsSent       = metrics.NewCounter(`vmalert_alerts_sent_total`)
	alertsSendErrors = metrics.NewCounter(`vmalert_alerts_send_errors_total`)

	remoteWriteSent   = metrics.NewCounter(`vmalert_remotewrite_sent_total`)
	remoteWriteErrors = metrics.NewCounter(`vmalert_remotewrite_errors_total`)
)

func (g *Group) close() {
	if g.doneCh == nil {
		return
	}
	g.doneCh <- struct{}{}
	<-g.finishedCh
}

func (g *Group) start(ctx context.Context,
	querier datasource.Querier, nr notifier.Notifier, rw *remotewrite.Client) {
	logger.Infof("group %q started", g.Name)
	t := time.NewTicker(g.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Infof("group %q: context cancelled", g.Name)
			close(g.finishedCh)
			return
		case <-g.doneCh:
			logger.Infof("group %q: received stop signal", g.Name)
			close(g.finishedCh)
			return
		case ng := <-g.updateCh:
			g.updateWith(ng)
		case <-t.C:
			iterationTotal.Inc()
			iterationStart := time.Now()
			for _, rule := range g.Rules {
				execTotal.Inc()

				execStart := time.Now()
				err := rule.Exec(ctx, querier)
				execDuration.UpdateDuration(execStart)

				if err != nil {
					execErrors.Inc()
					logger.Errorf("failed to execute rule %q.%q: %s", g.Name, rule.Name, err)
					continue
				}

				var alertsToSend []notifier.Alert
				for _, a := range rule.alerts {
					switch a.State {
					case notifier.StateFiring:
						// set End to execStart + 3 intervals
						// so notifier can resolve it automatically if `vmalert`
						// won't be able to send resolve for some reason
						a.End = execStart.Add(3 * g.Interval)
						alertsToSend = append(alertsToSend, *a)
						pushToRW(rw, rule, a, execStart)
					case notifier.StatePending:
						pushToRW(rw, rule, a, execStart)
					case notifier.StateInactive:
						// set End to execStart to notify
						// that it was just resolved
						a.End = execStart
						alertsToSend = append(alertsToSend, *a)
					}
				}
				if len(alertsToSend) == 0 {
					continue
				}
				alertsSent.Add(len(alertsToSend))
				if err := nr.Send(ctx, alertsToSend); err != nil {
					alertsSendErrors.Inc()
					logger.Errorf("failed to send alert for rule %q.%q: %s", g.Name, rule.Name, err)
				}
			}
			iterationDuration.UpdateDuration(iterationStart)
		}
	}
}

func pushToRW(rw *remotewrite.Client, rule *Rule, a *notifier.Alert, timestamp time.Time) {
	if rw == nil {
		return
	}
	tss := rule.AlertToTimeSeries(a, timestamp)
	remoteWriteSent.Add(len(tss))
	for _, ts := range tss {
		if err := rw.Push(ts); err != nil {
			remoteWriteErrors.Inc()
			logger.Errorf("failed to push timeseries to remotewrite: %s", err)
		}
	}
}
