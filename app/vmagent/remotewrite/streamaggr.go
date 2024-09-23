package remotewrite

import (
	"flag"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
	"github.com/VictoriaMetrics/metrics"
)

var (
	// Global config
	streamAggrGlobalConfig = flag.String("streamAggr.config", "", "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/stream-aggregation/ . "+
		"See also -streamAggr.keepInput, -streamAggr.dropInput and -streamAggr.dedupInterval")
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
	streamAggrConfig = flagutil.NewArrayString("remoteWrite.streamAggr.config", "Optional path to file with stream aggregation config for the corresponding -remoteWrite.url. "+
		"See https://docs.victoriametrics.com/stream-aggregation/ . "+
		"See also -remoteWrite.streamAggr.keepInput, -remoteWrite.streamAggr.dropInput and -remoteWrite.streamAggr.dedupInterval")
	streamAggrDropInput = flagutil.NewArrayBool("remoteWrite.streamAggr.dropInput", "Whether to drop all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config at the corresponding -remoteWrite.url. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.keepInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrKeepInput = flagutil.NewArrayBool("remoteWrite.streamAggr.keepInput", "Whether to keep all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config at the corresponding -remoteWrite.url. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.dropInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrDedupInterval = flagutil.NewArrayDuration("remoteWrite.streamAggr.dedupInterval", 0, "Input samples are de-duplicated with this interval before optional aggregation "+
		"with -remoteWrite.streamAggr.config at the corresponding -remoteWrite.url. See also -dedup.minScrapeInterval and https://docs.victoriametrics.com/stream-aggregation/#deduplication")
	streamAggrIgnoreOldSamples = flagutil.NewArrayBool("remoteWrite.streamAggr.ignoreOldSamples", "Whether to ignore input samples with old timestamps outside the current "+
		"aggregation interval for the corresponding -remoteWrite.streamAggr.config at the corresponding -remoteWrite.url. "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignoring-old-samples")
	streamAggrIgnoreFirstIntervals = flagutil.NewArrayInt("remoteWrite.streamAggr.ignoreFirstIntervals", 0, "Number of aggregation intervals to skip after the start "+
		"for the corresponding -remoteWrite.streamAggr.config at the corresponding -remoteWrite.url. Increase this value if "+
		"you observe incorrect aggregation results after vmagent restarts. It could be caused by receiving bufferred delayed data from clients pushing data into the vmagent. "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignore-aggregation-intervals-on-start")
	streamAggrDropInputLabels = flagutil.NewArrayString("remoteWrite.streamAggr.dropInputLabels", "An optional list of labels to drop from samples "+
		"before stream de-duplication and aggregation with -remoteWrite.streamAggr.config and -remoteWrite.streamAggr.dedupInterval at the corresponding -remoteWrite.url. "+
		"Multiple labels per remoteWrite.url must be delimited by '^^': -remoteWrite.streamAggr.dropInputLabels='replica^^az,replica'. "+
		"See https://docs.victoriametrics.com/stream-aggregation/#dropping-unneeded-labels")
)

// CheckStreamAggrConfigs checks -remoteWrite.streamAggr.config and -streamAggr.config.
func CheckStreamAggrConfigs() error {
	// Check global config
	sas, err := newStreamAggrConfigGlobal()
	if err != nil {
		return err
	}
	sas.MustStop()

	if len(*streamAggrConfig) > len(*remoteWriteURLs) {
		return fmt.Errorf("too many -remoteWrite.streamAggr.config args: %d; it mustn't exceed the number of -remoteWrite.url args: %d", len(*streamAggrConfig), len(*remoteWriteURLs))
	}

	pushNoop := func(_ []prompbmarshal.TimeSeries) {}
	for idx := range *streamAggrConfig {
		sas, err := newStreamAggrConfigPerURL(idx, pushNoop)
		if err != nil {
			return err
		}
		sas.MustStop()
	}
	return nil
}

func reloadStreamAggrConfigs() {
	reloadStreamAggrConfigGlobal()
	for _, rwctx := range rwctxsGlobal {
		rwctx.reloadStreamAggrConfig()
	}
}

func reloadStreamAggrConfigGlobal() {
	path := *streamAggrGlobalConfig
	if path == "" {
		return
	}

	logger.Infof("reloading stream aggregation configs pointed by -streamAggr.config=%q", path)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reloads_total{path=%q}`, path)).Inc()

	sasNew, err := newStreamAggrConfigGlobal()
	if err != nil {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reloads_errors_total{path=%q}`, path)).Inc()
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, path)).Set(0)
		logger.Errorf("cannot reload -streamAggr.config=%q; continue using the previously loaded config; error: %s", path, err)
		return
	}

	sas := sasGlobal.Load()
	if !sasNew.Equal(sas) {
		sasOld := sasGlobal.Swap(sasNew)
		sasOld.MustStop()
		logger.Infof("successfully reloaded -streamAggr.config=%q", path)
	} else {
		sasNew.MustStop()
		logger.Infof("-streamAggr.config=%q wasn't changed since the last reload", path)
	}
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, path)).Set(1)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_success_timestamp_seconds{path=%q}`, path)).Set(fasttime.UnixTimestamp())
}

func initStreamAggrConfigGlobal() {
	sas, err := newStreamAggrConfigGlobal()
	if err != nil {
		logger.Fatalf("cannot initialize gloabl stream aggregators: %s", err)
	}
	if sas != nil {
		filePath := sas.FilePath()
		sasGlobal.Store(sas)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, filePath)).Set(1)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_success_timestamp_seconds{path=%q}`, filePath)).Set(fasttime.UnixTimestamp())
	}
	dedupInterval := streamAggrGlobalDedupInterval.Duration()
	if dedupInterval > 0 {
		deduplicatorGlobal = streamaggr.NewDeduplicator(pushToRemoteStoragesTrackDropped, dedupInterval, *streamAggrGlobalDropInputLabels, "dedup-global")
	}
}

