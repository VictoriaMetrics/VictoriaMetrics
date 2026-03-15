package streamaggr

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/histogram"
	"gopkg.in/yaml.v2"
)

// defines ingested samples lag quantile to determine a time to wait before flush.
// It's not configurable at the moment.
const flushQuantile = 0.95

var supportedOutputs = []string{
	"avg",
	"count_samples",
	"count_series",
	"histogram_bucket",
	"increase",
	"increase_prometheus",
	"last",
	"max",
	"min",
	"quantiles(phi1, ..., phiN)",
	"rate_avg",
	"rate_sum",
	"stddev",
	"stdvar",
	"sum_samples",
	"total",
	"total_prometheus",
	"unique_samples",
}

var (
	// lc contains information about all compressed labels for streaming aggregation
	lc promutil.LabelsCompressor

	_ = metrics.NewGauge(`vm_streamaggr_labels_compressor_size_bytes`, func() float64 {
		return float64(lc.SizeBytes())
	})

	_ = metrics.NewGauge(`vm_streamaggr_labels_compressor_items_count`, func() float64 {
		return float64(lc.ItemsCount())
	})
)

// LoadFromFile loads Aggregators from the given path and uses the given pushFunc for pushing the aggregated data.
//
// opts can contain additional options. If opts is nil, then default options are used.
//
// alias is used as url label in metrics exposed for the returned Aggregators.
//
// The returned Aggregators must be stopped with MustStop() when no longer needed.
func LoadFromFile(path string, pushFunc PushFunc, opts *Options, alias string) (*Aggregators, error) {
	data, err := fscore.ReadFileOrHTTP(path)
	if err != nil {
		return nil, fmt.Errorf("cannot load aggregators: %w", err)
	}
	data = envtemplate.ReplaceBytes(data)

	as, err := loadFromData(data, path, pushFunc, opts, alias)
	if err != nil {
		return nil, fmt.Errorf("cannot initialize aggregators from %q: %w; see https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#stream-aggregation-config", path, err)
	}

	return as, nil
}

// Options contains optional settings for the Aggregators.
type Options struct {
	// DedupInterval is deduplication interval for samples received for the same time series.
	//
	// The last sample per each series is left per each DedupInterval if DedupInterval > 0.
	//
	// By default deduplication is disabled.
	//
	// The deduplication can be set up individually per each aggregation via dedup_interval option.
	DedupInterval time.Duration

	// DropInputLabels is an optional list of labels to drop from samples before de-duplication and stream aggregation.
	DropInputLabels []string

	// NoAlignFlushToInterval disables alignment of flushes to the aggregation interval.
	//
	// By default flushes are aligned to aggregation interval.
	//
	// The alignment of flushes can be disabled individually per each aggregation via no_align_flush_to_interval option.
	NoAlignFlushToInterval bool

	// FlushOnShutdown enables flush of incomplete aggregation state on startup and shutdown.
	//
	// By default incomplete state is dropped.
	//
	// The flush of incomplete state can be enabled individually per each aggregation via flush_on_shutdown option.
	FlushOnShutdown bool

	// KeepMetricNames instructs to leave metric names as is for the output time series without adding any suffix.
	//
	// By default the following suffix is added to every output time series:
	//
	//     input_name:<interval>[_by_<by_labels>][_without_<without_labels>]_<output>
	//
	// This option can be overridden individually per each aggregation via keep_metric_names option.
	KeepMetricNames bool

	// IgnoreOldSamples instructs to ignore samples with timestamps older than the current aggregation interval.
	//
	// By default all the samples are taken into account.
	//
	// This option can be overridden individually per each aggregation via ignore_old_samples option.
	IgnoreOldSamples bool

	// IgnoreFirstIntervals sets the number of aggregation intervals to be ignored on start.
	//
	// By default, zero intervals are ignored.
	//
	// This option can be overridden individually per each aggregation via ignore_first_intervals option.
	IgnoreFirstIntervals int

	// KeepInput enables keeping all the input samples after the aggregation.
	//
	// By default, aggregates samples are dropped, while the remaining samples are written to the corresponding -remoteWrite.url.
	KeepInput bool

	// EnableWindows enables aggregation in windows
	EnableWindows bool
}

