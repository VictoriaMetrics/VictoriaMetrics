package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/url"
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
	Type        datasource.Type
	Interval    time.Duration
	Concurrency int
	Checksum    string

	Labels map[string]string
	Params url.Values

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

// merges group rule labels into result map
// set2 has priority over set1.
func mergeLabels(groupName, ruleName string, set1, set2 map[string]string) map[string]string {
	r := map[string]string{}
	for k, v := range set1 {
		r[k] = v
	}
	for k, v := range set2 {
		if prevV, ok := r[k]; ok {
			logger.Infof("label %q=%q for rule %q.%q overwritten with external label %q=%q",
				k, prevV, groupName, ruleName, k, v)
		}
		r[k] = v
	}
	return r
}

func newGroup(cfg config.Group, qb datasource.QuerierBuilder, defaultInterval time.Duration, labels map[string]string) *Group {
	g := &Group{
		Type:        cfg.Type,
		Name:        cfg.Name,
		File:        cfg.File,
		Interval:    cfg.Interval.Duration(),
		Concurrency: cfg.Concurrency,
		Checksum:    cfg.Checksum,
		Params:      cfg.Params,
		Labels:      cfg.Labels,

		doneCh:     make(chan struct{}),
		finishedCh: make(chan struct{}),
		updateCh:   make(chan *Group),
	}
	g.metrics = newGroupMetrics(g.Name, g.File)
	if g.Interval == 0 {
		g.Interval = defaultInterval
	}
	if g.Concurrency < 1 {
		g.Concurrency = 1
	}
	rules := make([]Rule, len(cfg.Rules))
	for i, r := range cfg.Rules {
		var extraLabels map[string]string
		// apply external labels
		if len(labels) > 0 {
			extraLabels = labels
		}
		// apply group labels, it has priority on external labels
		if len(cfg.Labels) > 0 {
			extraLabels = mergeLabels(g.Name, r.Name(), extraLabels, g.Labels)
		}
		// apply rules labels, it has priority on other labels
		if len(extraLabels) > 0 {
			r.Labels = mergeLabels(g.Name, r.Name(), extraLabels, r.Labels)
		}

		rules[i] = g.newRule(qb, r)
	}
	g.Rules = rules
	return g
}

func (g *Group) newRule(qb datasource.QuerierBuilder, rule config.Rule) Rule {
	if rule.Alert != "" {
		return newAlertingRule(qb, g, rule)
	}
	return newRecordingRule(qb, g, rule)
}

// ID return unique group ID that consists of
// rules file and group name
func (g *Group) ID() uint64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	hash := fnv.New64a()
	hash.Write([]byte(g.File))
	hash.Write([]byte("\xff"))
	hash.Write([]byte(g.Name))
	hash.Write([]byte(g.Type.Get()))
	return hash.Sum64()
}

// Restore restores alerts state for group rules
func (g *Group) Restore(ctx context.Context, qb datasource.QuerierBuilder, lookback time.Duration, labels map[string]string) error {
	labels = mergeLabels(g.Name, "", labels, g.Labels)
	for _, rule := range g.Rules {
		rr, ok := rule.(*AlertingRule)
		if !ok {
			continue
		}
		if rr.For < 1 {
			continue
		}
		// ignore g.ExtraFilterLabels on purpose, so it
		// won't affect the restore procedure.
		q := qb.BuildWithParams(datasource.QuerierParams{})
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
	// note that g.Interval is not updated here
	// so the value can be compared later in
	// group.Start function
	g.Type = newGroup.Type
	g.Concurrency = newGroup.Concurrency
	g.Params = newGroup.Params
	g.Labels = newGroup.Labels
	g.Checksum = newGroup.Checksum
	g.Rules = newRules
	return nil
}

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

func (g *Group) start(ctx context.Context, nts []notifier.Notifier, rw *remotewrite.Client) {
	defer func() { close(g.finishedCh) }()

	// Spread group rules evaluation over time in order to reduce load on VictoriaMetrics.
	if !skipRandSleepOnGroupStart {
		randSleep := uint64(float64(g.Interval) * (float64(g.ID()) / (1 << 64)))
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
	e := &executor{rw: rw}
	for _, nt := range nts {
		ent := eNotifier{
			Notifier:         nt,
			alertsSent:       getOrCreateCounter(fmt.Sprintf("vmalert_alerts_sent_total{addr=%q}", nt.Addr())),
			alertsSendErrors: getOrCreateCounter(fmt.Sprintf("vmalert_alerts_send_errors_total{addr=%q}", nt.Addr())),
		}
		e.notifiers = append(e.notifiers, ent)
	}

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
			if len(g.Rules) > 0 {
				resolveDuration := getResolveDuration(g.Interval)
				errs := e.execConcurrently(ctx, g.Rules, g.Concurrency, resolveDuration)
				for err := range errs {
					if err != nil {
						logger.Errorf("group %q: %s", g.Name, err)
					}
				}
			}
			g.metrics.iterationDuration.UpdateDuration(iterationStart)
		}
	}
}

// resolveDuration for alerts is equal to 3 interval evaluations
// so in case if vmalert stops sending updates for some reason,
// notifier could automatically resolve the alert.
func getResolveDuration(groupInterval time.Duration) time.Duration {
	resolveInterval := groupInterval * 3
	if *maxResolveDuration > 0 && (resolveInterval > *maxResolveDuration) {
		return *maxResolveDuration
	}
	return resolveInterval
}

type executor struct {
	notifiers []eNotifier
	rw        *remotewrite.Client
}

type eNotifier struct {
	notifier.Notifier
	alertsSent       *counter
	alertsSendErrors *counter
}

func (e *executor) execConcurrently(ctx context.Context, rules []Rule, concurrency int, resolveDuration time.Duration) chan error {
	res := make(chan error, len(rules))
	if concurrency == 1 {
		// fast path
		for _, rule := range rules {
			res <- e.exec(ctx, rule, resolveDuration)
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
				res <- e.exec(ctx, r, resolveDuration)
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
	alertsFired = metrics.NewCounter(`vmalert_alerts_fired_total`)

	execTotal  = metrics.NewCounter(`vmalert_execution_total`)
	execErrors = metrics.NewCounter(`vmalert_execution_errors_total`)

	remoteWriteErrors = metrics.NewCounter(`vmalert_remotewrite_errors_total`)
	remoteWriteTotal  = metrics.NewCounter(`vmalert_remotewrite_total`)
)

func (e *executor) exec(ctx context.Context, rule Rule, resolveDuration time.Duration) error {
	execTotal.Inc()

	tss, err := rule.Exec(ctx)
	if err != nil {
		execErrors.Inc()
		return fmt.Errorf("rule %q: failed to execute: %w", rule, err)
	}

	if len(tss) > 0 && e.rw != nil {
		for _, ts := range tss {
			remoteWriteTotal.Inc()
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
			a.End = time.Now().Add(resolveDuration)
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

	errGr := new(utils.ErrGroup)
	for _, nt := range e.notifiers {
		nt.alertsSent.Add(len(alerts))
		if err := nt.Send(ctx, alerts); err != nil {
			nt.alertsSendErrors.Inc()
			errGr.Add(fmt.Errorf("rule %q: failed to send alerts: %w", rule, err))
		}
	}
	return errGr.Err()
}