func (rwctx *remoteWriteCtx) initStreamAggrConfig() {
	idx := rwctx.idx

	sas, err := rwctx.newStreamAggrConfig()
	if err != nil {
		logger.Fatalf("cannot initialize stream aggregators: %s", err)
	}
	if sas != nil {
		filePath := sas.FilePath()
		rwctx.sas.Store(sas)
		rwctx.streamAggrKeepInput = streamAggrKeepInput.GetOptionalArg(idx)
		rwctx.streamAggrDropInput = streamAggrDropInput.GetOptionalArg(idx)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, filePath)).Set(1)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_success_timestamp_seconds{path=%q}`, filePath)).Set(fasttime.UnixTimestamp())
	}
	dedupInterval := streamAggrDedupInterval.GetOptionalArg(idx)
	if dedupInterval > 0 {
		alias := fmt.Sprintf("dedup-%d", idx+1)
		var dropLabels []string
		if streamAggrDropInputLabels.GetOptionalArg(idx) != "" {
			dropLabels = strings.Split(streamAggrDropInputLabels.GetOptionalArg(idx), "^^")
		}
		rwctx.deduplicator = streamaggr.NewDeduplicator(rwctx.pushInternalTrackDropped, dedupInterval, dropLabels, alias)
	}
}

func (rwctx *remoteWriteCtx) reloadStreamAggrConfig() {
	path := streamAggrConfig.GetOptionalArg(rwctx.idx)
	if path == "" {
		return
	}

	logger.Infof("reloading stream aggregation configs pointed by -remoteWrite.streamAggr.config=%q", path)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reloads_total{path=%q}`, path)).Inc()

	sasNew, err := rwctx.newStreamAggrConfig()
	if err != nil {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reloads_errors_total{path=%q}`, path)).Inc()
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, path)).Set(0)
		logger.Errorf("cannot reload -remoteWrite.streamAggr.config=%q; continue using the previously loaded config; error: %s", path, err)
		return
	}

	sas := rwctx.sas.Load()
	if !sasNew.Equal(sas) {
		sasOld := rwctx.sas.Swap(sasNew)
		sasOld.MustStop()
		logger.Infof("successfully reloaded -remoteWrite.streamAggr.config=%q", path)
	} else {
		sasNew.MustStop()
		logger.Infof("-remoteWrite.streamAggr.config=%q wasn't changed since the last reload", path)
	}
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_successful{path=%q}`, path)).Set(1)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_config_reload_success_timestamp_seconds{path=%q}`, path)).Set(fasttime.UnixTimestamp())
}

func newStreamAggrConfigGlobal() (*streamaggr.Aggregators, error) {
	path := *streamAggrGlobalConfig
	if path == "" {
		return nil, nil
	}

	opts := &streamaggr.Options{
		DedupInterval:        streamAggrGlobalDedupInterval.Duration(),
		DropInputLabels:      *streamAggrGlobalDropInputLabels,
		IgnoreOldSamples:     *streamAggrGlobalIgnoreOldSamples,
		IgnoreFirstIntervals: *streamAggrGlobalIgnoreFirstIntervals,
		KeepInput:            *streamAggrGlobalKeepInput,
	}

	sas, err := streamaggr.LoadFromFile(path, pushToRemoteStoragesTrackDropped, opts, "global")
	if err != nil {
		return nil, fmt.Errorf("cannot load -streamAggr.config=%q: %w", *streamAggrGlobalConfig, err)
	}
	return sas, nil
}

func (rwctx *remoteWriteCtx) newStreamAggrConfig() (*streamaggr.Aggregators, error) {
	return newStreamAggrConfigPerURL(rwctx.idx, rwctx.pushInternalTrackDropped)
}

func newStreamAggrConfigPerURL(idx int, pushFunc streamaggr.PushFunc) (*streamaggr.Aggregators, error) {
	path := streamAggrConfig.GetOptionalArg(idx)
	if path == "" {
		return nil, nil
	}

	alias := fmt.Sprintf("%d:secret-url", idx+1)
	if *showRemoteWriteURL {
		alias = fmt.Sprintf("%d:%s", idx+1, remoteWriteURLs.GetOptionalArg(idx))
	}
	var dropLabels []string
	if streamAggrDropInputLabels.GetOptionalArg(idx) != "" {
		dropLabels = strings.Split(streamAggrDropInputLabels.GetOptionalArg(idx), "^^")
	}
	opts := &streamaggr.Options{
		DedupInterval:        streamAggrDedupInterval.GetOptionalArg(idx),
		DropInputLabels:      dropLabels,
		IgnoreOldSamples:     streamAggrIgnoreOldSamples.GetOptionalArg(idx),
		IgnoreFirstIntervals: streamAggrIgnoreFirstIntervals.GetOptionalArg(idx),
		KeepInput:            streamAggrKeepInput.GetOptionalArg(idx),
	}

	sas, err := streamaggr.LoadFromFile(path, pushFunc, opts, alias)
	if err != nil {
		return nil, fmt.Errorf("cannot load -remoteWrite.streamAggr.config=%q: %w", path, err)
	}
	return sas, nil
}