// Config is a configuration for a single stream aggregation.
type Config struct {
	// Name is an optional name of the Config.
	//
	// It is used as `name` label in the exposed metrics for the given Config.
	Name string `yaml:"name,omitempty"`

	// Match is a label selector for filtering time series for the given selector.
	//
	// If the match isn't set, then all the input time series are processed.
	Match *promrelabel.IfExpression `yaml:"match,omitempty"`

	// Interval is the interval between aggregations.
	Interval string `yaml:"interval"`

	// NoAlignFlushToInterval disables aligning of flushes to multiples of Interval.
	// By default flushes are aligned to Interval.
	//
	// See also FlushOnShutdown.
	NoAlignFlushToInterval *bool `yaml:"no_align_flush_to_interval,omitempty"`

	// FlushOnShutdown defines whether to flush incomplete aggregation state on startup and shutdown.
	// By default incomplete aggregation state is dropped, since it may confuse users.
	FlushOnShutdown *bool `yaml:"flush_on_shutdown,omitempty"`

	// DedupInterval is an optional interval for deduplication.
	DedupInterval string `yaml:"dedup_interval,omitempty"`

	// Staleness interval is interval after which the series state will be reset if no samples have been sent during it.
	// The parameter is only relevant for outputs: total, total_prometheus, increase, increase_prometheus and histogram_bucket.
	StalenessInterval string `yaml:"staleness_interval,omitempty"`

	// IgnoreFirstSampleInterval specifies the interval after which the agent begins sending samples.
	// By default, it is set to the staleness interval, and it helps reduce the initial sample load after an agent restart.
	// This parameter is relevant only for the following outputs: total, total_prometheus, increase, increase_prometheus, and histogram_bucket.
	IgnoreFirstSampleInterval string `yaml:"ignore_first_sample_interval,omitempty"`

	// Outputs is a list of output aggregate functions to produce.
	//
	// The following names are allowed:
	//
	// - avg - the average value across all the samples
	// - count_samples - counts the input samples
	// - count_series - counts the number of unique input series
	// - histogram_bucket - creates VictoriaMetrics histogram for input samples
	// - increase - calculates the increase over input series
	// - increase_prometheus - calculates the increase over input series, ignoring the first sample in new time series
	// - last - the last biggest sample value
	// - max - the maximum sample value
	// - min - the minimum sample value
	// - quantiles(phi1, ..., phiN) - quantiles' estimation for phi in the range [0..1]
	// - rate_avg - calculates average of rate for input counters
	// - rate_sum - calculates sum of rate for input counters
	// - stddev - standard deviation across all the samples
	// - stdvar - standard variance across all the samples
	// - sum_samples - sums the input sample values
	// - total - aggregates input counters
	// - total_prometheus - aggregates input counters, ignoring the first sample in new time series
	// - unique_samples - counts the number of unique sample values
	//
	// The output time series will have the following names by default:
	//
	//   input_name:<interval>[_by_<by_labels>][_without_<without_labels>]_<output>
	//
	// See also KeepMetricNames
	//
	Outputs []string `yaml:"outputs"`

	// KeepMetricNames instructs to leave metric names as is for the output time series without adding any suffix.
	KeepMetricNames *bool `yaml:"keep_metric_names,omitempty"`

	// IgnoreOldSamples instructs to ignore samples with old timestamps outside the current aggregation interval.
	IgnoreOldSamples *bool `yaml:"ignore_old_samples,omitempty"`

	// IgnoreFirstIntervals sets number of aggregation intervals to be ignored on start.
	IgnoreFirstIntervals *int `yaml:"ignore_first_intervals,omitempty"`

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

	// DropInputLabels is an optional list with labels, which must be dropped before further processing of input samples.
	//
	// Labels are dropped before de-duplication and aggregation.
	DropInputLabels *[]string `yaml:"drop_input_labels,omitempty"`

	// InputRelabelConfigs is an optional relabeling rules, which are applied on the input
	// before aggregation.
	InputRelabelConfigs []promrelabel.RelabelConfig `yaml:"input_relabel_configs,omitempty"`

	// OutputRelabelConfigs is an optional relabeling rules, which are applied
	// on the aggregated output before being sent to remote storage.
	OutputRelabelConfigs []promrelabel.RelabelConfig `yaml:"output_relabel_configs,omitempty"`

	// EnableWindows enables aggregation in windows
	EnableWindows *bool `yaml:"enable_windows,omitempty"`
}

// Aggregators aggregates metrics passed to Push and calls pushFunc for aggregated data.
type Aggregators struct {
	as []*aggregator

	// configData contains marshaled configs.
	// It is used in Equal() for comparing Aggregators.
	configData []byte

	// filePath is the path to config file used for creating the Aggregators.
	filePath string

	// ms contains metrics associated with the Aggregators.
	ms *metrics.Set
}

// FilePath returns path to file with the configuration used for creating the given Aggregators.
func (a *Aggregators) FilePath() string {
	return a.filePath
}

// LoadFromData loads aggregators from data.
//
// opts can contain additional options. If opts is nil, then default options are used.
func LoadFromData(data []byte, pushFunc PushFunc, opts *Options, alias string) (*Aggregators, error) {
	return loadFromData(data, "inmemory", pushFunc, opts, alias)
}

