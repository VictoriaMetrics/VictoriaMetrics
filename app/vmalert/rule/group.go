package rule

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"net/url"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/vmalertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

var (
	ruleUpdateEntriesLimit = flag.Int("rule.updateEntriesLimit", 20, "Defines the max number of rule's state updates stored in-memory. "+
		"Rule's updates are available on rule's Details page and are used for debugging purposes. The number of stored updates can be overridden per rule via update_entries_limit param.")
	resendDelay        = flag.Duration("rule.resendDelay", 0, "MiniMum amount of time to wait before resending an alert to notifier.")
	maxResolveDuration = flag.Duration("rule.maxResolveDuration", 0, "Limits the maxiMum duration for automatic alert expiration, "+
		"which by default is 4 times evaluationInterval of the parent group")
	evalDelay = flag.Duration("rule.evalDelay", 30*time.Second, "Adjustment of the `time` parameter for rule evaluation requests to compensate intentional data delay from the datasource. "+
		"Normally, should be equal to `-search.latencyOffset` (cmd-line flag configured for VictoriaMetrics single-node or vmselect). "+
		"This doesn't apply to groups with eval_offset specified.")
	disableAlertGroupLabel = flag.Bool("disableAlertgroupLabel", false, "Whether to disable adding group's Name as label to generated alerts and time series.")
	remoteReadLookBack     = flag.Duration("remoteRead.lookback", time.Hour, "Lookback defines how far to look into past for alerts timeseries. "+
		"For example, if lookback=1h then range from now() to now()-1h will be scanned.")
)

// Group is an entity for grouping rules
type Group struct {
	mu sync.RWMutex
	// id stores the unique id for the group, and shouldn't change after the group create.
	id         uint64
	Name       string
	File       string
	Rules      []Rule
	Type       config.Type
	Interval   time.Duration
	EvalOffset *time.Duration
	// EvalDelay will adjust timestamp for rule evaluation requests to compensate intentional query delay from datasource.
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5155
	EvalDelay   *time.Duration
	Limit       int
	Concurrency int
	// checksum stores the hash of yaml definition for this group.
	checksum       string
	LastEvaluation time.Time
	Debug          bool

	Labels          map[string]string
	Params          url.Values
	Headers         map[string]string
	NotifierHeaders map[string]string

	doneCh     chan struct{}
	finishedCh chan struct{}
	// channel accepts new Group obj
	// which supposed to update current group
	updateCh chan *Group
	// evalCancel stores the cancel fn for interrupting
	// rules evaluation. Used on groups update() and close().
	evalCancel context.CancelFunc
	// metrics contains metrics for group and its rules, will be created during Init()
	metrics *groupMetrics
	// evalAlignment will make the timestamp of group query
	// requests be aligned with interval
	evalAlignment *bool
}

