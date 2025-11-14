package common

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ratelimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeserieslimits"
)

// StartIngestionRateLimiter starts ingestion rate limiter.
//
// Ingestion rate limiter must be started before Init() call.
//
// StopIngestionRateLimiter must be called before Stop() call in order to unblock all the callers
// to ingestion rate limiter. Otherwise deadlock may occur at Stop() call.
func StartIngestionRateLimiter(maxIngestionRate int) {
	if maxIngestionRate <= 0 {
		return
	}
	ingestionRateLimitReached := metrics.NewCounter(`vm_max_ingestion_rate_limit_reached_total`)
	ingestionRateLimiterStopCh = make(chan struct{})
	ingestionRateLimiter = ratelimiter.New(int64(maxIngestionRate), ingestionRateLimitReached, ingestionRateLimiterStopCh)
}

// StopIngestionRateLimiter stops ingestion rate limiter.
func StopIngestionRateLimiter() {
	if ingestionRateLimiterStopCh == nil {
		return
	}
	close(ingestionRateLimiterStopCh)
	ingestionRateLimiterStopCh = nil
}

var (
	ingestionRateLimiter       *ratelimiter.RateLimiter
	ingestionRateLimiterStopCh chan struct{}
)

// InsertCtx contains common bits for data points insertion.
type InsertCtx struct {
	Labels sortedLabels

	mrs           []storage.MetricRow
	mms           []metricsmetadata.Row
	metricNameBuf []byte

	relabelCtx    relabel.Ctx
	streamAggrCtx streamAggrCtx

	skipStreamAggr bool
}

// Reset resets ctx for future fill with rowsLen rows.
func (ctx *InsertCtx) Reset(rowsLen int) {
	labels := ctx.Labels
	for i := range labels {
		labels[i] = prompb.Label{}
	}
	ctx.Labels = labels[:0]

	mrs := ctx.mrs
	for i := range mrs {
		cleanMetricRow(&mrs[i])
	}
	mrs = slicesutil.SetLength(mrs, rowsLen)
	ctx.mrs = mrs[:0]
	mms := ctx.mms
	for i := range mms {
		cleanMetricMetadata(&mms[i])
	}
	ctx.mms = mms[:0]

	ctx.metricNameBuf = ctx.metricNameBuf[:0]
	ctx.relabelCtx.Reset()
	ctx.streamAggrCtx.Reset()
	ctx.skipStreamAggr = false
}

func cleanMetricRow(mr *storage.MetricRow) {
	mr.MetricNameRaw = nil
}

func cleanMetricMetadata(mm *metricsmetadata.Row) {
	mm.MetricFamilyName = nil
	mm.Unit = nil
	mm.Help = nil
	mm.Type = 0
	mm.ProjectID = 0
	mm.AccountID = 0
}

func (ctx *InsertCtx) marshalMetricNameRaw(prefix []byte, labels []prompb.Label) []byte {
	start := len(ctx.metricNameBuf)
	ctx.metricNameBuf = append(ctx.metricNameBuf, prefix...)
	ctx.metricNameBuf = storage.MarshalMetricNameRaw(ctx.metricNameBuf, labels)
	metricNameRaw := ctx.metricNameBuf[start:]
	return metricNameRaw[:len(metricNameRaw):len(metricNameRaw)]
}

// TryPrepareLabels prepares context labels to the ingestion
//
// It returns false if timeseries should be skipped
func (ctx *InsertCtx) TryPrepareLabels(hasRelabeling bool) bool {
	if hasRelabeling {
		ctx.ApplyRelabeling()
	}
	if len(ctx.Labels) == 0 {
		return false
	}
	if timeserieslimits.Enabled() && timeserieslimits.IsExceeding(ctx.Labels) {
		return false
	}
	ctx.SortLabelsIfNeeded()

	return true
}

// WriteDataPoint writes (timestamp, value) with the given prefix and labels into ctx buffer.
//
// caller should invoke TryPrepareLabels before using this function if needed
func (ctx *InsertCtx) WriteDataPoint(prefix []byte, labels []prompb.Label, timestamp int64, value float64) error {
	metricNameRaw := ctx.marshalMetricNameRaw(prefix, labels)
	return ctx.addRow(metricNameRaw, timestamp, value)
}

// WriteDataPointExt writes (timestamp, value) with the given metricNameRaw and labels into ctx buffer.
//
// caller must invoke TryPrepareLabels before using this function
//
// It returns metricNameRaw for the given labels if len(metricNameRaw) == 0.
func (ctx *InsertCtx) WriteDataPointExt(metricNameRaw []byte, labels []prompb.Label, timestamp int64, value float64) ([]byte, error) {
	if len(metricNameRaw) == 0 {
		metricNameRaw = ctx.marshalMetricNameRaw(nil, labels)
	}
	err := ctx.addRow(metricNameRaw, timestamp, value)
	return metricNameRaw, err
}