func loadFromData(data []byte, filePath string, pushFunc PushFunc, opts *Options, alias string) (*Aggregators, error) {
	var cfgs []*Config
	if err := yaml.UnmarshalStrict(data, &cfgs); err != nil {
		return nil, fmt.Errorf("cannot parse stream aggregation config: %w", err)
	}

	ms := metrics.NewSet()
	as := make([]*aggregator, len(cfgs))
	for i, cfg := range cfgs {
		a, err := newAggregator(cfg, filePath, pushFunc, ms, opts, alias, i+1)
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

	metrics.RegisterSet(ms)
	return &Aggregators{
		as:         as,
		configData: configData,
		filePath:   filePath,
		ms:         ms,
	}, nil
}

// IsEnabled returns true if Aggregators has at least one configured aggregator
func (a *Aggregators) IsEnabled() bool {
	if a == nil {
		return false
	}
	if len(a.as) == 0 {
		return false
	}
	return true
}

// MustStop stops a.
func (a *Aggregators) MustStop() {
	if a == nil {
		return
	}

	metrics.UnregisterSet(a.ms, true)
	a.ms = nil

	for _, aggr := range a.as {
		aggr.MustStop()
	}
	a.as = nil
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
// It reuses the matchIdxs if it has enough capacity to hold len(tss) items.
// Otherwise it allocates new matchIdxs.
func (a *Aggregators) Push(tss []prompb.TimeSeries, matchIdxs []uint32) []uint32 {
	matchIdxs = slicesutil.SetLength(matchIdxs, len(tss))
	for i := range matchIdxs {
		matchIdxs[i] = 0
	}
	if a == nil {
		return matchIdxs
	}

	// use all available CPU cores to copy time-series into aggregators
	// See this issue https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9878
	var wg sync.WaitGroup
	concurrencyChan := make(chan struct{}, cgroup.AvailableCPUs())

	for _, aggr := range a.as {
		concurrencyChan <- struct{}{}
		wg.Go(func() {
			aggr.Push(tss, matchIdxs)
			<-concurrencyChan
		})
	}

	wg.Wait()

	return matchIdxs
}

type currentState struct {

	// isGreen defines a state, where data sample with minDeadline < timestamp < maxDeadline will be aggregated
	// and which is expected to be flushed with maxDeadline timestamp
	isGreen bool

	// maxDeadline is a threshold for samples from different states, when enableWindows is set
	maxDeadline int64
}

func (cs *currentState) newState() *currentState {
	return &currentState{
		isGreen:     cs.isGreen,
		maxDeadline: cs.maxDeadline,
	}
}

// aggregator aggregates input series according to the config passed to NewAggregator
type aggregator struct {
	match *promrelabel.IfExpression

	dropInputLabels []string

	inputRelabeling   *promrelabel.ParsedConfigs
	outputRelabeling  *promrelabel.ParsedConfigs
	stalenessInterval time.Duration

	keepMetricNames  bool
	ignoreOldSamples bool
	enableWindows    bool

	by                  []string
	without             []string
	aggregateOnlyByTime bool

	// interval is the interval between flushes
	interval time.Duration

	// dedupInterval is optional deduplication interval for incoming samples
	dedupInterval time.Duration

	// da is set to non-nil if input samples must be de-duplicated
	da *dedupAggr

	// cs persists information of state, which is expected to be flushed next
	cs atomic.Pointer[currentState]

	// minDeadline is used for ignoring old samples when ignoreOldSamples or enableWindows is set
	minDeadline atomic.Int64

	// aggrOutputs contains aggregate states for the given outputs
	aggrOutputs *aggrOutputs

	// time to wait after interval end before flush
	flushAfter   *histogram.Fast
	muFlushAfter sync.Mutex

	// suffix contains a suffix, which should be added to aggregate metric names
	//
	// It contains the interval, labels in (by, without), plus output name.
	// For example, foo_bar metric name is transformed to foo_bar:1m_by_job
	// for `interval: 1m`, `by: [job]`
	suffix string

	wg     sync.WaitGroup
	stopCh chan struct{}

	flushDuration *metrics.Histogram
	samplesLag    *metrics.Histogram

	flushTimeouts     *metrics.Counter
	ignoredOldSamples *metrics.Counter
	ignoredNaNSamples *metrics.Counter
	matchedSamples    *metrics.Counter
}

// PushFunc is called by Aggregators when it needs to push its state to metrics storage
type PushFunc func(tss []prompb.TimeSeries)

type aggrPushFunc func([]pushSample, int64, bool)

// newAggregator creates new aggregator for the given cfg, which pushes the aggregate data to pushFunc.
//
// opts can contain additional options. If opts is nil, then default options are used.
//
// The returned aggregator must be stopped when no longer needed by calling MustStop().
func newAggregator(cfg *Config, path string, pushFunc PushFunc, ms *metrics.Set, opts *Options, alias string, aggrID int) (*aggregator, error) {
	// check cfg.Interval
	if cfg.Interval == "" {
		return nil, fmt.Errorf("missing `interval` option")
	}
	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `interval: %q`: %w", cfg.Interval, err)
	}
	if interval < time.Second {
		return nil, fmt.Errorf("aggregation interval cannot be smaller than 1s; got %s", interval)
	}

	if opts == nil {
		opts = &Options{}
	}

	// check cfg.DedupInterval
	dedupInterval := opts.DedupInterval
	if cfg.DedupInterval != "" {
		di, err := time.ParseDuration(cfg.DedupInterval)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `dedup_interval: %q`: %w", cfg.DedupInterval, err)
		}
		dedupInterval = di
	}
	if dedupInterval > interval {
		return nil, fmt.Errorf("dedup_interval=%s cannot exceed interval=%s", dedupInterval, interval)
	}
	if dedupInterval > 0 && interval%dedupInterval != 0 {
		return nil, fmt.Errorf("interval=%s must be a multiple of dedup_interval=%s", interval, dedupInterval)
	}

	// check cfg.StalenessInterval
	stalenessInterval := interval * 2
	if cfg.StalenessInterval != "" {
		stalenessInterval, err = time.ParseDuration(cfg.StalenessInterval)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `staleness_interval: %q`: %w", cfg.StalenessInterval, err)
		}
		if stalenessInterval < interval {
			return nil, fmt.Errorf("staleness_interval=%s cannot be smaller than interval=%s", cfg.StalenessInterval, cfg.Interval)
		}
	}

	// check cfg.IgnoreFirstSampleInterval
	// by default, it equals to the staleness interval to have backward compatibility, see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7116
	ignoreFirstSampleInterval := stalenessInterval
	if cfg.IgnoreFirstSampleInterval != "" {
		ignoreFirstSampleInterval, err = time.ParseDuration(cfg.IgnoreFirstSampleInterval)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `ignore_first_sample_interval: %q`: %w", cfg.IgnoreFirstSampleInterval, err)
		}
	}

	// Check cfg.DropInputLabels
	dropInputLabels := opts.DropInputLabels
	if v := cfg.DropInputLabels; v != nil {
		dropInputLabels = *v
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
		return nil, fmt.Errorf("`by: %s` and `without: %s` lists cannot be set simultaneously; see https://docs.victoriametrics.com/victoriametrics/stream-aggregation/", by, without)
	}
	aggregateOnlyByTime := (len(by) == 0 && len(without) == 0)
	if !aggregateOnlyByTime && len(without) == 0 {
		by = addMissingUnderscoreName(by)
	}

	// check cfg.KeepMetricNames
	keepMetricNames := opts.KeepMetricNames
	if v := cfg.KeepMetricNames; v != nil {
		keepMetricNames = *v
	}
	if keepMetricNames {
		if opts.KeepInput {
			return nil, fmt.Errorf("`-streamAggr.keepInput` and `keep_metric_names` options can't be enabled in the same time," +
				"as it may result in time series collision")
		}
		if len(cfg.Outputs) != 1 {
			return nil, fmt.Errorf("`outputs` list must contain only a single entry if `keep_metric_names` is set; got %q; "+
				"see https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#output-metric-names", cfg.Outputs)
		}
		if cfg.Outputs[0] == "histogram_bucket" || strings.HasPrefix(cfg.Outputs[0], "quantiles(") && strings.Contains(cfg.Outputs[0], ",") {
			return nil, fmt.Errorf("`keep_metric_names` cannot be applied to `outputs: %q`, since they can generate multiple time series; "+
				"see https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#output-metric-names", cfg.Outputs)
		}
	}

	// check cfg.IgnoreOldSamples
	ignoreOldSamples := opts.IgnoreOldSamples
	if v := cfg.IgnoreOldSamples; v != nil {
		ignoreOldSamples = *v
	}

	// check cfg.IgnoreFirstIntervals
	ignoreFirstIntervals := opts.IgnoreFirstIntervals
	if v := cfg.IgnoreFirstIntervals; v != nil {
		ignoreFirstIntervals = *v
	}

	// check cfg.
	enableWindows := opts.EnableWindows
	if v := cfg.EnableWindows; v != nil {
		enableWindows = *v
	}

	// Initialize common metric labels
	name := cfg.Name
	if name == "" {
		name = "none"
	}

	// initialize aggrOutputs
	if len(cfg.Outputs) == 0 {
		return nil, fmt.Errorf("`outputs` list must contain at least a single entry from the list %s; "+
			"see https://docs.victoriametrics.com/victoriametrics/stream-aggregation/", supportedOutputs)
	}
	useInputKey := dedupInterval <= 0
	useSharedState := enableWindows && useInputKey
	aggrOutputs := &aggrOutputs{
		configs:        make([]aggrConfig, len(cfg.Outputs)),
		useSharedState: useSharedState,
		useInputKey:    useInputKey,
	}
	outputsSeen := make(map[string]struct{}, len(cfg.Outputs))
	for i, output := range cfg.Outputs {
		ac, err := newOutputConfig(output, outputsSeen, useSharedState, ignoreFirstSampleInterval)
		if err != nil {
			return nil, err
		}
		aggrOutputs.configs[i] = ac
	}
	outputsLabels := make([]string, 0, len(outputsSeen))
	for o := range outputsSeen {
		outputsLabels = append(outputsLabels, o)
	}
	metricLabels := fmt.Sprintf(`outputs=%q,name=%q,path=%q,url=%q,position="%d"`, strings.Join(outputsLabels, ","), name, path, alias, aggrID)
	aggrOutputs.outputSamples = ms.NewCounter(fmt.Sprintf(`vm_streamaggr_output_samples_total{%s}`, metricLabels))

	// initialize suffix to add to metric names after aggregation
	suffix := ":" + cfg.Interval
	if labels := removeUnderscoreName(by); len(labels) > 0 {
		suffix += fmt.Sprintf("_by_%s", strings.Join(labels, "_"))
	}
	if labels := removeUnderscoreName(without); len(labels) > 0 {
		suffix += fmt.Sprintf("_without_%s", strings.Join(labels, "_"))
	}
	suffix += "_"

	// initialize the aggregator
	a := &aggregator{
		match: cfg.Match,

		dropInputLabels:  dropInputLabels,
		inputRelabeling:  inputRelabeling,
		outputRelabeling: outputRelabeling,

		keepMetricNames:   keepMetricNames,
		ignoreOldSamples:  ignoreOldSamples,
		enableWindows:     enableWindows,
		stalenessInterval: stalenessInterval,

		by:                  by,
		without:             without,
		aggregateOnlyByTime: aggregateOnlyByTime,

		interval:      interval,
		dedupInterval: dedupInterval,

		aggrOutputs: aggrOutputs,

		suffix: suffix,

		stopCh: make(chan struct{}),

		flushDuration: ms.NewHistogram(fmt.Sprintf(`vm_streamaggr_flush_duration_seconds{%s}`, metricLabels)),
		samplesLag:    ms.NewHistogram(fmt.Sprintf(`vm_streamaggr_samples_lag_seconds{%s}`, metricLabels)),

		matchedSamples:    ms.NewCounter(fmt.Sprintf(`vm_streamaggr_matched_samples_total{%s}`, metricLabels)),
		flushTimeouts:     ms.NewCounter(fmt.Sprintf(`vm_streamaggr_flush_timeouts_total{%s}`, metricLabels)),
		ignoredNaNSamples: ms.NewCounter(fmt.Sprintf(`vm_streamaggr_ignored_samples_total{reason="nan",%s}`, metricLabels)),
		ignoredOldSamples: ms.NewCounter(fmt.Sprintf(`vm_streamaggr_ignored_samples_total{reason="too_old",%s}`, metricLabels)),
	}

	if dedupInterval > 0 {
		a.da = newDedupAggr()
		a.da.flushTimeouts = ms.NewCounter(fmt.Sprintf(`vm_streamaggr_dedup_flush_timeouts_total{%s}`, metricLabels))
		a.da.flushDuration = ms.NewHistogram(fmt.Sprintf(`vm_streamaggr_dedup_flush_duration_seconds{%s}`, metricLabels))

		_ = ms.NewGauge(fmt.Sprintf(`vm_streamaggr_dedup_state_size_bytes{%s}`, metricLabels), func() float64 {
			n := a.da.sizeBytes()
			return float64(n)
		})
		_ = ms.NewGauge(fmt.Sprintf(`vm_streamaggr_dedup_state_items_count{%s}`, metricLabels), func() float64 {
			n := a.da.itemsCount()
			return float64(n)
		})
	}

	alignFlushToInterval := !opts.NoAlignFlushToInterval
	if v := cfg.NoAlignFlushToInterval; v != nil {
		alignFlushToInterval = !*v
	}

	skipFlushOnShutdown := !opts.FlushOnShutdown
	if v := cfg.FlushOnShutdown; v != nil {
		skipFlushOnShutdown = !*v
	}

	startTime := time.Now()
	minTime := startTime
	if alignFlushToInterval {
		minTime = minTime.Truncate(a.interval)
		if !startTime.Equal(minTime) {
			minTime = minTime.Add(interval)
		}
	}
	if enableWindows {
		a.flushAfter = histogram.GetFast()
	}
	a.minDeadline.Store(minTime.UnixMilli())
	cs := &currentState{}
	if a.dedupInterval > 0 {
		cs.maxDeadline = minTime.Add(a.dedupInterval).UnixMilli()
	} else {
		cs.maxDeadline = minTime.Add(a.interval).UnixMilli()
	}
	a.cs.Store(cs)

	a.wg.Go(func() {
		a.runFlusher(pushFunc, alignFlushToInterval, skipFlushOnShutdown, ignoreFirstIntervals)
	})

	return a, nil
}

