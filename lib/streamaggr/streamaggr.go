package streamaggr

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"gopkg.in/yaml.v2"
)

var supportedOutputs = []string{
	"total",
	"increase",
	"count_series",
	"count_samples",
	"sum_samples",
	"last",
	"min",
	"max",
	"avg",
	"stddev",
	"stdvar",
	"histogram_bucket",
	"quantiles(phi1, ..., phiN)",
}

// LoadFromFile loads Aggregators from the given path and uses the given pushFunc for pushing the aggregated data.
//
// If dedupInterval > 0, then the input samples are de-duplicated before being aggregated,
// e.g. only the last sample per each time series per each dedupInterval is aggregated.
//
// The returned Aggregators must be stopped with MustStop() when no longer needed.
func LoadFromFile(path string, pushFunc PushFunc, dedupInterval time.Duration) (*Aggregators, error) {
	data, err := fscore.ReadFileOrHTTP(path)
	if err != nil {
		return nil, fmt.Errorf("cannot load aggregators: %w", err)
	}
	data, err = envtemplate.ReplaceBytes(data)
	if err != nil {
		return nil, fmt.Errorf("cannot expand environment variables in %q: %w", path, err)
	}

	as, err := newAggregatorsFromData(data, pushFunc, dedupInterval)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize aggregators from %q: %w", path, err)
	}

	return as, nil
}

func newAggregatorsFromData(data []byte, pushFunc PushFunc, dedupInterval time.Duration) (*Aggregators, error) {
	var cfgs []*Config
	if err := yaml.UnmarshalStrict(data, &cfgs); err != nil {
		return nil, fmt.Errorf("cannot parse stream aggregation config: %w", err)
	}
	return NewAggregators(cfgs, pushFunc, dedupInterval)
}

// Config is a configuration for a single stream aggregation.
type Config struct {
	// Match is a label selector for filtering time series for the given selector.
	//
	// If the match isn't set, then all the input time series are processed.
	Match *promrelabel.IfExpression `yaml:"match,omitempty"`

	// Interval is the interval between aggregations.
	Interval string `yaml:"interval"`

	// Staleness interval is interval after which the series state will be reset if no samples have been sent during it.
	// The parameter is only relevant for outputs: total, increase and histogram_bucket.
	StalenessInterval string `yaml:"staleness_interval,omitempty"`

	// Outputs is a list of output aggregate functions to produce.
	//
	// The following names are allowed:
	//
	// - total - aggregates input counters
	// - increase - counts the increase over input counters
	// - count_series - counts the input series
	// - count_samples - counts the input samples
	// - sum_samples - sums the input samples
	// - last - the last biggest sample value
	// - min - the minimum sample value
	// - max - the maximum sample value
	// - avg - the average value across all the samples
	// - stddev - standard deviation across all the samples
	// - stdvar - standard variance across all the samples
	// - histogram_bucket - creates VictoriaMetrics histogram for input samples
	// - quantiles(phi1, ..., phiN) - quantiles' estimation for phi in the range [0..1]
	//
	// The output time series will have the following names:
	//
	//   input_name:aggr_<interval>_<output>
	//
	Outputs []string `yaml:"outputs"`

	// By is an optional list of labels for grouping input series.
	//
	// See also Without.
	//
	// If neither By nor Without are set, then the Outputs are calculated
	// individually per each input time series.
	By []string `yaml:"by,omitempty"`

	// Without is an optional list of labels, which must be excluded when grouping input series.
	//
	// See also By.
	//
	// If neither By nor Without are set, then the Outputs are calculated
	// individually per each input time series.
	Without []string `yaml:"without,omitempty"`

	// InputRelabelConfigs is an optional relabeling rules, which are applied on the input
	// before aggregation.
	InputRelabelConfigs []promrelabel.RelabelConfig `yaml:"input_relabel_configs,omitempty"`

	// OutputRelabelConfigs is an optional relabeling rules, which are applied
	// on the aggregated output before being sent to remote storage.
	OutputRelabelConfigs []promrelabel.RelabelConfig `yaml:"output_relabel_configs,omitempty"`

	// FlushOnShutdown defines whether to flush the aggregation state on process termination
	// or config reload. Is `false` by default.
	FlushOnShutdown bool `yaml:"flush_on_shutdown,omitempty"`
}