type groupMetrics struct {
	set *metrics.Set

	iterationTotal    *metrics.Counter
	iterationDuration *metrics.Summary
	iterationMissed   *metrics.Counter
	iterationInterval *metrics.Gauge
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

// NewGroup returns a new group
func NewGroup(cfg config.Group, qb datasource.QuerierBuilder, defaultInterval time.Duration, labels map[string]string) *Group {
	g := &Group{
		Type:            cfg.Type,
		Name:            cfg.Name,
		File:            cfg.File,
		Interval:        cfg.Interval.Duration(),
		Limit:           cfg.Limit,
		Concurrency:     cfg.Concurrency,
		checksum:        cfg.Checksum,
		Params:          cfg.Params,
		Headers:         make(map[string]string),
		NotifierHeaders: make(map[string]string),
		Labels:          cfg.Labels,
		Debug:           cfg.Debug,
		evalAlignment:   cfg.EvalAlignment,

		doneCh:     make(chan struct{}),
		finishedCh: make(chan struct{}),
		updateCh:   make(chan *Group),
	}
	if g.Interval == 0 {
		g.Interval = defaultInterval
	}
	if g.Concurrency < 1 {
		g.Concurrency = 1
	}
	if cfg.EvalOffset != nil {
		g.EvalOffset = &cfg.EvalOffset.D
	}
	if cfg.EvalDelay != nil {
		g.EvalDelay = &cfg.EvalDelay.D
	}
	g.id = g.CreateID()
	for _, h := range cfg.Headers {
		g.Headers[h.Key] = h.Value
	}
	for _, h := range cfg.NotifierHeaders {
		g.NotifierHeaders[h.Key] = h.Value
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

func (g *Group) newRule(qb datasource.QuerierBuilder, r config.Rule) Rule {
	if r.Alert != "" {
		return NewAlertingRule(qb, g, r)
	}
	return NewRecordingRule(qb, g, r)
}

// GetCheckSum returns group checksum
func (g *Group) GetCheckSum() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.checksum
}

// GetID returns the unique group ID
func (g *Group) GetID() uint64 {
	return g.id
}

// CreateID returns the unique ID based on group basic fields.
// Should only be called when creating new group,
// and use GetID() afterward.
func (g *Group) CreateID() uint64 {
	hash := fnv.New64a()
	hash.Write([]byte(g.File))
	hash.Write([]byte("\xff"))
	hash.Write([]byte(g.Name))
	hash.Write([]byte(g.Type.Get()))
	hash.Write([]byte(g.Interval.String()))
	if g.EvalOffset != nil {
		hash.Write([]byte(g.EvalOffset.String()))
	}
	return hash.Sum64()
}

// restore restores alerts state for group rules
func (g *Group) restore(ctx context.Context, qb datasource.QuerierBuilder, ts time.Time, lookback time.Duration) error {
	for _, rule := range g.Rules {
		ar, ok := rule.(*AlertingRule)
		if !ok {
			continue
		}
		if ar.For < 1 {
			continue
		}
		q := qb.BuildWithParams(datasource.QuerierParams{
			EvaluationInterval: g.Interval,
			QueryParams:        g.Params,
			Headers:            g.Headers,
			Debug:              ar.Debug,
		})
		if err := ar.restore(ctx, q, ts, lookback); err != nil {
			return fmt.Errorf("error while restoring rule %q: %w", rule, err)
		}
	}
	return nil
}

// updateWith updates existing group with
// passed group object. This function ignores group
// evaluation interval change. It supposed to be updated
// in group.Start function.
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
			g.Rules[i].unregisterMetrics()
			g.Rules[i] = nil
			continue
		}
		if err := or.updateWith(nr); err != nil {
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
		nr.registerMetrics(g.metrics.set)
		newRules = append(newRules, nr)
	}

	g.Concurrency = newGroup.Concurrency
	g.Params = newGroup.Params
	g.Headers = newGroup.Headers
	g.NotifierHeaders = newGroup.NotifierHeaders
	g.Labels = newGroup.Labels
	g.Limit = newGroup.Limit
	g.checksum = newGroup.checksum
	g.Rules = newRules
	g.Debug = newGroup.Debug
	return nil
}

// InterruptEval interrupts in-flight rules evaluations
// within the group. It is expected that g.evalCancel
// will be repopulated after the call.
func (g *Group) InterruptEval() {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.evalCancel != nil {
		g.evalCancel()
	}
}

// Close stops the group and it's rules, unregisters group metrics
func (g *Group) Close() {
	if g.doneCh == nil {
		return
	}
	close(g.doneCh)
	g.InterruptEval()
	<-g.finishedCh

	g.closeGroupMetrics()
}

func (g *Group) closeGroupMetrics() {
	metrics.UnregisterSet(g.metrics.set, true)
}

// SkipRandSleepOnGroupStart will skip random sleep delay in group first evaluation
var SkipRandSleepOnGroupStart bool

// Init must be called before group Start()
func (g *Group) Init() {
	ns := metrics.NewSet()
	g.metrics = &groupMetrics{set: ns}
	labels := fmt.Sprintf(`group=%q, file=%q`, g.Name, g.File)
	g.metrics.iterationTotal = g.metrics.set.NewCounter(fmt.Sprintf(`vmalert_iteration_total{%s}`, labels))
	g.metrics.iterationDuration = g.metrics.set.NewSummary(fmt.Sprintf(`vmalert_iteration_duration_seconds{%s}`, labels))
	g.metrics.iterationMissed = g.metrics.set.NewCounter(fmt.Sprintf(`vmalert_iteration_missed_total{%s}`, labels))
	g.metrics.iterationInterval = g.metrics.set.NewGauge(fmt.Sprintf(`vmalert_iteration_interval_seconds{%s}`, labels), func() float64 {
		i := g.Interval.Seconds()
		return i
	})
	for i := range g.Rules {
		g.Rules[i].registerMetrics(g.metrics.set)
	}
	metrics.RegisterSet(g.metrics.set)
}