func newOutputConfig(output string, outputsSeen map[string]struct{}, useSharedState bool, ignoreFirstSampleInterval time.Duration) (aggrConfig, error) {
	// check for duplicated output
	if _, ok := outputsSeen[output]; ok {
		return nil, fmt.Errorf("`outputs` list contains duplicate aggregation function: %s", output)
	}
	outputsSeen[output] = struct{}{}

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
		for i, arg := range args {
			arg = strings.TrimSpace(arg)
			phi, err := strconv.ParseFloat(arg, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse phi=%q for quantiles(%s): %w", arg, argsStr, err)
			}
			if phi < 0 || phi > 1 {
				return nil, fmt.Errorf("phi inside quantiles(%s) must be in the range [0..1]; got %v", argsStr, phi)
			}
			phis[i] = phi
		}
		if _, ok := outputsSeen["quantiles"]; ok {
			return nil, fmt.Errorf("`outputs` list contains duplicated `quantiles()` function, please combine multiple phi* like `quantiles(0.5, 0.9)`")
		}
		outputsSeen["quantiles"] = struct{}{}
		return newQuantilesAggrConfig(phis), nil
	}
	ignoreFirstSampleIntervalSecs := uint64(ignoreFirstSampleInterval.Seconds())

	switch output {
	case "avg":
		return newAvgAggrConfig(), nil
	case "count_samples":
		return newCountSamplesAggrConfig(), nil
	case "count_series":
		return newCountSeriesAggrConfig(), nil
	case "histogram_bucket":
		return newHistogramBucketAggrConfig(useSharedState), nil
	case "increase":
		return newTotalAggrConfig(ignoreFirstSampleIntervalSecs, true, true), nil
	case "increase_prometheus":
		return newTotalAggrConfig(ignoreFirstSampleIntervalSecs, true, false), nil
	case "last":
		return newLastAggrConfig(), nil
	case "max":
		return newMaxAggrConfig(), nil
	case "min":
		return newMinAggrConfig(), nil
	case "rate_avg":
		return newRateAggrConfig(true), nil
	case "rate_sum":
		return newRateAggrConfig(false), nil
	case "stddev":
		return newStddevAggrConfig(), nil
	case "stdvar":
		return newStdvarAggrConfig(), nil
	case "sum_samples":
		return newSumSamplesAggrConfig(), nil
	case "total":
		return newTotalAggrConfig(ignoreFirstSampleIntervalSecs, false, true), nil
	case "total_prometheus":
		return newTotalAggrConfig(ignoreFirstSampleIntervalSecs, false, false), nil
	case "unique_samples":
		return newUniqueSamplesAggrConfig(), nil
	default:
		return nil, fmt.Errorf("unsupported output=%q; supported values: %s; see https://docs.victoriametrics.com/victoriametrics/stream-aggregation/", output, supportedOutputs)
	}
}