// Aggregators aggregates metrics passed to Push and calls pushFunc for aggregate data.
type Aggregators struct {
	as []*aggregator

	// configData contains marshaled configs passed to NewAggregators().
	// It is used in Equal() for comparing Aggregators.
	configData []byte
}

// NewAggregators creates Aggregators from the given cfgs.
//
// pushFunc is called when the aggregated data must be flushed.
//
// If dedupInterval > 0, then the input samples are de-duplicated before being aggregated,
// e.g. only the last sample per each time series per each dedupInterval is aggregated.
//
// MustStop must be called on the returned Aggregators when they are no longer needed.
func NewAggregators(cfgs []*Config, pushFunc PushFunc, dedupInterval time.Duration) (*Aggregators, error) {
	as := make([]*aggregator, len(cfgs))
	for i, cfg := range cfgs {
		a, err := newAggregator(cfg, pushFunc, dedupInterval)
		if err != nil {
			// Stop already initialized aggregators before returning the error.
			for _, a := range as[:i] {
				a.MustStop()
			}
			return nil, fmt.Errorf("cannot initialize aggregator #%d: %w", i, err)
		}
		as[i] = a
	}
	configData, err := json.Marshal(cfgs)
	if err != nil {
		logger.Panicf("BUG: cannot marshal the provided configs: %s", err)
	}
	return &Aggregators{
		as:         as,
		configData: configData,
	}, nil
}

// MustStop stops a.
func (a *Aggregators) MustStop() {
	if a == nil {
		return
	}
	for _, aggr := range a.as {
		aggr.MustStop()
	}
}

// Equal returns true if a and b are initialized from identical configs.
func (a *Aggregators) Equal(b *Aggregators) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return string(a.configData) == string(b.configData)
}

// Push pushes tss to a.
//
// Push sets matchIdxs[idx] to 1 if the corresponding tss[idx] was used in aggregations.
// Otherwise matchIdxs[idx] is set to 0.
//
// Push returns matchIdxs with len equal to len(tss).
// It re-uses the matchIdxs if it has enough capacity to hold len(tss) items.
// Otherwise it allocates new matchIdxs.
func (a *Aggregators) Push(tss []prompbmarshal.TimeSeries, matchIdxs []byte) []byte {
	matchIdxs = bytesutil.ResizeNoCopyMayOverallocate(matchIdxs, len(tss))
	for i := 0; i < len(matchIdxs); i++ {
		matchIdxs[i] = 0
	}

	if a != nil {
		for _, aggr := range a.as {
			aggr.Push(tss, matchIdxs)
		}
	}
	return matchIdxs
}

// aggregator aggregates input series according to the config passed to NewAggregator
type aggregator struct {
	match *promrelabel.IfExpression

	inputRelabeling  *promrelabel.ParsedConfigs
	outputRelabeling *promrelabel.ParsedConfigs

	by                  []string
	without             []string
	aggregateOnlyByTime bool

	// dedupAggr is set to non-nil if input samples must be de-duplicated according
	// to the dedupInterval passed to newAggregator().
	dedupAggr *lastAggrState

	// aggrStates contains aggregate states for the given outputs
	aggrStates []aggrState

	pushFunc PushFunc

	// suffix contains a suffix, which should be added to aggregate metric names
	//
	// It contains the interval, labels in (by, without), plus output name.
	// For example, foo_bar metric name is transformed to foo_bar:1m_by_job
	// for `interval: 1m`, `by: [job]`
	suffix string

	// flushOnShutdown defines whether to flush the state of aggregation
	// on MustStop call.
	flushOnShutdown bool

	wg     sync.WaitGroup
	stopCh chan struct{}
}

