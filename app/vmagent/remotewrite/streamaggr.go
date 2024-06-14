package remotewrite

import (
	"flag"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
)

var (
	// Global config
	streamAggrGlobalConfig = flag.String("streamAggr.config", "", "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/stream-aggregation/ . "+
		"See also -streamAggr.keepInput, -streamAggr.dropInput and -streamAggr.dedupInterval")
	streamAggrConfigCheckInterval = flag.Duration("streamAggr.configCheckInterval", 0, "Interval for checking changes in -streamAggr.config "+
		"and -remoteWrite.streamAggr.config")
	streamAggrGlobalKeepInput = flag.Bool("streamAggr.keepInput", false, "Whether to keep all the input samples after the aggregation "+
		"with -streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to remote storages write. See also -streamAggr.dropInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrGlobalDropInput = flag.Bool("streamAggr.dropInput", false, "Whether to drop all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to remote storages write. See also -streamAggr.keepInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrGlobalDedupInterval = flagutil.NewDuration("streamAggr.dedupInterval", "0s", "Input samples are de-duplicated with this interval on "+
		"aggregator before optional aggregation with -streamAggr.config . "+
		"See also -dedup.minScrapeInterval and https://docs.victoriametrics.com/stream-aggregation/#deduplication")
	streamAggrGlobalIgnoreOldSamples = flag.Bool("streamAggr.ignoreOldSamples", false, "Whether to ignore input samples with old timestamps outside the "+
		"current aggregation interval for aggregator. "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignoring-old-samples")
	streamAggrGlobalIgnoreFirstIntervals = flag.Int("streamAggr.ignoreFirstIntervals", 0, "Number of aggregation intervals to skip after the start for "+
		"aggregator. Increase this value if you observe incorrect aggregation results after vmagent restarts. It could be caused by receiving unordered delayed data from "+
		"clients pushing data into the vmagent. See https://docs.victoriametrics.com/stream-aggregation/#ignore-aggregation-intervals-on-start")
	streamAggrGlobalDropInputLabels = flagutil.NewArrayString("streamAggr.dropInputLabels", "An optional list of labels to drop from samples for aggregator "+
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
	streamAggrDropInputLabels = flagutil.NewArrayString("remoteWrite.streamAggr.dropInputLabels", "An optional list of labels to drop from samples "+
		"before stream de-duplication and aggregation . See https://docs.victoriametrics.com/stream-aggregation/#dropping-unneeded-labels")
)

// CheckStreamAggrConfigs checks -remoteWrite.streamAggr.config and -streamAggr.config.
func CheckStreamAggrConfigs() error {
	pushNoop := func(_ []prompbmarshal.TimeSeries) {}

	if _, err := newStreamAggrConfig(-1, pushNoop); err != nil {
		return fmt.Errorf("could not load -streamAggr.config stream aggregation config: %w", err)
	}
	if len(*streamAggrConfig) > len(*remoteWriteURLs) {
		return fmt.Errorf("too many -remoteWrite.streamAggr.config args: %d; it mustn't exceed the number of -remoteWrite.url args: %d",
			len(*streamAggrConfig), len(*remoteWriteURLs))
	}
	for idx := range *streamAggrConfig {
		if _, err := newStreamAggrConfig(idx, pushNoop); err != nil {
			return err
		}
	}
	return nil
}

// HasAnyStreamAggrConfigured checks if any streaming aggregation config provided
func HasAnyStreamAggrConfigured() bool {
	return len(*streamAggrConfig) > 0 || *streamAggrGlobalConfig != ""
}

func reloadStreamAggrConfigs() {
	reloadStreamAggrConfig(-1)
	for idx := range rwctxs {
		reloadStreamAggrConfig(idx)
	}
}

func reloadStreamAggrConfig(idx int) {
	path, _ := getStreamAggrOpts(idx)
	var sas *streamaggr.Aggregators
	var f string

	if idx < 0 {
		f = "-streamAggr.config"
		sas = sasGlobal.Load()
	} else {
		f = "-remoteWrite.streamAggr.config"
		sas = rwctxs[idx].sas.Load()
	}
	if sas == nil {
		return
	}
	if err := sas.Reload(); err != nil {
		logger.Errorf("cannot reload %s=%q; continue using the previously loaded config; error: %s", f, path, err)
		return
	}
}

func getStreamAggrOpts(idx int) (string, streamaggr.Options) {
	if idx < 0 {
		return *streamAggrGlobalConfig, streamaggr.Options{
			DedupInterval:        streamAggrGlobalDedupInterval.Duration(),
			DropInputLabels:      *streamAggrGlobalDropInputLabels,
			IgnoreOldSamples:     *streamAggrGlobalIgnoreOldSamples,
			IgnoreFirstIntervals: *streamAggrGlobalIgnoreFirstIntervals,
			Alias:                "global",
		}
	}
	url := fmt.Sprintf("%d:secret-url", idx+1)
	if *showRemoteWriteURL {
		url = fmt.Sprintf("%d:%s", idx+1, remoteWriteURLs.GetOptionalArg(idx))
	}
	opts := streamaggr.Options{
		DedupInterval:        streamAggrDedupInterval.GetOptionalArg(idx),
		DropInputLabels:      *streamAggrDropInputLabels,
		IgnoreOldSamples:     streamAggrIgnoreOldSamples.GetOptionalArg(idx),
		IgnoreFirstIntervals: *streamAggrIgnoreFirstIntervals,
		Alias:                url,
	}

	if len(*streamAggrConfig) == 0 {
		return "", opts
	}
	return streamAggrConfig.GetOptionalArg(idx), opts
}

func newStreamAggrConfigWithOpts(pushFunc streamaggr.PushFunc, path string, opts streamaggr.Options) (*streamaggr.Aggregators, error) {
	if len(path) == 0 {
		// Skip empty stream aggregation config.
		return nil, nil
	}
	return streamaggr.LoadFromFile(path, pushFunc, opts)
}

func newStreamAggrConfig(idx int, pushFunc streamaggr.PushFunc) (*streamaggr.Aggregators, error) {
	path, opts := getStreamAggrOpts(idx)
	return newStreamAggrConfigWithOpts(pushFunc, path, opts)
}