func (a *aggregator) runFlusher(pushFunc PushFunc, alignFlushToInterval, skipFlushOnShutdown bool, ignoreFirstIntervals int) {
	minTime := time.UnixMilli(a.minDeadline.Load())
	flushTime := minTime.Add(a.interval)
	interval := a.interval
	if a.dedupInterval > 0 {
		interval = a.dedupInterval
	}
	alignedSleep := func() {
		dSleep := time.Until(minTime)
		if dSleep <= 0 {
			return
		}
		timer := timerpool.Get(dSleep)
		defer timerpool.Put(timer)
		select {
		case <-a.stopCh:
		case <-timer.C:
		}
	}

	tickerWait := func(t *time.Ticker) bool {
		select {
		case <-a.stopCh:
			return false
		case <-t.C:
			return true
		}
	}

	alignedSleep()

	t := time.NewTicker(interval)
	defer t.Stop()

	var fa *histogram.Fast
	for tickerWait(t) {
		pf := pushFunc
		if a.enableWindows {
			// Calculate delay and wait
			a.muFlushAfter.Lock()
			fa, a.flushAfter = a.flushAfter, histogram.GetFast()
			a.muFlushAfter.Unlock()
			delay := time.Duration(fa.Quantile(flushQuantile)) * time.Millisecond
			histogram.PutFast(fa)
			time.Sleep(delay)
		}

		cs := a.cs.Load().newState()
		deadlineTime := time.UnixMilli(cs.maxDeadline)
		a.dedupFlush(deadlineTime, cs)

		if !flushTime.After(deadlineTime) {
			// It is time to flush the aggregated state
			if ignoreFirstIntervals > 0 {
				a.flush(nil, flushTime, cs, false)
				ignoreFirstIntervals--
			} else {
				a.flush(pf, flushTime, cs, false)
			}
			for time.Now().After(flushTime) {
				flushTime = flushTime.Add(a.interval)
			}
			if a.dedupInterval <= 0 {
				cs.maxDeadline = flushTime.UnixMilli()
			}
		}
		if a.enableWindows {
			cs.isGreen = !cs.isGreen
		}
		a.cs.Store(cs)
		if alignFlushToInterval {
			select {
			case <-t.C:
			default:
			}
		}
	}

	cs := a.cs.Load()
	var dedupTime time.Time
	if alignFlushToInterval {
		if a.dedupInterval > 0 {
			dedupTime = time.UnixMilli(cs.maxDeadline)
		}
	} else {
		flushTime = time.Now()
		if a.dedupInterval > 0 {
			dedupTime = flushTime
		}
	}

	a.dedupFlush(dedupTime, cs)
	pf := pushFunc
	if skipFlushOnShutdown || ignoreFirstIntervals > 0 {
		pf = nil
	}
	a.flush(pf, flushTime, cs, true)
}

