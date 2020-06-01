package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// Group is an entity for grouping rules
type Group struct {
	Name     string
	File     string
	Rules    []Rule
	Interval time.Duration

	doneCh     chan struct{}
	finishedCh chan struct{}
	// channel accepts new Group obj
	// which supposed to update current group
	updateCh chan *Group
	mu       sync.RWMutex
}

func newGroup(cfg config.Group, defaultInterval time.Duration) *Group {
	g := &Group{
		Name:       cfg.Name,
		File:       cfg.File,
		Interval:   cfg.Interval,
		doneCh:     make(chan struct{}),
		finishedCh: make(chan struct{}),
		updateCh:   make(chan *Group),
	}
	if g.Interval == 0 {
		g.Interval = defaultInterval
	}
	rules := make([]Rule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		rules[i] = g.newRule(r)
	}
	g.Rules = rules
	return g
}

func (g *Group) newRule(rule config.Rule) Rule {
	if rule.Alert != "" {
		return newAlertingRule(g.ID(), rule)
	}
	return newRecordingRule(g.ID(), rule)
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

// Restore restores alerts state for group rules
func (g *Group) Restore(ctx context.Context, q datasource.Querier, lookback time.Duration) error {
	for _, rule := range g.Rules {
		rr, ok := rule.(*AlertingRule)
		if !ok {
			continue
		}
		if rr.For < 1 {
			continue
		}
		if err := rr.Restore(ctx, q, lookback); err != nil {
			return fmt.Errorf("error while restoring rule %q: %s", rule, err)
		}
	}
	return nil
}

// updateWith updates existing group with
// passed group object. This function ignores group
// evaluation interval change. It supposed to be updated
// in group.start function.
// Not thread-safe.
func (g *Group) updateWith(newGroup *Group) error {
	rulesRegistry := make(map[uint64]Rule)
	for _, nr := range newGroup.Rules {
		rulesRegistry[nr.ID()] = nr
	}

	for i, or := range g.Rules {
		nr, ok := rulesRegistry[or.ID()]
		if !ok {
			// old rule is not present in the new list
			// so we mark it for removing
			g.Rules[i] = nil
			continue
		}
		if err := or.UpdateWith(nr); err != nil {
			return err
		}
		delete(rulesRegistry, nr.ID())
	}

	var newRules []Rule
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
	return nil
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
	close(g.doneCh)
	<-g.finishedCh
}

func (g *Group) start(ctx context.Context, querier datasource.Querier, nr notifier.Notifier, rw *remotewrite.Client) {
	logger.Infof("group %q started with interval %v", g.Name, g.Interval)

	var returnSeries bool
	if rw != nil {
		returnSeries = true
	}

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
			g.mu.Lock()
			err := g.updateWith(ng)
			if err != nil {
				logger.Errorf("group %q: failed to update: %s", g.Name, err)
				g.mu.Unlock()
				continue
			}
			if g.Interval != ng.Interval {
				g.Interval = ng.Interval
				t.Stop()
				t = time.NewTicker(g.Interval)
				logger.Infof("group %q: changed evaluation interval to %v", g.Name, g.Interval)
			}
			g.mu.Unlock()
		case <-t.C:
			iterationTotal.Inc()
			iterationStart := time.Now()
			for _, rule := range g.Rules {
				execTotal.Inc()

				execStart := time.Now()
				tss, err := rule.Exec(ctx, querier, returnSeries)
				execDuration.UpdateDuration(execStart)

				if err != nil {
					execErrors.Inc()
					logger.Errorf("failed to execute rule %q.%q: %s", g.Name, rule, err)
					continue
				}

				if len(tss) > 0 {
					remoteWriteSent.Add(len(tss))
					for _, ts := range tss {
						if err := rw.Push(ts); err != nil {
							remoteWriteErrors.Inc()
							logger.Errorf("failed to remote write for rule %q.%q: %s", g.Name, rule, err)
						}
					}
				}

				ar, ok := rule.(*AlertingRule)
				if !ok {
					continue
				}
				var alerts []notifier.Alert
				for _, a := range ar.alerts {
					switch a.State {
					case notifier.StateFiring:
						// set End to execStart + 3 intervals
						// so notifier can resolve it automatically if `vmalert`
						// won't be able to send resolve for some reason
						a.End = execStart.Add(3 * g.Interval)
						alerts = append(alerts, *a)
					case notifier.StateInactive:
						// set End to execStart to notify
						// that it was just resolved
						a.End = execStart
						alerts = append(alerts, *a)
					}
				}
				if len(alerts) < 1 {
					continue
				}
				alertsSent.Add(len(alerts))
				if err := nr.Send(ctx, alerts); err != nil {
					alertsSendErrors.Inc()
					logger.Errorf("failed to send alert for rule %q.%q: %s", g.Name, rule, err)
				}
			}
			iterationDuration.UpdateDuration(iterationStart)
		}
	}
}
