package remotewrite

import (
	"flag"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

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
	streamAggrConfig = flagutil.NewDictValue("remoteWrite.streamAggr.config", "", '/', "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/stream-aggregation/ . "+
		"See also -remoteWrite.streamAggr.keepInput, -remoteWrite.streamAggr.dropInput and -remoteWrite.streamAggr.dedupInterval")
	streamAggrDropInput = flagutil.NewDictValue("remoteWrite.streamAggr.dropInput", false, '/', "Whether to drop all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.keepInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrKeepInput = flagutil.NewDictValue("remoteWrite.streamAggr.keepInput", false, '/', "Whether to keep all the input samples after the aggregation "+
		"with -remoteWrite.streamAggr.config. By default, only aggregates samples are dropped, while the remaining samples "+
		"are written to the corresponding -remoteWrite.url . See also -remoteWrite.streamAggr.dropInput and https://docs.victoriametrics.com/stream-aggregation/")
	streamAggrDedupInterval = flagutil.NewDictValue("remoteWrite.streamAggr.dedupInterval", time.Duration(0), '/', "Input samples are de-duplicated with this interval before optional aggregation "+
		"with -remoteWrite.streamAggr.config . See also -dedup.minScrapeInterval and https://docs.victoriametrics.com/stream-aggregation/#deduplication")
	streamAggrIgnoreOldSamples = flagutil.NewDictValue("remoteWrite.streamAggr.ignoreOldSamples", false, '/', "Whether to ignore input samples with old timestamps outside the current "+
		"aggregation interval for the corresponding -remoteWrite.streamAggr.config . "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignoring-old-samples")
	streamAggrIgnoreFirstIntervals = flagutil.NewDictValue("remoteWrite.streamAggr.ignoreFirstIntervals", 0, '/', "Number of aggregation intervals to skip after the start. Increase this value if "+
		"you observe incorrect aggregation results after vmagent restarts. It could be caused by receiving unordered delayed data from clients pushing data into the vmagent. "+
		"See https://docs.victoriametrics.com/stream-aggregation/#ignore-aggregation-intervals-on-start")
	streamAggrDropInputLabels = flagutil.NewDictValue("remoteWrite.streamAggr.dropInputLabels", "", '/', "An optional list of labels to drop from samples "+
		"before stream de-duplication and aggregation . See https://docs.victoriametrics.com/stream-aggregation/#dropping-unneeded-labels")
)

// CheckStreamAggrConfigs checks -remoteWrite.streamAggr.config and -streamAggr.config.
func CheckStreamAggrConfigs() error {
	pushNoop := func(_ []prompbmarshal.TimeSeries) {}
	as := &aggrConfigs{}
	if err := as.loadAggrConfigs(pushNoop); err != nil {
		return err
	}
	return nil
}

type aggrConfig struct {
	aggregator   *streamaggr.Aggregators
	deduplicator *streamaggr.Deduplicator
}

type aggrConfigs struct {
	global   atomic.Pointer[aggrConfig]
	perGroup []atomic.Pointer[aggrConfig]
	perCtx   []atomic.Pointer[aggrConfig]
}

func (as *aggrConfigs) reloadConfigs() {
	logger.Infof("reloading aggregators configs pointed by -streamAggr.config and -remoteWrite.streamAggr.config")
	if err := as.loadAggrConfigs(nil); err != nil {
		logger.Errorf("cannot reload aggregators configs; preserving the previous configs; error: %s", err)
		return
	}
	logger.Infof("successfully reloaded aggregators configs")
}

func (as *aggrConfigs) loadAggrConfigs(pushFunc streamaggr.PushFunc) error {
	if err := as.loadInternalAggrConfigs(pushFunc); err != nil {
		return err
	}
	return nil
}

func aggrMustStop(ac *atomic.Pointer[aggrConfig]) {
	sac := ac.Load()
	if sac != nil {
		sac.deduplicator.MustStop()
		sac.aggregator.MustStop()
	}
}

func (as *aggrConfigs) MustStop() {
	aggrMustStop(&as.global)
	for i := range as.perGroup {
		aggrMustStop(&as.perGroup[i])
	}
	for i := range as.perCtx {
		aggrMustStop(&as.perCtx[i])
	}
}

func updateAggrConfig(as *atomic.Pointer[aggrConfig], pushFunc streamaggr.PushFunc, opts streamaggr.Options, path, group, flag string) error {
	acOld := as.Load()
	acNew := aggrConfig{}
	var err error
	if path != "" {
		metricLabels := fmt.Sprintf("path=%q", path)
		if group != "" {
			flag = fmt.Sprintf(`-%s="%s/%s"`, flag, group, path)
		} else {
			flag = fmt.Sprintf("-%s=%q", flag, path)
		}
		metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_streamaggr_config_reloads_total{%s}", metricLabels)).Inc()
		acNew.aggregator, err = streamaggr.LoadFromFile(path, pushFunc, opts)
		if err != nil {
			metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_streamaggr_config_reloads_errors_total{%s}", metricLabels)).Inc()
			metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_streamaggr_config_last_reload_successful{%s}", metricLabels)).Set(0)
			return fmt.Errorf("cannot load -%s: %w", flag, err)
		}
		if acOld == nil {
			as.Store(&acNew)
		} else if !acOld.aggregator.Equal(acNew.aggregator) {
			acOld = as.Swap(&acNew)
			acOld.aggregator.MustStop()
			logger.Infof("successfully reloaded config at -%s", flag)
		} else {
			acNew.aggregator.MustStop()
			logger.Infof(`the config at -%s wasn't changed`, flag)
		}
		metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_streamaggr_config_last_reload_successful{%s}", metricLabels)).Set(1)
		metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_streamaggr_config_last_reload_success_timestamp_seconds{%s}", metricLabels)).Set(fasttime.UnixTimestamp())
	} else if opts.DedupInterval > 0 && acOld == nil {
		acNew.deduplicator = streamaggr.NewDeduplicator(pushFunc, opts.DedupInterval, opts.DropInputLabels, opts.Alias)
		as.Store(&acNew)
	}
	return nil
}

func (as *aggrConfigs) loadInternalAggrConfigs(pushFunc streamaggr.PushFunc) error {
	flag := "streamAggr.config"
	path, opts := getAggrOpts("", -1)
	var pf streamaggr.PushFunc
	if pushFunc == nil {
		pf = pushToRemoteStoragesDropFailed
	} else {
		pf = pushFunc
	}
	if err := updateAggrConfig(&as.global, pf, opts, path, "", flag); err != nil {
		return err
	}
	remoteWriteGroups := remoteWriteURLs.Keys()
	streamAggrGroups := streamAggrConfig.Keys()
	if len(as.perGroup) == 0 {
		as.perGroup = make([]atomic.Pointer[aggrConfig], len(remoteWriteGroups))
	}
	perCtxURLs := remoteWriteURLs.GetAll("")
	if len(as.perCtx) == 0 {
		as.perCtx = make([]atomic.Pointer[aggrConfig], len(perCtxURLs))
	}
	flag = "remoteWrite.streamAggr.config"
	for g, groupName := range streamAggrGroups {
		if pushFunc == nil {
			pf = rwctxGroups[g].pushInternalTrackDropped
		} else {
			pf = pushFunc
		}
		if !slices.Contains(remoteWriteGroups, groupName) {
			return fmt.Errorf("no -remoteWrite.url args in group %q, which appeared in -%s", groupName, flag)
		}
		aggrConfigs := streamAggrConfig.GetAll(groupName)
		if groupName != "" {
			if len(aggrConfigs) > 1 {
				return fmt.Errorf("no more than 1 -%s flag (defined %d) should be defined for group %q", flag, len(aggrConfigs), groupName)
			}
			path, opts := getAggrOpts(groupName, 0)
			if err := updateAggrConfig(&as.perGroup[g], pf, opts, path, groupName, flag); err != nil {
				return err
			}
		} else {
			perCtxURLs := remoteWriteURLs.GetAll(groupName)
			if len(aggrConfigs) > len(perCtxURLs) {
				return fmt.Errorf("too many ungrouped -%s args: %d; it mustn't exceed the number of -remoteWrite.url args: %d", flag,
					len(aggrConfigs), len(perCtxURLs))
			}
			for i := range aggrConfigs {
				if pushFunc == nil {
					pf = rwctxGroups[g].ctxs[i].pushInternalTrackDropped
				} else {
					pf = pushFunc
				}
				path, opts := getAggrOpts(groupName, i)
				if err := updateAggrConfig(&as.perCtx[i], pf, opts, path, groupName, flag); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// HasAnyStreamAggrConfigured checks if any streaming aggregation config provided
func HasAnyStreamAggrConfigured() bool {
	return len(streamAggrConfig.Keys()) > 0 || *streamAggrGlobalConfig != ""
}

func getAggrOpts(groupName string, idx int) (string, streamaggr.Options) {
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
		url = fmt.Sprintf("%d:%s", idx+1, remoteWriteURLs.GetOptionalArg(groupName, idx))
	}
	opts := streamaggr.Options{
		DedupInterval:        streamAggrDedupInterval.GetOptionalArg(groupName, idx),
		DropInputLabels:      streamAggrDropInputLabels.GetAll(groupName),
		IgnoreOldSamples:     streamAggrIgnoreOldSamples.GetOptionalArg(groupName, idx),
		IgnoreFirstIntervals: streamAggrIgnoreFirstIntervals.GetOptionalArg(groupName, idx),
		Alias:                url,
	}
	return streamAggrConfig.GetOptionalArg(groupName, idx), opts
}