func (a *aggregator) dedupFlush(dedupTime time.Time, cs *currentState) {
	if a.dedupInterval <= 0 {
		// The de-duplication is disabled.
		return
	}

	a.minDeadline.Store(cs.maxDeadline)
	startTime := time.Now()
	deleteDeadline := dedupTime.Add(a.stalenessInterval)

	a.da.flush(a.aggrOutputs.pushSamples, deleteDeadline.UnixMilli(), cs.isGreen)

	d := time.Since(startTime)
	a.da.flushDuration.Update(d.Seconds())
	if d > a.dedupInterval {
		a.da.flushTimeouts.Inc()
		logger.Warnf("deduplication couldn't be finished in the configured dedup_interval=%s; it took %.03fs; "+
			"possible solutions: increase dedup_interval; use match filter matching smaller number of series; "+
			"reduce samples' ingestion rate to stream aggregation", a.dedupInterval, d.Seconds())
	}
	for time.Now().After(dedupTime) {
		dedupTime = dedupTime.Add(a.dedupInterval)
	}
	cs.maxDeadline = dedupTime.UnixMilli()
}

// flush flushes aggregator state to pushFunc.
//
// If pushFunc is nil, then the aggregator state is just reset.
func (a *aggregator) flush(pushFunc PushFunc, flushTime time.Time, cs *currentState, isLast bool) {
	startTime := time.Now()
	ao := a.aggrOutputs

	ctx := getFlushCtx(a, ao, pushFunc, flushTime.UnixMilli(), isLast)
	if a.dedupInterval <= 0 {
		a.minDeadline.Store(cs.maxDeadline)
		ctx.isGreen = cs.isGreen
	}
	ao.flushState(ctx)
	ctx.flushSeries()
	putFlushCtx(ctx)

	d := time.Since(startTime)
	a.flushDuration.Update(d.Seconds())
	if d > a.interval {
		a.flushTimeouts.Inc()
		logger.Warnf("stream aggregation couldn't be finished in the configured interval=%s; it took %.03fs; "+
			"possible solutions: increase interval; use match filter matching smaller number of series; "+
			"reduce samples' ingestion rate to stream aggregation", a.interval, d.Seconds())
	}
}

var flushConcurrencyCh = make(chan struct{}, cgroup.AvailableCPUs())

// MustStop stops the aggregator.
//
// The aggregator stops pushing the aggregated metrics after this call.
func (a *aggregator) MustStop() {
	close(a.stopCh)
	a.wg.Wait()
}