// Start starts group's evaluation
func (g *Group) Start(ctx context.Context, nts func() []notifier.Notifier, rw remotewrite.RWClient, rr datasource.QuerierBuilder) {
	defer func() { close(g.finishedCh) }()
	evalTS := time.Now()
	// sleep random duration to spread group rules evaluation
	// over time in order to reduce load on datasource.
	if !SkipRandSleepOnGroupStart {
		sleepBeforeStart := delayBeforeStart(evalTS, g.GetID(), g.Interval, g.EvalOffset)
		g.infof("will start in %v", sleepBeforeStart)

		sleepTimer := time.NewTimer(sleepBeforeStart)
	randSleep:
		for {
			select {
			case <-ctx.Done():
				sleepTimer.Stop()
				return
			case <-g.doneCh:
				sleepTimer.Stop()
				return
			case ng := <-g.updateCh:
				g.mu.Lock()
				err := g.updateWith(ng)
				if err != nil {
					logger.Errorf("group %q: failed to update: %s", g.Name, err)
					g.mu.Unlock()
					continue
				}
				g.mu.Unlock()
				g.infof("reload successfully")
			case <-sleepTimer.C:
				break randSleep
			}
		}
		evalTS = evalTS.Add(sleepBeforeStart)
	}

	e := &executor{
		Rw:              rw,
		Notifiers:       nts,
		notifierHeaders: g.NotifierHeaders,
	}

	g.infof("started")

	eval := func(ctx context.Context, ts time.Time) {
		g.metrics.iterationTotal.Inc()

		start := time.Now()

		if len(g.Rules) < 1 {
			g.metrics.iterationDuration.UpdateDuration(start)
			g.LastEvaluation = start
			return
		}

		resolveDuration := getResolveDuration(g.Interval, *resendDelay, *maxResolveDuration)
		// adjust request timestamp using evalDelay and evalAlignment if necessary
		ts = g.adjustReqTimestamp(ts)
		errs := e.execConcurrently(ctx, g.Rules, ts, g.Concurrency, resolveDuration, g.Limit)
		for err := range errs {
			if err != nil {
				logger.Errorf("group %q: %s", g.Name, err)
			}
		}
		g.metrics.iterationDuration.UpdateDuration(start)
		g.LastEvaluation = start
	}

	evalCtx, cancel := context.WithCancel(ctx)
	g.mu.Lock()
	g.evalCancel = cancel
	g.mu.Unlock()
	defer g.evalCancel()

	eval(evalCtx, evalTS)

	t := time.NewTicker(g.Interval)
	defer t.Stop()

	// restore the rules state after the first evaluation
	// so only active alerts can be restored.
	if rr != nil {
		err := g.restore(ctx, rr, evalTS, *remoteReadLookBack)
		if err != nil {
			logger.Errorf("error while restoring ruleState for group %q: %s", g.Name, err)
		}
	}

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

			// it is expected that g.evalCancel will be evoked
			// somewhere else to unblock group from the rules evaluation.
			// we recreate the evalCtx and g.evalCancel, so it can
			// be called again.
			evalCtx, cancel = context.WithCancel(ctx)
			g.evalCancel = cancel

			err := g.updateWith(ng)
			if err != nil {
				logger.Errorf("group %q: failed to update: %s", g.Name, err)
				g.mu.Unlock()
				continue
			}

			e.notifierHeaders = g.NotifierHeaders
			g.mu.Unlock()

			g.infof("re-started")
		case <-t.C:
			missed := (time.Since(evalTS) / g.Interval) - 1
			if missed < 0 {
				// missed can become < 0 due to irregular delays during evaluation
				// which can result in time.Since(evalTS) < g.Interval
				missed = 0
			}
			if missed > 0 {
				g.metrics.iterationMissed.Inc()
			}
			evalTS = evalTS.Add((missed + 1) * g.Interval)

			eval(evalCtx, evalTS)
		}
	}
}

