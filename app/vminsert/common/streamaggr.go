package common

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
	"github.com/VictoriaMetrics/metrics"
)

var (
	streamAggrConfig = flag.String("streamAggr.config", "", "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/victoriametrics/stream-aggregation/ . "+
		"See also -streamAggr.keepInput, -streamAggr.dropInput and -streamAggr.dedupInterval")
	streamAggrKeepInput = flag.Bool("streamAggr.keepInput", false, "Whether to keep input samples that match any rule in -streamAggr.config. "+
		"By default, matched raw samples are aggregated and dropped, while unmatched samples are written to the remote storage. "+
		"See also -streamAggr.dropInput and https://docs.victoriametrics.com/victoriametrics/stream-aggregation/")
	streamAggrDropInput = flag.Bool("streamAggr.dropInput", false, "Whether to drop input samples that not matching any rule in -streamAggr.config. "+
		"By default, only matched raw samples are dropped, while unmatched samples are written to the remote storage."+
		"See also -streamAggr.keepInput and https://docs.victoriametrics.com/victoriametrics/stream-aggregation/")
	streamAggrDedupInterval = flag.Duration("streamAggr.dedupInterval", 0, "Input samples are de-duplicated with this interval before optional aggregation with -streamAggr.config . "+
		"See also -streamAggr.dropInputLabels and -dedup.minScrapeInterval and https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#deduplication")
	streamAggrDropInputLabels = flagutil.NewArrayString("streamAggr.dropInputLabels", "An optional list of labels to drop from samples "+
		"before stream de-duplication and aggregation . See https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#dropping-unneeded-labels")
	streamAggrIgnoreOldSamples = flag.Bool("streamAggr.ignoreOldSamples", false, "Whether to ignore input samples with old timestamps outside the current aggregation interval. "+
		"See https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#ignoring-old-samples")
	streamAggrIgnoreFirstIntervals = flag.Int("streamAggr.ignoreFirstIntervals", 0, "Number of aggregation intervals to skip after the start. Increase this value if you observe incorrect aggregation results after restarts. It could be caused by receiving unordered delayed data from clients pushing data into the database. "+
		"See https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#ignore-aggregation-intervals-on-start")
	streamAggrEnableWindows = flag.Bool("streamAggr.enableWindows", false, "Enables aggregation within fixed windows for all aggregators. "+
		"This allows to get more precise results, but impacts resource usage as it requires twice more memory to store two states. "+
		"See https://docs.victoriametrics.com/victoriametrics/stream-aggregation/#aggregation-windows.")
)

var (
	saCfgReloaderStopCh chan struct{}
	saCfgReloaderWG     sync.WaitGroup

	saCfgReloads   *metrics.Counter
	saCfgReloadErr *metrics.Counter
	saCfgSuccess   *metrics.Gauge
	saCfgTimestamp *metrics.Counter

	sasGlobal    atomic.Pointer[streamaggr.Aggregators]
	deduplicator *streamaggr.Deduplicator
)

// CheckStreamAggrConfig checks config pointed by -stramaggr.config
func CheckStreamAggrConfig() error {
	if *streamAggrConfig == "" {
		return nil
	}
	pushNoop := func(_ []prompb.TimeSeries) {}
	opts := &streamaggr.Options{
		DedupInterval:        *streamAggrDedupInterval,
		DropInputLabels:      *streamAggrDropInputLabels,
		IgnoreOldSamples:     *streamAggrIgnoreOldSamples,
		IgnoreFirstIntervals: *streamAggrIgnoreFirstIntervals,
		EnableWindows:        *streamAggrEnableWindows,
	}
	sas, err := streamaggr.LoadFromFile(*streamAggrConfig, pushNoop, opts, "global")
	if err != nil {
		return fmt.Errorf("error when loading -streamAggr.config=%q: %w", *streamAggrConfig, err)
	}
	sas.MustStop()
	return nil
}

// InitStreamAggr must be called after flag.Parse and before using the common package.
//
// MustStopStreamAggr must be called when stream aggr is no longer needed.
func InitStreamAggr() {
	saCfgReloaderStopCh = make(chan struct{})
	if *streamAggrConfig == "" {
		if *streamAggrDedupInterval > 0 {
			deduplicator = streamaggr.NewDeduplicator(pushAggregateSeries, *streamAggrEnableWindows, *streamAggrDedupInterval, *streamAggrDropInputLabels, "global")
		}
		return
	}

	saCfgReloads = metrics.NewCounter(`vminsert_streamagg_config_reloads_total`)
	saCfgReloadErr = metrics.NewCounter(`vminsert_streamagg_config_reloads_errors_total`)
	saCfgSuccess = metrics.NewGauge(`vminsert_streamagg_config_last_reload_successful`, nil)
	saCfgTimestamp = metrics.NewCounter(`vminsert_streamagg_config_last_reload_success_timestamp_seconds`)

	sighupCh := procutil.NewSighupChan()

	opts := &streamaggr.Options{
		DedupInterval:        *streamAggrDedupInterval,
		DropInputLabels:      *streamAggrDropInputLabels,
		IgnoreOldSamples:     *streamAggrIgnoreOldSamples,
		IgnoreFirstIntervals: *streamAggrIgnoreFirstIntervals,
	}
	sas, err := streamaggr.LoadFromFile(*streamAggrConfig, pushAggregateSeries, opts, "global")
	if err != nil {
		logger.Fatalf("cannot load -streamAggr.config=%q: %s", *streamAggrConfig, err)
	}

	sasGlobal.Store(sas)
	saCfgSuccess.Set(1)
	saCfgTimestamp.Set(fasttime.UnixTimestamp())

	// Start config reloader.
	saCfgReloaderWG.Add(1)
	go func() {
		defer saCfgReloaderWG.Done()
		for {
			select {
			case <-sighupCh:
			case <-saCfgReloaderStopCh:
				return
			}
			reloadStreamAggrConfig()
		}
	}()
}

