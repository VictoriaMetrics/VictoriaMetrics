package common

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
	"github.com/VictoriaMetrics/metrics"
)

var (
	streamAggrConfig = flag.String("streamAggr.config", "", "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/stream-aggregation.html . "+
		"See also -streamAggr.keepInput, -streamAggr.dropInput and -streamAggr.dedupInterval")
	streamAggrKeepInput = flag.Bool("streamAggr.keepInput", false, "Whether to keep all the input samples after the aggregation with -streamAggr.config. "+
		"By default, only aggregated samples are dropped, while the remaining samples are stored in the database. "+
		"See also -streamAggr.dropInput and https://docs.victoriametrics.com/stream-aggregation.html")
	streamAggrDropInput = flag.Bool("streamAggr.dropInput", false, "Whether to drop all the input samples after the aggregation with -streamAggr.config. "+
		"By default, only aggregated samples are dropped, while the remaining samples are stored in the database. "+
		"See also -streamAggr.keepInput and https://docs.victoriametrics.com/stream-aggregation.html")
	streamAggrDedupInterval = flag.Duration("streamAggr.dedupInterval", 0, "Input samples are de-duplicated with this interval before being aggregated. "+
		"Only the last sample per each time series per each interval is aggregated if the interval is greater than zero")
)

var (
	saCfgReloaderStopCh chan struct{}
	saCfgReloaderWG     sync.WaitGroup

	saCfgReloads   = metrics.NewCounter(`vminsert_streamagg_config_reloads_total`)
	saCfgReloadErr = metrics.NewCounter(`vminsert_streamagg_config_reloads_errors_total`)
	saCfgSuccess   = metrics.NewGauge(`vminsert_streamagg_config_last_reload_successful`, nil)
	saCfgTimestamp = metrics.NewCounter(`vminsert_streamagg_config_last_reload_success_timestamp_seconds`)

	sasGlobal atomic.Pointer[streamaggr.Aggregators]
)

// CheckStreamAggrConfig checks config pointed by -stramaggr.config
func CheckStreamAggrConfig() error {
	if *streamAggrConfig == "" {
		return nil
	}
	pushNoop := func(tss []prompbmarshal.TimeSeries) {}
	sas, err := streamaggr.LoadFromFile(*streamAggrConfig, pushNoop, *streamAggrDedupInterval)
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
		return
	}

	sighupCh := procutil.NewSighupChan()

	sas, err := streamaggr.LoadFromFile(*streamAggrConfig, pushAggregateSeries, *streamAggrDedupInterval)
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

	sasNew, err := streamaggr.LoadFromFile(*streamAggrConfig, pushAggregateSeries, *streamAggrDedupInterval)
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
}

type streamAggrCtx struct {
	mn  storage.MetricName
	tss [1]prompbmarshal.TimeSeries
}

func (ctx *streamAggrCtx) Reset() {
	ctx.mn.Reset()
	ts := &ctx.tss[0]
	promrelabel.CleanLabels(ts.Labels)
}

func (ctx *streamAggrCtx) push(mrs []storage.MetricRow, matchIdxs []byte) []byte {
	matchIdxs = bytesutil.ResizeNoCopyMayOverallocate(matchIdxs, len(mrs))
	for i := 0; i < len(matchIdxs); i++ {
		matchIdxs[i] = 0
	}

	mn := &ctx.mn
	tss := ctx.tss[:]
	ts := &tss[0]
	labels := ts.Labels
	samples := ts.Samples
	sas := sasGlobal.Load()
	var matchIdxsLocal []byte
	for idx, mr := range mrs {
		if err := mn.UnmarshalRaw(mr.MetricNameRaw); err != nil {
			logger.Panicf("BUG: cannot unmarshal recently marshaled MetricName: %s", err)
		}

		labels = append(labels[:0], prompbmarshal.Label{
			Name:  "__name__",
			Value: bytesutil.ToUnsafeString(mn.MetricGroup),
		})
		for _, tag := range mn.Tags {
			labels = append(labels, prompbmarshal.Label{
				Name:  bytesutil.ToUnsafeString(tag.Key),
				Value: bytesutil.ToUnsafeString(tag.Value),
			})
		}

		samples = append(samples[:0], prompbmarshal.Sample{
			Timestamp: mr.Timestamp,
			Value:     mr.Value,
		})

		ts.Labels = labels
		ts.Samples = samples

		matchIdxsLocal = sas.Push(tss, matchIdxsLocal)
		if matchIdxsLocal[0] != 0 {
			matchIdxs[idx] = 1
		}
	}

	return matchIdxs
}

func pushAggregateSeries(tss []prompbmarshal.TimeSeries) {
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