// UpdateWith inserts new group to updateCh
func (g *Group) UpdateWith(newGroup *Group) {
	g.updateCh <- newGroup
}

// DeepCopy returns a deep copy of group
func (g *Group) DeepCopy() *Group {
	g.mu.RLock()
	data, _ := json.Marshal(g)
	g.mu.RUnlock()
	newG := Group{}
	_ = json.Unmarshal(data, &newG)
	newG.Rules = g.Rules
	return &newG
}

// if offset is specified, delayBeforeStart returns a duration to help aligning timestamp with offset;
// otherwise, it returns a random duration between [0..interval] based on group key.
func delayBeforeStart(ts time.Time, key uint64, interval time.Duration, offset *time.Duration) time.Duration {
	if offset != nil {
		currentOffsetPoint := ts.Truncate(interval).Add(*offset)
		if currentOffsetPoint.Before(ts) {
			// wait until the next offset point
			return currentOffsetPoint.Add(interval).Sub(ts)
		}
		return currentOffsetPoint.Sub(ts)
	}

	var randSleep time.Duration
	randSleep = time.Duration(float64(interval) * (float64(key) / (1 << 64)))
	sleepOffset := time.Duration(ts.UnixNano() % interval.Nanoseconds())
	if randSleep < sleepOffset {
		randSleep += interval
	}
	randSleep -= sleepOffset
	return randSleep
}

func (g *Group) infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logger.Infof("group %q %s; interval=%v; eval_offset=%v; concurrency=%d",
		g.Name, msg, g.Interval, g.EvalOffset, g.Concurrency)
}

// Replay performs group replay
func (g *Group) Replay(start, end time.Time, rw remotewrite.RWClient, maxDataPoint, replayRuleRetryAttempts int, replayDelay time.Duration, disableProgressBar bool) int {
	var total int
	step := g.Interval * time.Duration(maxDataPoint)
	ri := rangeIterator{start: start, end: end, step: step}
	iterations := int(end.Sub(start)/step) + 1
	fmt.Printf("\nGroup %q"+
		"\ninterval: \t%v"+
		"\nrequests to make: \t%d"+
		"\nmax range per request: \t%v\n",
		g.Name, g.Interval, iterations, step)
	if g.Limit > 0 {
		fmt.Printf("\nPlease note, `limit: %d` param has no effect during replay.\n",
			g.Limit)
	}
	for _, rule := range g.Rules {
		fmt.Printf("> Rule %q (ID: %d)\n", rule, rule.ID())
		var bar *pb.ProgressBar
		if !disableProgressBar {
			bar = pb.StartNew(iterations)
		}
		ri.reset()
		for ri.next() {
			n, err := replayRule(rule, ri.s, ri.e, rw, replayRuleRetryAttempts)
			if err != nil {
				logger.Fatalf("rule %q: %s", rule, err)
			}
			total += n
			if bar != nil {
				bar.Increment()
			}
		}
		if bar != nil {
			bar.Finish()
		}
		// sleep to let remote storage to flush data on-disk
		// so chained rules could be calculated correctly
		time.Sleep(replayDelay)
	}
	return total
}

// ExecOnce evaluates all the rules under group for once with given timestamp.
func (g *Group) ExecOnce(ctx context.Context, nts func() []notifier.Notifier, rw remotewrite.RWClient, evalTS time.Time) chan error {
	e := &executor{
		Rw:              rw,
		Notifiers:       nts,
		notifierHeaders: g.NotifierHeaders,
	}
	if len(g.Rules) < 1 {
		return nil
	}
	resolveDuration := getResolveDuration(g.Interval, *resendDelay, *maxResolveDuration)
	return e.execConcurrently(ctx, g.Rules, evalTS, g.Concurrency, resolveDuration, g.Limit)
}

type rangeIterator struct {
	step       time.Duration
	start, end time.Time

	iter int
	s, e time.Time
}

