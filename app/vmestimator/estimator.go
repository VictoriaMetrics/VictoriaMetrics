package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/axiomhq/hyperloglog"
	"github.com/dgryski/go-metro"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmestimator/protoparser"
)

type estimator struct {
	groupBy          []string
	groupByKeysLabel string

	groupLimit              int64
	groupSize               atomic.Int64
	groupRejectedMu         sync.Mutex
	groupRejectedSketch     *hyperloglog.Sketch
	groupRejectedSketchPrev *hyperloglog.Sketch

	buckets []*estimatorBucket

	metricsSet  *metrics.Set
	insertTotal *metrics.Counter

	stopCh chan struct{}
}

func newEstimator(cfg EstimatorConfig) (*estimator, error) {
	if cfg.Interval == 0 {
		cfg.Interval = time.Minute * 5
	}
	if cfg.GroupLimit <= 0 {
		cfg.GroupLimit = 10000
	}
	if cfg.Buckets <= 0 {
		cfg.Buckets = min(64, 2*cgroup.AvailableCPUs())
	}
	if cfg.HLLPrecision == 0 {
		cfg.HLLPrecision = 14
	}
	if cfg.HLLSparse == nil {
		cfg.HLLSparse = new(true)
	}

	metricPrefix := fmt.Sprintf("cardinality_estimate{interval=%q", cfg.Interval)
	if len(cfg.Labels) > 0 {
		keys := make([]string, 0, len(cfg.Labels))
		for k := range cfg.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			metricPrefix += fmt.Sprintf(",%s=%q", k, cfg.Labels[k])
		}
	}

	groupByKeysLabel := "__global__"
	if len(cfg.GroupBy) > 0 {
		groupByKeysLabel = strings.Join(cfg.GroupBy, `,`)
	}

	e := &estimator{
		groupBy:                 cfg.GroupBy,
		groupByKeysLabel:        groupByKeysLabel,
		groupLimit:              int64(cfg.GroupLimit),
		groupRejectedSketch:     mustNewGroupRejectSketch(),
		groupRejectedSketchPrev: mustNewGroupRejectSketch(),
		buckets:                 make([]*estimatorBucket, cfg.Buckets),
		metricsSet:              metrics.NewSet(),
		stopCh:                  make(chan struct{}),
	}

	e.insertTotal = e.metricsSet.NewCounter(
		fmt.Sprintf(`vmestimator_estimator_insert_total{group_by_keys=%q}`, e.groupByKeysLabel),
	)
	e.metricsSet.NewGauge(fmt.Sprintf(`vmestimator_estimator_group_rejected_size{group_by_keys=%q}`, e.groupByKeysLabel), func() float64 {
		e.groupRejectedMu.Lock()
		defer e.groupRejectedMu.Unlock()
		return float64(e.groupRejectedSketch.Estimate())
	})

	for i := 0; i < len(e.buckets); i++ {
		eb := &estimatorBucket{
			groupBy:             cfg.GroupBy,
			extraLabels:         cfg.Labels,
			interval:            cfg.Interval,
			metricPrefix:        metricPrefix,
			groupByKeysLabel:    groupByKeysLabel,
			groupLimit:          int64(cfg.GroupLimit),
			groupSize:           &e.groupSize,
			groupRejectedMu:     &e.groupRejectedMu,
			groupRejectedSketch: e.groupRejectedSketch,

			precision: cfg.HLLPrecision,
			sparse:    *cfg.HLLSparse,
		}

		if len(cfg.GroupBy) == 0 {
			eb.sketch = eb.newSketch()
		} else {
			eb.groups = make(map[string]groupSketch)
			eb.prevGroups = make(map[string]groupSketch)

			e.metricsSet.NewGauge(fmt.Sprintf(`vmestimator_estimator_group_size{group_by_keys=%q,bucket="%d"}`, eb.groupByKeysLabel, i), func() float64 {
				return float64(eb.groupSize.Load())
			})
			e.metricsSet.NewGauge(fmt.Sprintf(`vmestimator_estimator_group_limit{group_by_keys=%q,bucket="%d"}`, eb.groupByKeysLabel, i), func() float64 {
				return float64(eb.groupLimit)
			})
		}

		e.buckets[i] = eb
	}

	go e.runRotation(cfg.Interval)

	metrics.RegisterSet(e.metricsSet)

	return e, nil
}