type aggrState interface {
	pushSample(inputKey, outputKey string, value float64)
	appendSeriesForFlush(ctx *flushCtx)
}

// PushFunc is called by Aggregators when it needs to push its state to metrics storage
type PushFunc func(tss []prompbmarshal.TimeSeries)

// newAggregator creates new aggregator for the given cfg, which pushes the aggregate data to pushFunc.
//
// If dedupInterval > 0, then the input samples are de-duplicated before being aggregated,
// e.g. only the last sample per each time series per each dedupInterval is aggregated.
//
// The returned aggregator must be stopped when no longer needed by calling MustStop().
func newAggregator(cfg *Config, pushFunc PushFunc, dedupInterval time.Duration) (*aggregator, error) {
	// check cfg.Interval
	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `interval: %q`: %w", cfg.Interval, err)
	}
	if interval <= time.Second {
		return nil, fmt.Errorf("aggregation interval should be higher than 1s; got %s", interval)
	}

	// check cfg.StalenessInterval
	stalenessInterval := interval * 2
	if cfg.StalenessInterval != "" {
		stalenessInterval, err = time.ParseDuration(cfg.StalenessInterval)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `staleness_interval: %q`: %w", cfg.StalenessInterval, err)
		}
		if stalenessInterval < interval {
			return nil, fmt.Errorf("staleness_interval cannot be less than interval (%s); got %s", cfg.Interval, cfg.StalenessInterval)
		}
	}

	// initialize input_relabel_configs and output_relabel_configs
	inputRelabeling, err := promrelabel.ParseRelabelConfigs(cfg.InputRelabelConfigs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse input_relabel_configs: %w", err)
	}
	outputRelabeling, err := promrelabel.ParseRelabelConfigs(cfg.OutputRelabelConfigs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse output_relabel_configs: %w", err)
	}

	// check by and without lists
	by := sortAndRemoveDuplicates(cfg.By)
	without := sortAndRemoveDuplicates(cfg.Without)
	if len(by) > 0 && len(without) > 0 {
		return nil, fmt.Errorf("`by: %s` and `without: %s` lists cannot be set simultaneously", by, without)
	}
	aggregateOnlyByTime := (len(by) == 0 && len(without) == 0)
	if !aggregateOnlyByTime && len(without) == 0 {
		by = addMissingUnderscoreName(by)
	}

	// initialize outputs list
	if len(cfg.Outputs) == 0 {
		return nil, fmt.Errorf("`outputs` list must contain at least a single entry from the list %s; "+
			"see https://docs.victoriametrics.com/stream-aggregation.html", supportedOutputs)
	}
	aggrStates := make([]aggrState, len(cfg.Outputs))
	for i, output := range cfg.Outputs {
		if strings.HasPrefix(output, "quantiles(") {
			if !strings.HasSuffix(output, ")") {
				return nil, fmt.Errorf("missing closing brace for `quantiles()` output")
			}
			argsStr := output[len("quantiles(") : len(output)-1]
			if len(argsStr) == 0 {
				return nil, fmt.Errorf("`quantiles()` must contain at least one phi")
			}
			args := strings.Split(argsStr, ",")
			phis := make([]float64, len(args))
			for j, arg := range args {
				arg = strings.TrimSpace(arg)
				phi, err := strconv.ParseFloat(arg, 64)
				if err != nil {
					return nil, fmt.Errorf("cannot parse phi=%q for quantiles(%s): %w", arg, argsStr, err)
				}
				if phi < 0 || phi > 1 {
					return nil, fmt.Errorf("phi inside quantiles(%s) must be in the range [0..1]; got %v", argsStr, phi)
				}
				phis[j] = phi
			}
			aggrStates[i] = newQuantilesAggrState(phis)
			continue
		}
		switch output {
		case "total":
			aggrStates[i] = newTotalAggrState(interval, stalenessInterval)
		case "increase":
			aggrStates[i] = newIncreaseAggrState(interval, stalenessInterval)
		case "count_series":
			aggrStates[i] = newCountSeriesAggrState()
		case "count_samples":
			aggrStates[i] = newCountSamplesAggrState()
		case "sum_samples":
			aggrStates[i] = newSumSamplesAggrState()
		case "last":
			aggrStates[i] = newLastAggrState()
		case "min":
			aggrStates[i] = newMinAggrState()
		case "max":
			aggrStates[i] = newMaxAggrState()
		case "avg":
			aggrStates[i] = newAvgAggrState()
		case "stddev":
			aggrStates[i] = newStddevAggrState()
		case "stdvar":
			aggrStates[i] = newStdvarAggrState()
		case "histogram_bucket":
			aggrStates[i] = newHistogramBucketAggrState(stalenessInterval)
		default:
			return nil, fmt.Errorf("unsupported output=%q; supported values: %s; "+
				"see https://docs.victoriametrics.com/stream-aggregation.html", output, supportedOutputs)
		}
	}

	// initialize suffix to add to metric names after aggregation
	suffix := ":" + cfg.Interval
	if labels := removeUnderscoreName(by); len(labels) > 0 {
		suffix += fmt.Sprintf("_by_%s", strings.Join(labels, "_"))
	}
	if labels := removeUnderscoreName(without); len(labels) > 0 {
		suffix += fmt.Sprintf("_without_%s", strings.Join(labels, "_"))
	}
	suffix += "_"

	var dedupAggr *lastAggrState
	if dedupInterval > 0 {
		dedupAggr = newLastAggrState()
	}

	// initialize the aggregator
	a := &aggregator{
		match: cfg.Match,

		inputRelabeling:  inputRelabeling,
		outputRelabeling: outputRelabeling,

		by:                  by,
		without:             without,
		aggregateOnlyByTime: aggregateOnlyByTime,

		dedupAggr:  dedupAggr,
		aggrStates: aggrStates,
		pushFunc:   pushFunc,

		suffix:          suffix,
		flushOnShutdown: cfg.FlushOnShutdown,

		stopCh: make(chan struct{}),
	}

	if dedupAggr != nil {
		a.wg.Add(1)
		go func() {
			a.runDedupFlusher(dedupInterval)
			a.wg.Done()
		}()
	}
	a.wg.Add(1)
	go func() {
		a.runFlusher(interval)
		a.wg.Done()
	}()

	return a, nil
}