func (ri *rangeIterator) reset() {
	ri.iter = 0
	ri.s, ri.e = time.Time{}, time.Time{}
}

func (ri *rangeIterator) next() bool {
	ri.s = ri.start.Add(ri.step * time.Duration(ri.iter))
	if !ri.end.After(ri.s) {
		return false
	}
	ri.e = ri.s.Add(ri.step)
	if ri.e.After(ri.end) {
		ri.e = ri.end
	}
	ri.iter++
	return true
}

// getResolveDuration returns the duration after which firing alert
// can be considered as resolved.
func getResolveDuration(groupInterval, delta, maxDuration time.Duration) time.Duration {
	if groupInterval > delta {
		delta = groupInterval
	}
	resolveDuration := delta * 4
	if maxDuration > 0 && resolveDuration > maxDuration {
		resolveDuration = maxDuration
	}
	return resolveDuration
}

func (g *Group) adjustReqTimestamp(timestamp time.Time) time.Time {
	// if `eval_offset` is specified, timestamp is already aligned with offset, do nothing
	if g.EvalOffset != nil {
		return timestamp
	}

	timestamp = timestamp.Add(-g.getEvalDelay())

	// apply the alignment as the last step
	if g.evalAlignment == nil || *g.evalAlignment {
		// align query time with interval to get similar result with grafana when plotting time series.
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5049
		// and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1232
		return timestamp.Truncate(g.Interval)
	}
	return timestamp
}

func (g *Group) getEvalDelay() time.Duration {
	if g.EvalDelay != nil {
		return *g.EvalDelay
	}
	return *evalDelay
}

// executor contains group's notify and rw configs
type executor struct {
	Notifiers       func() []notifier.Notifier
	notifierHeaders map[string]string

	Rw remotewrite.RWClient
}

// execConcurrently executes rules concurrently if concurrency>1
func (e *executor) execConcurrently(ctx context.Context, rules []Rule, ts time.Time, concurrency int, resolveDuration time.Duration, limit int) chan error {
	res := make(chan error, len(rules))
	if concurrency == 1 {
		// fast path
		for _, rule := range rules {
			res <- e.exec(ctx, rule, ts, resolveDuration, limit)
		}
		close(res)
		return res
	}

	sem := make(chan struct{}, concurrency)
	go func() {
		wg := sync.WaitGroup{}
		for _, r := range rules {
			sem <- struct{}{}
			wg.Add(1)
			go func(r Rule) {
				res <- e.exec(ctx, r, ts, resolveDuration, limit)
				<-sem
				wg.Done()
			}(r)
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
)

func (e *executor) exec(ctx context.Context, r Rule, ts time.Time, resolveDuration time.Duration, limit int) error {
	execTotal.Inc()

	tss, err := r.exec(ctx, ts, limit)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			// the context can be cancelled on graceful shutdown
			// or on group update. So no need to handle the error as usual.
			return nil
		}
		execErrors.Inc()
		return fmt.Errorf("rule %q: failed to execute: %w", r, err)
	}

	if e.Rw != nil {
		pushToRW := func(tss []prompbmarshal.TimeSeries) error {
			var lastErr error
			for _, ts := range tss {
				if err := e.Rw.Push(ts); err != nil {
					lastErr = fmt.Errorf("rule %q: remote write failure: %w", r, err)
				}
			}
			return lastErr
		}
		if err := pushToRW(tss); err != nil {
			return err
		}
	}

	ar, ok := r.(*AlertingRule)
	if !ok {
		return nil
	}

	alerts := ar.alertsToSend(resolveDuration, *resendDelay)
	if len(alerts) < 1 {
		return nil
	}

	wg := sync.WaitGroup{}
	errGr := new(vmalertutil.ErrGroup)
	for _, nt := range e.Notifiers() {
		wg.Add(1)
		go func(nt notifier.Notifier) {
			if err := nt.Send(ctx, alerts, e.notifierHeaders); err != nil {
				errGr.Add(fmt.Errorf("rule %q: failed to send alerts to addr %q: %w", r, nt.Addr(), err))
			}
			wg.Done()
		}(nt)
	}
	wg.Wait()
	return errGr.Err()
}