func (e *estimator) stop() {
	close(e.stopCh)
	e.metricsSet.UnregisterAllMetrics()
}

var groupValuesPool = sync.Pool{}

func getGroupValuesKeySlice() *[]byte {
	v0 := groupValuesPool.Get()
	if v0 == nil {
		v := make([]byte, 128)
		return &v
	}

	return v0.(*[]byte)
}

func putGroupValuesSlice(key *[]byte) {
	if key == nil {
		return
	}

	*key = (*key)[:0]
	groupValuesPool.Put(key)
}

func (e *estimator) insertMany(tss []protoparser.TimeSerie) {
	bucketsNum := uint64(len(e.buckets))

	groupValuesKeyP := getGroupValuesKeySlice()
	groupValuesKey := *groupValuesKeyP
	defer func() {
		*groupValuesKeyP = groupValuesKey
		putGroupValuesSlice(groupValuesKeyP)
	}()

	groupValues := make([]string, len(e.groupBy))

	var cnt int
	for _, ts := range tss {
		if len(e.groupBy) == 0 {
			i := int(ts.Fingerprint % bucketsNum)
			e.buckets[i].insert(ts, "", nil)
			cnt++
			continue
		}

		groupValuesKey = groupValuesKey[:0]
		clear(groupValues)
		var hasNames bool
		for i, labelName := range e.groupBy {
			if i > 0 {
				groupValuesKey = append(groupValuesKey, ',')
			}

			for _, l := range ts.GroupLabels {
				if l.Name == labelName {
					hasNames = true

					groupValuesKey = append(groupValuesKey, l.Value...)
					groupValues[i] = l.Value
					break
				}
			}
		}

		// time series does not contribute to this groupBy
		if !hasNames {
			continue
		}

		i := int(hash(groupValuesKey) % bucketsNum)
		e.buckets[i].insert(ts, bytesutil.ToUnsafeString(groupValuesKey), groupValues)
		cnt++
	}

	e.insertTotal.Add(cnt)
}

func (e *estimator) reset() {
	e.groupSize.Store(0)
	for _, b := range e.buckets {
		b.reset()
	}

	e.groupRejectedMu.Lock()
	e.groupRejectedSketch.Reset()
	e.groupRejectedMu.Unlock()
}

func (e *estimator) writeMetrics(w io.Writer) {
	eb0 := e.buckets[0]

	if len(e.groupBy) == 0 {
		formatBuf := make([]byte, 0, 1024)
		resSK := eb0.newSketch()
		for _, eb := range e.buckets {
			eb.writeNoGroupMetric(resSK)
		}

		formatBuf = append(formatBuf, eb0.metricPrefix...)
		formatBuf = append(formatBuf, `,group_by_keys="__global__"} `...)
		formatBuf = strconv.AppendUint(formatBuf, resSK.Estimate(), 10)
		formatBuf = append(formatBuf, "\n"...)
		if _, err := w.Write(formatBuf); err != nil {
			logger.Errorf("writing metrics failed: %s; written cardinality metrics might be incomplete or invalid", err)
		}
		return
	}

	formatBuf := make([]byte, 0, 16384)
	formatBuf = append(formatBuf, eb0.metricPrefix...)
	formatBuf = append(formatBuf, `,group_by_keys="`...)
	formatBuf = append(formatBuf, eb0.groupByKeysLabel...)
	formatBuf = append(formatBuf, `",group_by_values=`...)

	prefixLen := len(formatBuf)
	resSK := eb0.newSketch()
	for _, eb := range e.buckets {
		formatBuf = eb.writeGroupMetrics(w, resSK, formatBuf[:prefixLen])
	}

	groupSize := e.groupSize.Load()
	if groupSize >= int64(float64(e.groupLimit)*0.8) {
		e.groupRejectedMu.Lock()
		res := mustNewGroupRejectSketch()
		if err := res.Merge(e.groupRejectedSketch); err != nil {
			logger.Fatalf("BUG: groupRejectedSketch merge failed: %s", err)
		}
		if err := res.Merge(e.groupRejectedSketchPrev); err != nil {
			logger.Fatalf("BUG: groupRejectedSketchPrev merge failed: %s", err)
		}
		e.groupRejectedMu.Unlock()

		groupSize += int64(res.Estimate())
	}

	formatBuf = formatBuf[:0]
	formatBuf = append(formatBuf, eb0.metricPrefix...)
	formatBuf = append(formatBuf, `,group_by_keys="__group__",group_by_values="`...)
	formatBuf = append(formatBuf, eb0.groupByKeysLabel...)
	formatBuf = append(formatBuf, `"} `...)
	formatBuf = strconv.AppendInt(formatBuf, groupSize, 10)
	formatBuf = append(formatBuf, "\n"...)
	if _, err := w.Write(formatBuf); err != nil {
		logger.Errorf("writing metrics failed: %s; written cardinality metrics might be incomplete or invalid", err)
	}
}