func (a *aggregator) runDedupFlusher(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-a.stopCh:
			return
		case <-t.C:
		}

		// Globally limit the concurrency for metrics' flush
		// in order to limit memory usage when big number of aggregators
		// are flushed at the same time.
		flushConcurrencyCh <- struct{}{}
		a.dedupFlush()
		<-flushConcurrencyCh
	}
}

func (a *aggregator) runFlusher(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-a.stopCh:
			return
		case <-t.C:
		}

		// Globally limit the concurrency for metrics' flush
		// in order to limit memory usage when big number of aggregators
		// are flushed at the same time.
		flushConcurrencyCh <- struct{}{}
		a.flush()
		<-flushConcurrencyCh
	}
}

var flushConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs())

func (a *aggregator) dedupFlush() {
	ctx := &flushCtx{
		skipAggrSuffix: true,
	}
	a.dedupAggr.appendSeriesForFlush(ctx)
	a.push(ctx.tss, nil)
}

func (a *aggregator) flush() {
	ctx := &flushCtx{
		suffix: a.suffix,
	}
	for _, as := range a.aggrStates {
		ctx.reset()
		as.appendSeriesForFlush(ctx)

		tss := ctx.tss

		if a.outputRelabeling == nil {
			// Fast path - push the output metrics.
			a.pushFunc(tss)
			continue
		}

		// Slower path - apply output relabeling and then push the output metrics.
		auxLabels := promutils.GetLabels()
		dstLabels := auxLabels.Labels[:0]
		dst := tss[:0]
		for _, ts := range tss {
			dstLabelsLen := len(dstLabels)
			dstLabels = append(dstLabels, ts.Labels...)
			dstLabels = a.outputRelabeling.Apply(dstLabels, dstLabelsLen)
			if len(dstLabels) == dstLabelsLen {
				// The metric has been deleted by the relabeling
				continue
			}
			ts.Labels = dstLabels[dstLabelsLen:]
			dst = append(dst, ts)
		}
		a.pushFunc(dst)
		auxLabels.Labels = dstLabels
		promutils.PutLabels(auxLabels)
	}
}