func (ctx *InsertCtx) addRow(metricNameRaw []byte, timestamp int64, value float64) error {
	mrs := ctx.mrs
	if cap(mrs) > len(mrs) {
		mrs = mrs[:len(mrs)+1]
	} else {
		mrs = append(mrs, storage.MetricRow{})
	}
	mr := &mrs[len(mrs)-1]
	ctx.mrs = mrs
	mr.MetricNameRaw = metricNameRaw
	mr.Timestamp = timestamp
	mr.Value = value
	if len(ctx.metricNameBuf) > 16*1024*1024 {
		if err := ctx.FlushBufs(); err != nil {
			return err
		}
	}
	return nil
}

// WriteMetadata writes given prometheus protobuf  metadata into the storage.
func (ctx *InsertCtx) WriteMetadata(mmpbs []prompb.MetricMetadata) error {
	if len(mmpbs) == 0 {
		return nil
	}
	mms := ctx.mms
	mms = slicesutil.SetLength(mms, len(mmpbs))
	for idx, mmpb := range mmpbs {
		mm := &mms[idx]
		mm.MetricFamilyName = bytesutil.ToUnsafeBytes(mmpb.MetricFamilyName)
		mm.Help = bytesutil.ToUnsafeBytes(mmpb.Help)
		mm.Type = mmpb.Type
		mm.Unit = bytesutil.ToUnsafeBytes(mmpb.Unit)
	}

	err := vmstorage.AddMetadataRows(mms)
	if err != nil {
		return &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf("cannot store metrics metadata: %w", err),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
	return nil
}

// WritePromMetadata writes given prometheus metric metadata into the storage
func (ctx *InsertCtx) WritePromMetadata(mmps []prometheus.Metadata) error {
	if len(mmps) == 0 {
		return nil
	}
	mms := ctx.mms
	mms = slicesutil.SetLength(mms, len(mmps))
	for idx, mmpb := range mmps {
		mm := &mms[idx]
		mm.MetricFamilyName = bytesutil.ToUnsafeBytes(mmpb.Metric)
		mm.Help = bytesutil.ToUnsafeBytes(mmpb.Help)
		mm.Type = mmpb.Type
	}

	err := vmstorage.AddMetadataRows(mms)
	if err != nil {
		return &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf("cannot store prometheus metrics metadata: %w", err),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
	return nil
}

// AddLabelBytes adds (name, value) label to ctx.Labels.
//
// name and value must exist until ctx.Labels is used.
func (ctx *InsertCtx) AddLabelBytes(name, value []byte) {
	if len(value) == 0 {
		// Skip labels without values, since they have no sense.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/600
		// Do not skip labels with empty name, since they are equal to __name__.
		return
	}
	ctx.Labels = append(ctx.Labels, prompb.Label{
		// Do not copy name and value contents for performance reasons.
		// This reduces GC overhead on the number of objects and allocations.
		Name:  bytesutil.ToUnsafeString(name),
		Value: bytesutil.ToUnsafeString(value),
	})
}

// AddLabel adds (name, value) label to ctx.Labels.
//
// name and value must exist until ctx.Labels is used.
func (ctx *InsertCtx) AddLabel(name, value string) {
	if len(value) == 0 {
		// Skip labels without values, since they have no sense.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/600
		// Do not skip labels with empty name, since they are equal to __name__.
		return
	}
	ctx.Labels = append(ctx.Labels, prompb.Label{
		// Do not copy name and value contents for performance reasons.
		// This reduces GC overhead on the number of objects and allocations.
		Name:  name,
		Value: value,
	})
}

// ApplyRelabeling applies relabeling to ic.Labels.
func (ctx *InsertCtx) ApplyRelabeling() {
	ctx.Labels = ctx.relabelCtx.ApplyRelabeling(ctx.Labels)
}

// FlushBufs flushes buffered rows to the underlying storage.
func (ctx *InsertCtx) FlushBufs() error {
	sas := sasGlobal.Load()
	if (sas.IsEnabled() || deduplicator != nil) && !ctx.skipStreamAggr {
		matchIdxs := matchIdxsPool.Get()
		matchIdxs.B = ctx.streamAggrCtx.push(ctx.mrs, matchIdxs.B)
		if !*streamAggrKeepInput {
			// Remove aggregated rows from ctx.mrs
			ctx.dropAggregatedRows(matchIdxs.B)
		}
		matchIdxsPool.Put(matchIdxs)
	}
	ingestionRateLimiter.Register(len(ctx.mrs))

	// There is no need in limiting the number of concurrent calls to vmstorage.AddRows() here,
	// since the number of concurrent FlushBufs() calls should be already limited via writeconcurrencylimiter
	// used at every stream.Parse() call under lib/protoparser/*

	err := vmstorage.AddRows(ctx.mrs)
	ctx.Reset(0)
	if err == nil {
		return nil
	}
	return &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf("cannot store metrics: %w", err),
		StatusCode: http.StatusServiceUnavailable,
	}
}

func (ctx *InsertCtx) dropAggregatedRows(matchIdxs []uint32) {
	dst := ctx.mrs[:0]
	src := ctx.mrs
	if !*streamAggrDropInput {
		for idx, match := range matchIdxs {
			if match == 1 {
				continue
			}
			dst = append(dst, src[idx])
		}
	}
	tail := src[len(dst):]
	for i := range tail {
		cleanMetricRow(&tail[i])
	}
	ctx.mrs = dst
}

var matchIdxsPool slicesutil.BufferPool[uint32]