func reloadStreamAggrConfig() {
	logger.Infof("reloading -streamAggr.config=%q", *streamAggrConfig)
	saCfgReloads.Inc()

	opts := &streamaggr.Options{
		DedupInterval:        *streamAggrDedupInterval,
		DropInputLabels:      *streamAggrDropInputLabels,
		IgnoreOldSamples:     *streamAggrIgnoreOldSamples,
		IgnoreFirstIntervals: *streamAggrIgnoreFirstIntervals,
	}
	sasNew, err := streamaggr.LoadFromFile(*streamAggrConfig, pushAggregateSeries, opts, "global")
	if err != nil {
		saCfgSuccess.Set(0)
		saCfgReloadErr.Inc()
		logger.Errorf("cannot reload -streamAggr.config=%q: use the previously loaded config; error: %s", *streamAggrConfig, err)
		return
	}
	sas := sasGlobal.Load()
	if !sasNew.Equal(sas) {
		sasOld := sasGlobal.Swap(sasNew)
		sasOld.MustStop()
		logger.Infof("successfully reloaded stream aggregation config at -streamAggr.config=%q", *streamAggrConfig)
	} else {
		logger.Infof("nothing changed in -streamAggr.config=%q", *streamAggrConfig)
		sasNew.MustStop()
	}
	saCfgSuccess.Set(1)
	saCfgTimestamp.Set(fasttime.UnixTimestamp())
}

// MustStopStreamAggr stops stream aggregators.
func MustStopStreamAggr() {
	close(saCfgReloaderStopCh)
	saCfgReloaderWG.Wait()

	sas := sasGlobal.Swap(nil)
	sas.MustStop()

	if deduplicator != nil {
		deduplicator.MustStop()
		deduplicator = nil
	}
}

type streamAggrCtx struct {
	mn      storage.MetricName
	tss     []prompb.TimeSeries
	labels  []prompb.Label
	samples []prompb.Sample
	buf     []byte
}

func (ctx *streamAggrCtx) Reset() {
	ctx.mn.Reset()

	clear(ctx.tss)
	ctx.tss = ctx.tss[:0]

	clear(ctx.labels)
	ctx.labels = ctx.labels[:0]

	ctx.samples = ctx.samples[:0]
	ctx.buf = ctx.buf[:0]
}

func (ctx *streamAggrCtx) push(mrs []storage.MetricRow, matchIdxs []uint32) []uint32 {
	mn := &ctx.mn
	tss := ctx.tss
	labels := ctx.labels
	samples := ctx.samples
	buf := ctx.buf

	tssLen := len(tss)
	for _, mr := range mrs {
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			logger.Panicf("BUG: cannot unmarshal recently marshaled MetricName: %s", err)
		}

		labelsLen := len(labels)

		bufLen := len(buf)
		buf = append(buf, mn.MetricGroup...)
		metricGroup := bytesutil.ToUnsafeString(buf[bufLen:])
		labels = append(labels, prompb.Label{
			Name:  "__name__",
			Value: metricGroup,
		})

		for _, tag := range mn.Tags {
			bufLen = len(buf)
			buf = append(buf, tag.Key...)
			name := bytesutil.ToUnsafeString(buf[bufLen:])

			bufLen = len(buf)
			buf = append(buf, tag.Value...)
			value := bytesutil.ToUnsafeString(buf[bufLen:])
			labels = append(labels, prompb.Label{
				Name:  name,
				Value: value,
			})
		}

		samplesLen := len(samples)
		samples = append(samples, prompb.Sample{
			Timestamp: mr.Timestamp,
			Value:     mr.Value,
		})

		tss = append(tss, prompb.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[samplesLen:],
		})
	}
	ctx.tss = tss
	ctx.labels = labels
	ctx.samples = samples
	ctx.buf = buf

	tss = tss[tssLen:]

	sas := sasGlobal.Load()
	if sas.IsEnabled() {
		matchIdxs = sas.Push(tss, matchIdxs)
	} else if deduplicator != nil {
		matchIdxs = slicesutil.SetLength(matchIdxs, len(tss))
		for i := range matchIdxs {
			matchIdxs[i] = 1
		}
		deduplicator.Push(tss)
	}

	ctx.Reset()

	return matchIdxs
}

func pushAggregateSeries(tss []prompb.TimeSeries) {
	currentTimestamp := int64(fasttime.UnixTimestamp()) * 1000
	var ctx InsertCtx
	ctx.Reset(len(tss))
	ctx.skipStreamAggr = true
	for _, ts := range tss {
		labels := ts.Labels
		ctx.Labels = ctx.Labels[:0]
		for _, label := range labels {
			name := label.Name
			if name == "__name__" {
				name = ""
			}
			ctx.AddLabel(name, label.Value)
		}
		value := ts.Samples[0].Value
		if err := ctx.WriteDataPoint(nil, ctx.Labels, currentTimestamp, value); err != nil {
			logger.Errorf("cannot store aggregate series: %s", err)
			// Do not continue pushing the remaining samples, since it is likely they will return the same error.
			return
		}
	}
	// There is no need in limiting the number of concurrent calls to vmstorage.AddRows() here,
	// since the number of concurrent pushAggregateSeries() calls should be already limited by lib/streamaggr.
	if err := vmstorage.AddRows(ctx.mrs); err != nil {
		logger.Errorf("cannot flush aggregate series: %s", err)
	}
}
