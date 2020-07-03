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
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// Group is an entity for grouping rules
type Group struct {
	mu          sync.RWMutex
	Name        string
	File        string
	Rules       []Rule
	Interval    time.Duration
	Concurrency int

	doneCh     chan struct{}
	finishedCh chan struct{}
	// channel accepts new Group obj
	// which supposed to update current group
	updateCh chan *Group
}

func newGroup(cfg config.Group, defaultInterval time.Duration) *Group {
	g := &Group{
		Name:        cfg.Name,
		File:        cfg.File,
		Interval:    cfg.Interval,
		Concurrency: cfg.Concurrency,
		doneCh:      make(chan struct{}),
		finishedCh:  make(chan struct{}),
		updateCh:    make(chan *Group),
	}
	if g.Interval == 0 {
		g.Interval = defaultInterval
	}
	if g.Concurrency < 1 {
		g.Concurrency = 1
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
			return fmt.Errorf("error while restoring rule %q: %w", rule, err)
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
	g.Concurrency = newGroup.Concurrency
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

func (g *Group) start(ctx context.Context, querier datasource.Querier, nts []notifier.Notifier, rw *remotewrite.Client) {
	defer func() { close(g.finishedCh) }()
	logger.Infof("group %q started; interval=%v; concurrency=%d", g.Name, g.Interval, g.Concurrency)
	e := &executor{querier, nts, rw}
	t := time.NewTicker(g.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Infof("group %q: context cancelled", g.Name)
			return
		case <-g.doneCh:
			logger.Infof("group %q: received stop signal", g.Name)
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
			}
			g.mu.Unlock()
			logger.Infof("group %q re-started; interval=%v; concurrency=%d", g.Name, g.Interval, g.Concurrency)
		case <-t.C:
			iterationTotal.Inc()
			iterationStart := time.Now()

			errs := e.execConcurrently(ctx, g.Rules, g.Concurrency, g.Interval)
			for err := range errs {
				if err != nil {
					logger.Errorf("group %q: %s", g.Name, err)
				}
			}

			iterationDuration.UpdateDuration(iterationStart)
		}
	}
}

type executor struct {
	querier   datasource.Querier
	notifiers []notifier.Notifier
	rw        *remotewrite.Client
}

func (e *executor) execConcurrently(ctx context.Context, rules []Rule, concurrency int, interval time.Duration) chan error {
	res := make(chan error, len(rules))
	var returnSeries bool
	if e.rw != nil {
		returnSeries = true
	}

	if concurrency == 1 {
		// fast path
		for _, rule := range rules {
			res <- e.exec(ctx, rule, returnSeries, interval)
		}
		close(res)
		return res
	}

	sem := make(chan struct{}, concurrency)
	go func() {
		wg := sync.WaitGroup{}
		for _, rule := range rules {
			sem <- struct{}{}
			wg.Add(1)
			go func(r Rule) {
				res <- e.exec(ctx, r, returnSeries, interval)
				<-sem
				wg.Done()
			}(rule)
		}
		wg.Wait()
		close(res)
	}()
	return res
}

func (e *executor) exec(ctx context.Context, rule Rule, returnSeries bool, interval time.Duration) error {
	execTotal.Inc()
	execStart := time.Now()
	defer func() {
		execDuration.UpdateDuration(execStart)
	}()

	tss, err := rule.Exec(ctx, e.querier, returnSeries)
	if err != nil {
		execErrors.Inc()
		return fmt.Errorf("rule %q: failed to execute: %w", rule, err)
	}

	if len(tss) > 0 && e.rw != nil {
		remoteWriteSent.Add(len(tss))
		for _, ts := range tss {
			if err := e.rw.Push(ts); err != nil {
				remoteWriteErrors.Inc()
				return fmt.Errorf("rule %q: remote write failure: %w", rule, err)
			}
		}
	}

	ar, ok := rule.(*AlertingRule)
	if !ok {
		return nil
	}
	var alerts []notifier.Alert
	for _, a := range ar.alerts {
		switch a.State {
		case notifier.StateFiring:
			// set End to execStart + 3 intervals
			// so notifier can resolve it automatically if `vmalert`
			// won't be able to send resolve for some reason
			a.End = time.Now().Add(3 * interval)
			alerts = append(alerts, *a)
		case notifier.StateInactive:
			// set End to execStart to notify
			// that it was just resolved
			a.End = time.Now()
			alerts = append(alerts, *a)
		}
	}
	if len(alerts) < 1 {
		return nil
	}

	alertsSent.Add(len(alerts))
	errGr := new(utils.ErrGroup)
	for _, nt := range e.notifiers {
		if err := nt.Send(ctx, alerts); err != nil {
			alertsSendErrors.Inc()
			errGr.Add(fmt.Errorf("rule %q: failed to send alerts: %w", rule, err))
		}
	}
	return errGr.Err()
}
