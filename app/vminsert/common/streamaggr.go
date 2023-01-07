package common

import (
	"flag"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
)

var (
	streamAggrConfig = flag.String("streamAggr.config", "", "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/stream-aggregation.html . "+
		"See also -remoteWrite.streamAggr.keepInput")
	streamAggrKeepInput = flag.Bool("streamAggr.keepInput", false, "Whether to keep input samples after the aggregation with -streamAggr.config. "+
		"By default the input is dropped after the aggregation, so only the aggregate data is stored. "+
		"See https://docs.victoriametrics.com/stream-aggregation.html")
)

// InitStreamAggr must be called after flag.Parse and before using the common package.
//
// MustStopStreamAggr must be called when stream aggr is no longer needed.
func InitStreamAggr() {
	if *streamAggrConfig == "" {
		// Nothing to initialize
		return
	}
	a, err := streamaggr.LoadFromFile(*streamAggrConfig, pushAggregateSeries)
	if err != nil {
		logger.Fatalf("cannot load -streamAggr.config=%q: %s", *streamAggrConfig, err)
	}
	sa = a
}

// MustStopStreamAggr stops stream aggregators.
func MustStopStreamAggr() {
	sa.MustStop()
	sa = nil
}

var sa *streamaggr.Aggregators

type streamAggrCtx struct {
	mn  storage.MetricName
	tss [1]prompbmarshal.TimeSeries
}

func (ctx *streamAggrCtx) Reset() {
	ctx.mn.Reset()
	ts := &ctx.tss[0]
	promrelabel.CleanLabels(ts.Labels)
}

func (ctx *streamAggrCtx) push(mrs []storage.MetricRow) {
	mn := &ctx.mn
	tss := ctx.tss[:]
	ts := &tss[0]
	labels := ts.Labels
	samples := ts.Samples
	for _, mr := range mrs {
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

		sa.Push(tss)
	}
}

func pushAggregateSeries(tss []prompbmarshal.TimeSeries) {
	currentTimestamp := int64(fasttime.UnixTimestamp()) * 1000
	var ctx InsertCtx
	ctx.Reset(len(tss))
	ctx.skipStreamAggr = true
	for _, ts := range tss {
		labels := ts.Labels
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
