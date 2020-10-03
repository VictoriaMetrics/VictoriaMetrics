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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
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
	AuthToken   *auth.Token
	Checksum    string

	doneCh     chan struct{}
	finishedCh chan struct{}
	// channel accepts new Group obj
	// which supposed to update current group
	updateCh chan *Group

	metrics *groupMetrics
}

type groupMetrics struct {
	iterationTotal    *counter
	iterationDuration *summary
}

func newGroupMetrics(name, file string) *groupMetrics {
	m := &groupMetrics{}
	labels := fmt.Sprintf(`group=%q, file=%q`, name, file)
	m.iterationTotal = getOrCreateCounter(fmt.Sprintf(`vmalert_iteration_total{%s}`, labels))
	m.iterationDuration = getOrCreateSummary(fmt.Sprintf(`vmalert_iteration_duration_seconds{%s}`, labels))
	return m
}

func newGroup(cfg config.Group, defaultInterval time.Duration, labels map[string]string) *Group {
	g := &Group{
		Name:        cfg.Name,
		File:        cfg.File,
		Interval:    cfg.Interval,
		Concurrency: cfg.Concurrency,
		Checksum:    cfg.Checksum,
		doneCh:      make(chan struct{}),
		finishedCh:  make(chan struct{}),
		updateCh:    make(chan *Group),
	}
	// error format of auth token is filter at rule file Parse
	token, _ := auth.NewToken(cfg.Tenant)
	g.AuthToken = token
	g.metrics = newGroupMetrics(g.Name, g.File)
	if g.Interval == 0 {
		g.Interval = defaultInterval
	}
	if g.Concurrency < 1 {
		g.Concurrency = 1
	}
	rules := make([]Rule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		// override rule labels with external labels
		for k, v := range labels {
			if prevV, ok := r.Labels[k]; ok {
				logger.Infof("label %q=%q for rule %q.%q overwritten with external label %q=%q",
					k, prevV, g.Name, r.Name(), k, v)
			}
			if r.Labels == nil {
				r.Labels = map[string]string{}
			}
			r.Labels[k] = v
		}
		rules[i] = g.newRule(r)
	}
	g.Rules = rules
	return g
}

func (g *Group) newRule(rule config.Rule) Rule {
	if rule.Alert != "" {
		return newAlertingRule(g, rule)
	}
	return newRecordingRule(g, rule)
}

// ID return unique group ID that consists of
// rules file and group name
func (g *Group) ID() uint64 {
	hash := fnv.New64a()
	hash.Write([]byte(g.File))
	hash.Write([]byte("\xff"))
	hash.Write([]byte(g.Name))
	hash.Write([]byte("\xff"))
	hash.Write([]byte(g.AuthToken.String()))
	return hash.Sum64()
}

// Restore restores alerts state for group rules
func (g *Group) Restore(ctx context.Context, q datasource.Querier, lookback time.Duration, labels map[string]string) error {
	for _, rule := range g.Rules {
		rr, ok := rule.(*AlertingRule)
		if !ok {
			continue
		}
		if rr.For < 1 {
			continue
		}
		if err := rr.Restore(ctx, q, lookback, labels); err != nil {
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
			g.Rules[i].Close()
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
	g.Checksum = newGroup.Checksum
	g.AuthToken = newGroup.AuthToken
	g.Rules = newRules
	return nil
}

var (
	alertsFired      = metrics.NewCounter(`vmalert_alerts_fired_total`)
	alertsSent       = metrics.NewCounter(`vmalert_alerts_sent_total`)
	alertsSendErrors = metrics.NewCounter(`vmalert_alerts_send_errors_total`)
)

func (g *Group) close() {
	if g.doneCh == nil {
		return
	}
	close(g.doneCh)
	<-g.finishedCh

	metrics.UnregisterMetric(g.metrics.iterationDuration.name)
	metrics.UnregisterMetric(g.metrics.iterationTotal.name)
	for _, rule := range g.Rules {
		rule.Close()
	}
}

var skipRandSleepOnGroupStart bool

func (g *Group) start(ctx context.Context, querier datasource.Querier, nts []notifier.Notifier, rw *remotewrite.Client) {
	defer func() { close(g.finishedCh) }()

	// Spread group rules evaluation over time in order to reduce load on VictoriaMetrics.
	if !skipRandSleepOnGroupStart {
		randSleep := uint64(float64(g.Interval) * (float64(uint32(g.ID())) / (1 << 32)))
		sleepOffset := uint64(time.Now().UnixNano()) % uint64(g.Interval)
		if randSleep < sleepOffset {
			randSleep += uint64(g.Interval)
		}
		randSleep -= sleepOffset
		sleepTimer := time.NewTimer(time.Duration(randSleep))
		select {
		case <-ctx.Done():
			sleepTimer.Stop()
			return
		case <-g.doneCh:
			sleepTimer.Stop()
			return
		case <-sleepTimer.C:
		}
	}

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
			g.metrics.iterationTotal.Inc()
			iterationStart := time.Now()

			errs := e.execConcurrently(ctx, g.Rules, g.Concurrency, g.Interval)
			for err := range errs {
				if err != nil {
					logger.Errorf("group %q: %s", g.Name, err)
				}
			}

			g.metrics.iterationDuration.UpdateDuration(iterationStart)
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

var (
	execTotal    = metrics.NewCounter(`vmalert_execution_total`)
	execErrors   = metrics.NewCounter(`vmalert_execution_errors_total`)
	execDuration = metrics.NewSummary(`vmalert_execution_duration_seconds`)

	remoteWriteErrors = metrics.NewCounter(`vmalert_remotewrite_errors_total`)
)

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