// MustStop stops the aggregator.
//
// The aggregator stops pushing the aggregated metrics after this call.
func (a *aggregator) MustStop() {
	close(a.stopCh)
	a.wg.Wait()

	if !a.flushOnShutdown {
		return
	}

	// Flush the remaining data from the last interval if needed.
	flushConcurrencyCh <- struct{}{}
	if a.dedupAggr != nil {
		a.dedupFlush()
	}
	a.flush()
	<-flushConcurrencyCh
}

// Push pushes tss to a.
func (a *aggregator) Push(tss []prompbmarshal.TimeSeries, matchIdxs []byte) {
	if a.dedupAggr == nil {
		// Deduplication is disabled.
		a.push(tss, matchIdxs)
		return
	}

	// Deduplication is enabled.
	// push samples to dedupAggr, so later they will be pushed to the configured aggregators.
	pushSample := a.dedupAggr.pushSample
	inputKey := ""
	bb := bbPool.Get()
	labels := promutils.GetLabels()
	for idx, ts := range tss {
		if !a.match.Match(ts.Labels) {
			continue
		}
		matchIdxs[idx] = 1

		labels.Labels = append(labels.Labels[:0], ts.Labels...)
		labels.Labels = a.inputRelabeling.Apply(labels.Labels, 0)
		if len(labels.Labels) == 0 {
			// The metric has been deleted by the relabeling
			continue
		}
		labels.Sort()

		bb.B = marshalLabelsFast(bb.B[:0], labels.Labels)
		outputKey := bytesutil.InternBytes(bb.B)
		for _, sample := range ts.Samples {
			pushSample(inputKey, outputKey, sample.Value)
		}
	}
	promutils.PutLabels(labels)
	bbPool.Put(bb)
}

func (a *aggregator) push(tss []prompbmarshal.TimeSeries, matchIdxs []byte) {
	labels := promutils.GetLabels()
	tmpLabels := promutils.GetLabels()
	bb := bbPool.Get()
	applyFilters := matchIdxs != nil
	for idx, ts := range tss {
		if applyFilters {
			if !a.match.Match(ts.Labels) {
				continue
			}
			matchIdxs[idx] = 1
		}

		labels.Labels = append(labels.Labels[:0], ts.Labels...)
		if applyFilters {
			labels.Labels = a.inputRelabeling.Apply(labels.Labels, 0)
			if len(labels.Labels) == 0 {
				// The metric has been deleted by the relabeling
				continue
			}
			labels.Sort()
		}

		if a.aggregateOnlyByTime {
			bb.B = marshalLabelsFast(bb.B[:0], labels.Labels)
		} else {
			tmpLabels.Labels = removeUnneededLabels(tmpLabels.Labels[:0], labels.Labels, a.by, a.without)
			bb.B = marshalLabelsFast(bb.B[:0], tmpLabels.Labels)
		}
		outputKey := bytesutil.InternBytes(bb.B)
		inputKey := ""
		if !a.aggregateOnlyByTime {
			tmpLabels.Labels = extractUnneededLabels(tmpLabels.Labels[:0], labels.Labels, a.by, a.without)
			bb.B = marshalLabelsFast(bb.B[:0], tmpLabels.Labels)
			inputKey = bytesutil.InternBytes(bb.B)
		}

		for _, sample := range ts.Samples {
			a.pushSample(inputKey, outputKey, sample.Value)
		}
	}
	bbPool.Put(bb)
	promutils.PutLabels(tmpLabels)
	promutils.PutLabels(labels)
}