func (e *estimator) runRotation(interval time.Duration) {
	t := time.NewTicker(interval / 2)
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

func (e *estimator) rotate() {
	e.groupSize.Store(0)

	var wg sync.WaitGroup
	for i := range e.buckets {
		wg.Go(e.buckets[i].rotate)
	}
	wg.Wait()

	e.groupRejectedMu.Lock()
	prevSK := e.groupRejectedSketchPrev
	prevSK.Reset()
	e.groupRejectedSketchPrev = e.groupRejectedSketch
	e.groupRejectedSketch = prevSK
	e.groupRejectedMu.Unlock()
}

type estimatorBucket struct {
	mu sync.Mutex

	groupBy          []string
	groupLimit       int64
	extraLabels      map[string]string
	interval         time.Duration
	metricPrefix     string
	groupByKeysLabel string
	precision        uint8
	sparse           bool

	sketch     *hyperloglog.Sketch
	prevSketch *hyperloglog.Sketch

	groupSize  *atomic.Int64
	groups     map[string]groupSketch
	prevGroups map[string]groupSketch

	groupRejectedMu     *sync.Mutex
	groupRejectedSketch *hyperloglog.Sketch
}

func (eb *estimatorBucket) String() string {
	return fmt.Sprintf(
		"interval: %s; group_by: %v; extra_labels: %v", eb.interval, eb.groupBy, eb.extraLabels)
}

func (eb *estimatorBucket) reset() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if len(eb.groupBy) == 0 {
		eb.prevSketch.Reset()
		eb.sketch.Reset()
		return
	}

	eb.groups = make(map[string]groupSketch)
	eb.prevGroups = make(map[string]groupSketch)
}

func (eb *estimatorBucket) rotate() {
	if len(eb.groupBy) == 0 {
		eb.mu.Lock()
		eb.prevSketch = eb.sketch
		eb.sketch = eb.newSketch()
		eb.mu.Unlock()
		return
	}

	eb.mu.Lock()
	eb.prevGroups = eb.groups
	eb.groups = make(map[string]groupSketch, len(eb.groups))
	eb.mu.Unlock()

	eb.groupSize.Add(int64(len(eb.prevGroups)))
}

