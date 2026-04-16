package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/axiomhq/hyperloglog"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// Estimator tracks cardinality for a single stream configuration.
type Estimator struct {
	groupBy     []string          // label names to group by; empty means no grouping
	extraLabels map[string]string // extra labels added to output metrics
	filterStr   string
	filters     []labelFilter // compiled from config filter
	interval    time.Duration

	mu         sync.Mutex
	sketch     *hyperloglog.Sketch            // current-interval sketch; used when group == ""
	prevSketch *hyperloglog.Sketch            // previous-interval sketch for smooth rotation
	groups     map[string]*hyperloglog.Sketch // current-interval per-group sketches; used when group != ""
	prevGroups map[string]*hyperloglog.Sketch // previous-interval per-group sketches for smooth rotation

	stopCh chan struct{} // closed by Stop to terminate the rotation goroutine
}

func (e *Estimator) String() string {
	return fmt.Sprintf(
		"filter: %s; interval: %s; group_by: %v; extra_labels: %v",
		e.filterStr, e.interval, e.groupBy, e.extraLabels)
}

func newEstimator(cfg EstimatorConfig) (*Estimator, error) {
	if cfg.Interval == 0 {
		cfg.Interval = Duration(time.Minute * 5)
	}

	e := &Estimator{
		groupBy:     cfg.Group,
		extraLabels: cfg.Labels,
		filterStr:   cfg.Filter,
		interval:    time.Duration(cfg.Interval),
		stopCh:      make(chan struct{}),
	}

	filters, err := compileFilters(cfg.Filter)
	if err != nil {
		return nil, fmt.Errorf("stream: %s: parse filters failed: %w", e, err)
	}
	e.filters = filters

	if len(cfg.Group) == 0 {
		sk, err := hyperloglog.NewSketch(14, true)
		if err != nil {
			return nil, fmt.Errorf("cannot create HLL sketch for stream %s: %w", e, err)
		}
		e.sketch = sk
	} else {
		e.groups = make(map[string]*hyperloglog.Sketch)
	}

	go e.runRotation()

	return e, nil
}

// Stop stops the background rotation goroutine, if any.
func (e *Estimator) Stop() {
	close(e.stopCh)
}

// runRotation resets the sketches on every tick until stopCh is closed.
func (e *Estimator) runRotation() {
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			e.rotate()
		case <-e.stopCh:
			return
		}
	}
}

// rotate promotes current sketches to previous and starts fresh current sketches.
// Estimates are computed as the union of previous and current (see estimateSketch /
// estimateGroup), so cardinality does not drop to zero immediately after rotation.
func (e *Estimator) rotate() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.groupBy) == 0 {
		sk, err := hyperloglog.NewSketch(14, true)
		if err != nil {
			return
		}
		e.prevSketch = e.sketch
		e.sketch = sk
		return
	}
	e.prevGroups = e.groups
	e.groups = make(map[string]*hyperloglog.Sketch)
}

// insert adds a time series to the estimator if it matches the configured filter.
func (e *Estimator) insert(labels []prompb.Label) {
	if !matchesFilters(labels, e.filters) {
		return
	}
	fp := fingerprintLabels(labels)

	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.groupBy) == 0 {
		e.sketch.Insert(fp)
		return
	}

	var key []byte
	first := true
	for _, labelName := range e.groupBy {
		if !first {
			key = append(key, ',')
		}

		for _, l := range labels {
			if l.Name == labelName {
				key = append(key, l.Value...)
				break
			}
		}

		first = false
	}
	groupKey := string(key)
	sk := e.groups[groupKey]
	if sk == nil {
		var err error
		sk, err = hyperloglog.NewSketch(14, true)
		if err != nil {
			panic(fmt.Sprintf("FATAL: cannot create HLL sketch for stream %s: %s", e, err))
		}
		e.groups[groupKey] = sk
	}
	sk.Insert(fp)
}

// writeMetrics writes cardinality_estimate metrics to w in Prometheus text format.
func (e *Estimator) writeMetrics(w io.Writer) {
	e.mu.Lock()
	defer e.mu.Unlock()

	metricPrefix := fmt.Sprintf("cardinality_estimate{interval=%q,filter=%q,group_by_keys=%q",
		e.interval, e.filterStr, strings.Join(e.groupBy, ","))

	if len(e.extraLabels) > 0 {
		keys := make([]string, 0, len(e.extraLabels))
		for k := range e.extraLabels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			metricPrefix += fmt.Sprintf(",%s=%q", k, e.extraLabels[k])
		}
	}

	if len(e.groupBy) == 0 {
		card := e.estimateSketch(e.sketch, e.prevSketch)
		fmt.Fprintf(w, "%s} %d\n", metricPrefix, card)
		return
	}

	// Collect all group keys from both current and previous intervals.
	keys := make(map[string]struct{}, len(e.groups)+len(e.prevGroups))
	for k := range e.groups {
		keys[k] = struct{}{}
	}
	for k := range e.prevGroups {
		keys[k] = struct{}{}
	}
	for groupKey := range keys {
		card := e.estimateSketch(e.groups[groupKey], e.prevGroups[groupKey])
		fmt.Fprintf(w, "%s,group_by_values=%q} %d\n", metricPrefix, groupKey, card)
	}
}

// estimateSketch returns the cardinality estimate for the union of cur and prev.
// If prev is nil (no rotation has happened yet, or no previous interval data),
// only cur is used.  This prevents an abrupt drop to zero right after rotation.
func (e *Estimator) estimateSketch(cur, prev *hyperloglog.Sketch) uint64 {
	if cur == nil && prev == nil {
		return 0
	}
	if prev == nil {
		return cur.Estimate()
	}
	if cur == nil {
		return prev.Estimate()
	}
	// Merge into a temporary copy so the originals are not mutated.
	merged := cur.Clone()
	if err := merged.Merge(prev); err != nil {
		// Merge only fails on precision mismatch; both sketches use precision 14.
		return cur.Estimate()
	}
	return merged.Estimate()
}

// fingerprintLabels returns a byte slice that uniquely identifies a label set.
// The Prometheus remote write protocol guarantees labels are sorted by name,
// so no additional sorting is needed.
func fingerprintLabels(labels []prompb.Label) []byte {
	var b []byte
	for _, l := range labels {
		b = append(b, l.Name...)
		b = append(b, 0x00)
		b = append(b, l.Value...)
		b = append(b, 0x00)
	}
	return b
}

// matchesFilters returns true if all filters are satisfied by labels.
func matchesFilters(labels []prompb.Label, filters []labelFilter) bool {
	for _, f := range filters {
		val := ""
		for _, l := range labels {
			if l.Name == f.label {
				val = l.Value
				break
			}
		}
		var matched bool
		if f.isRegexp {
			matched = f.re.MatchString(val)
		} else {
			matched = val == f.value
		}
		if f.isNegative {
			matched = !matched
		}
		if !matched {
			return false
		}
	}
	return true
}