// Push pushes tss to a.
func (a *aggregator) Push(tss []prompb.TimeSeries, matchIdxs []uint32) {
	ctx := getPushCtx()
	defer putPushCtx(ctx)

	buf := ctx.buf
	labels := &ctx.labels
	inputLabels := &ctx.inputLabels
	outputLabels := &ctx.outputLabels
	now := time.Now()
	nowMsec := now.UnixMilli()
	deleteDeadline := now.Add(a.stalenessInterval)
	deleteDeadlineMsec := deleteDeadline.UnixMilli()

	minDeadline := a.minDeadline.Load()
	dropLabels := a.dropInputLabels
	ignoreOldSamples := a.ignoreOldSamples
	enableWindows := a.enableWindows
	cs := a.cs.Load()

	var maxLagMsec int64
	for idx, ts := range tss {
		if !a.match.Match(ts.Labels) {
			continue
		}
		atomic.StoreUint32(&matchIdxs[idx], 1)

		if len(dropLabels) > 0 {
			labels.Labels = dropSeriesLabels(labels.Labels[:0], ts.Labels, dropLabels)
		} else {
			labels.Labels = append(labels.Labels[:0], ts.Labels...)
		}
		labels.Labels = a.inputRelabeling.Apply(labels.Labels, 0)
		if len(labels.Labels) == 0 {
			// The metric has been deleted by the relabeling
			continue
		}
		labels.Sort()

		inputLabels.Reset()
		outputLabels.Reset()
		if !a.aggregateOnlyByTime {
			inputLabels.Labels, outputLabels.Labels = getInputOutputLabels(inputLabels.Labels, outputLabels.Labels, labels.Labels, a.by, a.without)
		} else {
			outputLabels.Labels = append(outputLabels.Labels, labels.Labels...)
		}

		bufLen := len(buf)
		buf = compressLabels(buf, inputLabels.Labels, outputLabels.Labels)
		// key remains valid only by the end of this function and can't be reused after
		// do not intern key because number of unique keys could be too high
		key := bytesutil.ToUnsafeString(buf[bufLen:])
		for _, s := range ts.Samples {
			if math.IsNaN(s.Value) {
				// Skip NaN values
				a.ignoredNaNSamples.Inc()
				continue
			}
			if (ignoreOldSamples || enableWindows) && s.Timestamp < minDeadline {
				// Skip old samples outside the current aggregation interval
				a.ignoredOldSamples.Inc()
				continue
			}
			lagMsec := nowMsec - s.Timestamp
			if lagMsec > maxLagMsec {
				maxLagMsec = lagMsec
			}
			if enableWindows && s.Timestamp <= cs.maxDeadline == cs.isGreen {
				ctx.green = append(ctx.green, pushSample{
					key:       key,
					value:     s.Value,
					timestamp: s.Timestamp,
				})
			} else {
				ctx.blue = append(ctx.blue, pushSample{
					key:       key,
					value:     s.Value,
					timestamp: s.Timestamp,
				})
			}
		}
	}
	if enableWindows && maxLagMsec > 0 {
		a.muFlushAfter.Lock()
		a.flushAfter.Update(float64(maxLagMsec))
		a.muFlushAfter.Unlock()
	}
	a.samplesLag.Update(float64(maxLagMsec) / 1_000)

	ctx.buf = buf

	pushSamples := a.aggrOutputs.pushSamples
	if a.da != nil {
		pushSamples = a.da.pushSamples
	}

	if len(ctx.blue) > 0 {
		a.matchedSamples.Add(len(ctx.blue))
		pushSamples(ctx.blue, deleteDeadlineMsec, false)
	}

	if len(ctx.green) > 0 {
		a.matchedSamples.Add(len(ctx.green))
		pushSamples(ctx.green, deleteDeadlineMsec, true)
	}
}

func compressLabels(dst []byte, inputLabels, outputLabels []prompb.Label) []byte {
	bb := bbPool.Get()
	bb.B = lc.Compress(bb.B, outputLabels)
	dst = encoding.MarshalVarUint64(dst, uint64(len(bb.B)))
	dst = append(dst, bb.B...)
	bbPool.Put(bb)
	dst = lc.Compress(dst, inputLabels)
	return dst
}

func decompressLabels(dst []prompb.Label, key string) []prompb.Label {
	return lc.Decompress(dst, bytesutil.ToUnsafeBytes(key))
}

type pushCtx struct {
	green        []pushSample
	blue         []pushSample
	labels       promutil.Labels
	inputLabels  promutil.Labels
	outputLabels promutil.Labels
	buf          []byte
}

func (ctx *pushCtx) reset() {
	ctx.blue = ctx.blue[:0]
	ctx.green = ctx.green[:0]
	ctx.labels.Reset()
	ctx.inputLabels.Reset()
	ctx.outputLabels.Reset()
	ctx.buf = ctx.buf[:0]
}

type pushSample struct {
	// key identifies a sample that belongs to unique series
	// key value can't be reused
	key       string
	value     float64
	timestamp int64
}

func getPushCtx() *pushCtx {
	v := pushCtxPool.Get()
	if v == nil {
		return &pushCtx{}
	}
	return v.(*pushCtx)
}

func putPushCtx(ctx *pushCtx) {
	ctx.reset()
	pushCtxPool.Put(ctx)
}

var pushCtxPool sync.Pool