func (eb *estimatorBucket) insert(ts protoparser.TimeSerie, groupValuesKey string, groupValues []string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if len(eb.groupBy) == 0 {
		eb.sketch.InsertHash(ts.Fingerprint)
		return
	}

	gsk, ok := eb.groups[groupValuesKey]
	if !ok {
		if _, ok := eb.prevGroups[groupValuesKey]; !ok {
			groupSize := eb.groupSize.Load()
			if groupSize+1 > eb.groupLimit {
				eb.groupRejectedMu.Lock()
				eb.groupRejectedSketch.InsertHash(hash([]byte(groupValuesKey)))
				eb.groupRejectedMu.Unlock()
				return
			}

			eb.groupSize.Add(1)
		}

		formatBuf := make([]byte, 0, 1024)
		formatBuf = strconv.AppendQuote(formatBuf, groupValuesKey)
		for i := range groupValues {
			formatBuf = append(formatBuf, ',')
			if eb.groupBy[i] == `__name__` {
				formatBuf = append(formatBuf, `by__name__`...)
			} else {
				formatBuf = append(formatBuf, `by_`...)
				formatBuf = append(formatBuf, eb.groupBy[i]...)
			}
			formatBuf = append(formatBuf, '=')
			formatBuf = strconv.AppendQuote(formatBuf, groupValues[i])
		}
		formatBuf = append(formatBuf, `} `...)

		gsk = groupSketch{
			groupValueLabels: bytesutil.ToUnsafeString(formatBuf),

			Sketch: eb.newSketch(),
		}

		eb.groups[strings.Clone(groupValuesKey)] = gsk
	}
	gsk.InsertHash(ts.Fingerprint)
}

func (eb *estimatorBucket) writeNoGroupMetric(res *hyperloglog.Sketch) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.mergeSketches(eb.sketch, eb.prevSketch, res)
}

func (eb *estimatorBucket) writeGroupMetrics(w io.Writer, res *hyperloglog.Sketch, formatBuf []byte) []byte {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	prefixLen := len(formatBuf)

	for valuesKey, gsk := range eb.groups {
		res.Reset()
		formatBuf = formatBuf[:prefixLen]

		formatBuf = append(formatBuf, gsk.groupValueLabels...)

		eb.mergeSketches(gsk.Sketch, eb.prevGroups[valuesKey].Sketch, res)
		formatBuf = strconv.AppendUint(formatBuf, res.Estimate(), 10)
		formatBuf = append(formatBuf, "\n"...)
		if _, err := w.Write(formatBuf); err != nil {
			logger.Errorf("writing metrics failed: %s; written cardinality metrics might be incomplete or invalid", err)
		}
	}

	for valuesKey := range eb.prevGroups {
		if _, ok := eb.groups[valuesKey]; ok {

			continue
		}

		res.Reset()
		formatBuf = formatBuf[:prefixLen]

		gsk := eb.prevGroups[valuesKey]
		formatBuf = append(formatBuf, gsk.groupValueLabels...)

		eb.mergeSketches(nil, eb.prevGroups[valuesKey].Sketch, res)
		formatBuf = strconv.AppendUint(formatBuf, res.Estimate(), 10)
		formatBuf = append(formatBuf, "\n"...)
		if _, err := w.Write(formatBuf); err != nil {
			logger.Errorf("writing metrics failed: %s; written cardinality metrics might be incomplete or invalid", err)
		}
	}

	return formatBuf[:prefixLen]
}

func (eb *estimatorBucket) mergeSketches(cur, prev, res *hyperloglog.Sketch) {
	if err := res.Merge(cur); err != nil {
		panic(err)
	}
	if prev != nil {
		if err := res.Merge(prev); err != nil {
			panic(err)
		}
	}
}

func (eb *estimatorBucket) newSketch() *hyperloglog.Sketch {
	return mustNewSketch(eb.precision, eb.sparse)
}

type groupSketch struct {
	groupValueLabels string

	*hyperloglog.Sketch
}

func mustNewGroupRejectSketch() *hyperloglog.Sketch {
	return mustNewSketch(10, true)
}

func mustNewSketch(precision uint8, sparse bool) *hyperloglog.Sketch {
	sk, err := hyperloglog.NewSketch(precision, sparse)
	if err != nil {
		panic(fmt.Sprintf("cannot create HLL sketch with precision=%d and sparse=%v: %s", precision, sparse, err))
	}

	return sk
}

func hash(v []byte) uint64 {
	return metro.Hash64(v, 1337)
}