var bbPool bytesutil.ByteBufferPool

func (a *aggregator) pushSample(inputKey, outputKey string, value float64) {
	if math.IsNaN(value) {
		// Skip nan samples
		return
	}
	for _, as := range a.aggrStates {
		as.pushSample(inputKey, outputKey, value)
	}
}

func extractUnneededLabels(dst, labels []prompbmarshal.Label, by, without []string) []prompbmarshal.Label {
	if len(without) > 0 {
		for _, label := range labels {
			if hasInArray(label.Name, without) {
				dst = append(dst, label)
			}
		}
	} else {
		for _, label := range labels {
			if !hasInArray(label.Name, by) {
				dst = append(dst, label)
			}
		}
	}
	return dst
}

func removeUnneededLabels(dst, labels []prompbmarshal.Label, by, without []string) []prompbmarshal.Label {
	if len(without) > 0 {
		for _, label := range labels {
			if !hasInArray(label.Name, without) {
				dst = append(dst, label)
			}
		}
	} else {
		for _, label := range labels {
			if hasInArray(label.Name, by) {
				dst = append(dst, label)
			}
		}
	}
	return dst
}

func hasInArray(name string, a []string) bool {
	for _, s := range a {
		if name == s {
			return true
		}
	}
	return false
}

func marshalLabelsFast(dst []byte, labels []prompbmarshal.Label) []byte {
	dst = encoding.MarshalUint32(dst, uint32(len(labels)))
	for _, label := range labels {
		dst = encoding.MarshalUint32(dst, uint32(len(label.Name)))
		dst = append(dst, label.Name...)
		dst = encoding.MarshalUint32(dst, uint32(len(label.Value)))
		dst = append(dst, label.Value...)
	}
	return dst
}

func unmarshalLabelsFast(dst []prompbmarshal.Label, src []byte) ([]prompbmarshal.Label, error) {
	if len(src) < 4 {
		return dst, fmt.Errorf("cannot unmarshal labels count from %d bytes; needs at least 4 bytes", len(src))
	}
	n := encoding.UnmarshalUint32(src)
	src = src[4:]
	for i := uint32(0); i < n; i++ {
		// Unmarshal label name
		if len(src) < 4 {
			return dst, fmt.Errorf("cannot unmarshal label name length from %d bytes; needs at least 4 bytes", len(src))
		}
		labelNameLen := encoding.UnmarshalUint32(src)
		src = src[4:]
		if uint32(len(src)) < labelNameLen {
			return dst, fmt.Errorf("cannot unmarshal label name from %d bytes; needs at least %d bytes", len(src), labelNameLen)
		}
		labelName := bytesutil.InternBytes(src[:labelNameLen])
		src = src[labelNameLen:]

		// Unmarshal label value
		if len(src) < 4 {
			return dst, fmt.Errorf("cannot unmarshal label value length from %d bytes; needs at least 4 bytes", len(src))
		}
		labelValueLen := encoding.UnmarshalUint32(src)
		src = src[4:]
		if uint32(len(src)) < labelValueLen {
			return dst, fmt.Errorf("cannot unmarshal label value from %d bytes; needs at least %d bytes", len(src), labelValueLen)
		}
		labelValue := bytesutil.InternBytes(src[:labelValueLen])
		src = src[labelValueLen:]

		dst = append(dst, prompbmarshal.Label{
			Name:  labelName,
			Value: labelValue,
		})
	}
	if len(src) > 0 {
		return dst, fmt.Errorf("unexpected non-empty tail after unmarshaling labels; tail length is %d bytes", len(src))
	}
	return dst, nil
}

type flushCtx struct {
	skipAggrSuffix bool
	suffix         string

	tss     []prompbmarshal.TimeSeries
	labels  []prompbmarshal.Label
	samples []prompbmarshal.Sample
}