func getInputOutputLabels(dstInput, dstOutput, labels []prompb.Label, by, without []string) ([]prompb.Label, []prompb.Label) {
	if len(without) > 0 {
		for _, label := range labels {
			if slices.Contains(without, label.Name) {
				dstInput = append(dstInput, label)
			} else {
				dstOutput = append(dstOutput, label)
			}
		}
	} else {
		for _, label := range labels {
			if !slices.Contains(by, label.Name) {
				dstInput = append(dstInput, label)
			} else {
				dstOutput = append(dstOutput, label)
			}
		}
	}
	return dstInput, dstOutput
}

func getFlushCtx(a *aggregator, ao *aggrOutputs, pushFunc PushFunc, flushTimestamp int64, isLast bool) *flushCtx {
	v := flushCtxPool.Get()
	if v == nil {
		v = &flushCtx{}
	}
	ctx := v.(*flushCtx)
	ctx.a = a
	ctx.ao = ao
	ctx.pushFunc = pushFunc
	ctx.flushTimestamp = flushTimestamp
	ctx.isLast = isLast
	return ctx
}

func putFlushCtx(ctx *flushCtx) {
	ctx.reset()
	flushCtxPool.Put(ctx)
}

var flushCtxPool sync.Pool

type flushCtx struct {
	a              *aggregator
	ao             *aggrOutputs
	pushFunc       PushFunc
	flushTimestamp int64
	isGreen        bool
	isLast         bool

	tss     []prompb.TimeSeries
	labels  []prompb.Label
	samples []prompb.Sample
}

func (ctx *flushCtx) reset() {
	ctx.a = nil
	ctx.ao = nil
	ctx.pushFunc = nil
	ctx.isGreen = false
	ctx.isLast = false
	ctx.flushTimestamp = 0
	ctx.resetSeries()
}

func (ctx *flushCtx) resetSeries() {
	clear(ctx.tss)
	ctx.tss = ctx.tss[:0]

	clear(ctx.labels)
	ctx.labels = ctx.labels[:0]

	ctx.samples = ctx.samples[:0]
}

func (ctx *flushCtx) flushSeries() {
	defer ctx.resetSeries()

	tss := ctx.tss
	if len(tss) == 0 {
		// nothing to flush
		return
	}

	outputRelabeling := ctx.a.outputRelabeling
	if outputRelabeling == nil {
		// Fast path - push the output metrics.
		if ctx.pushFunc != nil {
			ctx.pushFunc(tss)
			ctx.ao.outputSamples.Add(len(tss))
		}
		return
	}

	// Slow path - apply output relabeling and then push the output metrics.
	auxLabels := promutil.GetLabels()
	dstLabels := auxLabels.Labels[:0]
	dst := tss[:0]
	for _, ts := range tss {
		dstLabelsLen := len(dstLabels)
		dstLabels = append(dstLabels, ts.Labels...)
		dstLabels = outputRelabeling.Apply(dstLabels, dstLabelsLen)
		if len(dstLabels) == dstLabelsLen {
			// The metric has been deleted by the relabeling
			continue
		}
		ts.Labels = dstLabels[dstLabelsLen:]
		dst = append(dst, ts)
	}
	if ctx.pushFunc != nil {
		ctx.pushFunc(dst)
		ctx.ao.outputSamples.Add(len(dst))
	}
	auxLabels.Labels = dstLabels
	promutil.PutLabels(auxLabels)
}

func (ctx *flushCtx) appendSeries(key, suffix string, value float64) {
	labelsLen := len(ctx.labels)
	samplesLen := len(ctx.samples)
	ctx.labels = decompressLabels(ctx.labels, key)
	if !ctx.a.keepMetricNames {
		ctx.labels = addMetricSuffix(ctx.labels, labelsLen, ctx.a.suffix, suffix)
	}
	ctx.samples = append(ctx.samples, prompb.Sample{
		Timestamp: ctx.flushTimestamp,
		Value:     value,
	})
	ctx.tss = append(ctx.tss, prompb.TimeSeries{
		Labels:  ctx.labels[labelsLen:],
		Samples: ctx.samples[samplesLen:],
	})

	// Limit the maximum length of ctx.tss in order to limit memory usage.
	if len(ctx.tss) >= 10_000 {
		ctx.flushSeries()
	}
}

func (ctx *flushCtx) appendSeriesWithExtraLabel(key, suffix string, value float64, extraName, extraValue string) {
	labelsLen := len(ctx.labels)
	samplesLen := len(ctx.samples)
	ctx.labels = decompressLabels(ctx.labels, key)
	if !ctx.a.keepMetricNames {
		ctx.labels = addMetricSuffix(ctx.labels, labelsLen, ctx.a.suffix, suffix)
	}
	ctx.labels = append(ctx.labels, prompb.Label{
		Name:  extraName,
		Value: extraValue,
	})
	ctx.samples = append(ctx.samples, prompb.Sample{
		Timestamp: ctx.flushTimestamp,
		Value:     value,
	})
	ctx.tss = append(ctx.tss, prompb.TimeSeries{
		Labels:  ctx.labels[labelsLen:],
		Samples: ctx.samples[samplesLen:],
	})

	// Limit the maximum length of ctx.tss in order to limit memory usage.
	if len(ctx.tss) >= 10_000 {
		ctx.flushSeries()
	}
}

func addMetricSuffix(labels []prompb.Label, offset int, firstSuffix, lastSuffix string) []prompb.Label {
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
	labels = append(labels, prompb.Label{
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

var bbPool bytesutil.ByteBufferPool
