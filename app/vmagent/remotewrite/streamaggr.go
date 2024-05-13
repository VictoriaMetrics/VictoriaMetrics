package remotewrite

import (
	"flag"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
)

var (
	// Global config
	streamAggrGlobalConfig = flag.String("remoteWrite.streamAggr.global.config", "", "Optional path to file with stream aggregation global config. "+
		"See https://docs.victoriametrics.com/stream-aggregation/ . "+
		"See also -remoteWrite.streamAggr.global.keepInput, -remoteWrite.streamAggr.global.dropInput and -remoteWrite.streamAggr.global.dedupInterval")
	streamAggrGlobalKeepInput = flag.Bool("remoteWrite.streamAggr.global.keepInput", false, "Whether to keep all the input samples after the global aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.dropInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrGlobalDropInput = flag.Bool("remoteWrite.streamAggr.global.dropInput", false, "Whether to drop all the input samples after the global aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.global.keepInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrGlobalDedupInterval = flagutil.NewDuration("remoteWrite.streamAggr.global.dedupInterval", "0s", "Input samples are de-duplicated with this interval on global "+
		"aggregator before optional aggregation with -remoteWrite.streamAggr.config . "+
		"See also -dedup.minScrapeInterval and https://docs.victoriametrics.com/stream-aggregation/#deduplication")
	streamAggrGlobalIgnoreOldSamples = flag.Bool("remoteWrite.streamAggr.global.ignoreOldSamples", false, "Whether to ignore input samples with old timestamps outside the "+
		"current aggregation interval for global aggregator for the corresponding -remoteWrite.streamAggr.config . "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignoring-old-samples")
	streamAggrGlobalIgnoreFirstIntervals = flag.Int("remoteWrite.streamAggr.global.ignoreFirstIntervals", 0, "Number of aggregation intervals to skip after the start for global "+
		"aggregator. Increase this value if you observe incorrect aggregation results after vmagent restarts. It could be caused by receiving unordered delayed data from "+
		"clients pushing data into the vmagent. See https://docs.victoriametrics.com/stream-aggregation/#ignore-aggregation-intervals-on-start")
	streamAggrGlobalDropInputLabels = flagutil.NewArrayString("streamAggr.global.dropInputLabels", "An optional list of labels to drop from samples for global aggregator "+
		"before stream de-duplication and aggregation . See https://docs.victoriametrics.com/stream-aggregation/#dropping-unneeded-labels")

	// Per URL config
	streamAggrConfig = flagutil.NewArrayString("remoteWrite.streamAggr.config", "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/stream-aggregation/ . "+
		"See also -remoteWrite.streamAggr.keepInput, -remoteWrite.streamAggr.dropInput and -remoteWrite.streamAggr.dedupInterval")
	streamAggrDropInput = flagutil.NewArrayBool("remoteWrite.streamAggr.dropInput", "Whether to drop all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.keepInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrKeepInput = flagutil.NewArrayBool("remoteWrite.streamAggr.keepInput", "Whether to keep all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.dropInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrDedupInterval = flagutil.NewArrayDuration("remoteWrite.streamAggr.dedupInterval", 0, "Input samples are de-duplicated with this interval before optional aggregation "+
		"with -remoteWrite.streamAggr.config . See also -dedup.minScrapeInterval and https://docs.victoriametrics.com/stream-aggregation/#deduplication")
	streamAggrIgnoreOldSamples = flagutil.NewArrayBool("remoteWrite.streamAggr.ignoreOldSamples", "Whether to ignore input samples with old timestamps outside the current "+
		"aggregation interval for the corresponding -remoteWrite.streamAggr.config . "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignoring-old-samples")
	streamAggrIgnoreFirstIntervals = flag.Int("remoteWrite.streamAggr.ignoreFirstIntervals", 0, "Number of aggregation intervals to skip after the start. Increase this value if "+
		"you observe incorrect aggregation results after vmagent restarts. It could be caused by receiving unordered delayed data from clients pushing data into the vmagent. "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignore-aggregation-intervals-on-start")
	streamAggrDropInputLabels = flagutil.NewArrayString("streamAggr.dropInputLabels", "An optional list of labels to drop from samples "+
		"before stream de-duplication and aggregation . See https://docs.victoriametrics.com/stream-aggregation/#dropping-unneeded-labels")
)

// CheckStreamAggrConfigs checks -remoteWrite.streamAggr.config and -remoteWrite.streamAggr.globalConfig.
func CheckStreamAggrConfigs() error {
	pushNoop := func(_ []prompbmarshal.TimeSeries) {}
	sac := &streamAggrConfigs{}
	if err := sac.loadStreamAggrGlobal(pushNoop); err != nil {
		return fmt.Errorf("could not load global stream aggregation config: %w", err)
	}
	if len(*streamAggrConfig) > len(*remoteWriteURLs) {
		return fmt.Errorf("too many -remoteWrite.streamAggr.config args: %d; it mustn't exceed the number of -remoteWrite.url args: %d",
			len(*streamAggrConfig), len(*remoteWriteURLs))
	}
	for i := range *streamAggrConfig {
		if err := sac.loadStreamAggrPerCtx(i, pushNoop); err != nil {
			return err
		}
	}
	return nil
}

func (sac *streamAggrConfigs) reloadStreamAggrConfigs() error {
	if err := sac.global.Reload(); err != nil {
		return fmt.Errorf("failed to reload global config: %w", err)
	}
	for _, perCtx := range sac.perCtx {
		if err := perCtx.Reload(); err != nil {
			return fmt.Errorf("failed to reload config at location %q: %w", perCtx.ConfigPath(), err)
		}
	}
	return nil
}

func (sac *streamAggrConfigs) loadStreamAggrGlobal(pushFunc streamaggr.PushFunc) error {
	sac.perCtx = make([]*streamaggr.Aggregators, len(*remoteWriteURLs))
	if *streamAggrGlobalConfig != "" {
		path := *streamAggrGlobalConfig
		opts := &streamaggr.Options{
			DedupInterval:        streamAggrGlobalDedupInterval.Duration(),
			DropInputLabels:      *streamAggrGlobalDropInputLabels,
			IgnoreOldSamples:     *streamAggrGlobalIgnoreOldSamples,
			IgnoreFirstIntervals: *streamAggrGlobalIgnoreFirstIntervals,
		}
		sac.global = streamaggr.NewAggregators(path, pushFunc, opts)
		return sac.global.Load()
	}
	return nil
}

func (sac *streamAggrConfigs) loadStreamAggrPerCtx(idx int, pushFunc streamaggr.PushFunc) error {
	if len(*streamAggrConfig) == 0 {
		return nil
	}
	paths := *streamAggrConfig
	path := paths[idx]
	if len(path) == 0 {
		// Skip empty stream aggregation config.
		return nil
	}
	opts := &streamaggr.Options{
		DedupInterval:        streamAggrDedupInterval.GetOptionalArg(idx),
		DropInputLabels:      *streamAggrDropInputLabels,
		IgnoreOldSamples:     streamAggrIgnoreOldSamples.GetOptionalArg(idx),
		IgnoreFirstIntervals: *streamAggrIgnoreFirstIntervals,
	}
	sac.perCtx[idx] = streamaggr.NewAggregators(path, pushFunc, opts)
	return sac.perCtx[idx].Load()
}

type streamAggrConfigs struct {
	global *streamaggr.Aggregators
	perCtx []*streamaggr.Aggregators
}