func (ctx *flushCtx) reset() {
	ctx.tss = prompbmarshal.ResetTimeSeries(ctx.tss)
	promrelabel.CleanLabels(ctx.labels)
	ctx.labels = ctx.labels[:0]
	ctx.samples = ctx.samples[:0]
}

func (ctx *flushCtx) appendSeries(labelsMarshaled, suffix string, timestamp int64, value float64) {
	var err error
	labelsLen := len(ctx.labels)
	samplesLen := len(ctx.samples)
	ctx.labels, err = unmarshalLabelsFast(ctx.labels, bytesutil.ToUnsafeBytes(labelsMarshaled))
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal labels from output key: %s", err)
	}
	if !ctx.skipAggrSuffix {
		ctx.labels = addMetricSuffix(ctx.labels, labelsLen, ctx.suffix, suffix)
	}
	ctx.samples = append(ctx.samples, prompbmarshal.Sample{
		Timestamp: timestamp,
		Value:     value,
	})
	ctx.tss = append(ctx.tss, prompbmarshal.TimeSeries{
		Labels:  ctx.labels[labelsLen:],
		Samples: ctx.samples[samplesLen:],
	})
}

func (ctx *flushCtx) appendSeriesWithExtraLabel(labelsMarshaled, suffix string, timestamp int64, value float64, extraName, extraValue string) {
	var err error
	labelsLen := len(ctx.labels)
	samplesLen := len(ctx.samples)
	ctx.labels, err = unmarshalLabelsFast(ctx.labels, bytesutil.ToUnsafeBytes(labelsMarshaled))
	if err != nil {
		logger.Panicf("BUG: cannot unmarshal labels from output key: %s", err)
	}
	ctx.labels = addMetricSuffix(ctx.labels, labelsLen, ctx.suffix, suffix)
	ctx.labels = append(ctx.labels, prompbmarshal.Label{
		Name:  extraName,
		Value: extraValue,
	})
	ctx.samples = append(ctx.samples, prompbmarshal.Sample{
		Timestamp: timestamp,
		Value:     value,
	})
	ctx.tss = append(ctx.tss, prompbmarshal.TimeSeries{
		Labels:  ctx.labels[labelsLen:],
		Samples: ctx.samples[samplesLen:],
	})
}

func addMetricSuffix(labels []prompbmarshal.Label, offset int, firstSuffix, lastSuffix string) []prompbmarshal.Label {
	src := labels[offset:]
	for i := range src {
		label := &src[i]
		if label.Name != "__name__" {
			continue
		}
		bb := bbPool.Get()
		bb.B = append(bb.B, label.Value...)
		bb.B = append(bb.B, firstSuffix...)
		bb.B = append(bb.B, lastSuffix...)
		label.Value = bytesutil.InternBytes(bb.B)
		bbPool.Put(bb)
		return labels
	}
	// The __name__ isn't found. Add it
	bb := bbPool.Get()
	bb.B = append(bb.B, firstSuffix...)
	bb.B = append(bb.B, lastSuffix...)
	labelValue := bytesutil.InternBytes(bb.B)
	labels = append(labels, prompbmarshal.Label{
		Name:  "__name__",
		Value: labelValue,
	})
	return labels
}

func addMissingUnderscoreName(labels []string) []string {
	result := []string{"__name__"}
	for _, s := range labels {
		if s == "__name__" {
			continue
		}
		result = append(result, s)
	}
	return result
}

func removeUnderscoreName(labels []string) []string {
	var result []string
	for _, s := range labels {
		if s == "__name__" {
			continue
		}
		result = append(result, s)
	}
	return result
}

func sortAndRemoveDuplicates(a []string) []string {
	if len(a) == 0 {
		return nil
	}
	a = append([]string{}, a...)
	sort.Strings(a)
	dst := a[:1]
	for _, v := range a[1:] {
		if v != dst[len(dst)-1] {
			dst = append(dst, v)
		}
	}
	return dst
}
