package common

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ratelimiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeserieslimits"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vminsertapi"
)

func SetVMInsertAPI(api vminsertapi.API) {
	vmInsertAPI = api
}

var vmInsertAPI vminsertapi.API

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

	at auth.Token
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

func (ctx *InsertCtx) marshalMetricNameRaw(prefix []byte, at *auth.Token, labels []prompb.Label) []byte {
	start := len(ctx.metricNameBuf)
	ctx.metricNameBuf = append(ctx.metricNameBuf, prefix...)
	ctx.metricNameBuf = storage.MarshalMetricNameRaw(ctx.metricNameBuf, at.AccountID, at.ProjectID, labels)
	metricNameRaw := ctx.metricNameBuf[start:]
	return metricNameRaw[:len(metricNameRaw):len(metricNameRaw)]
}

// TryPrepareLabels prepares context labels to the ingestion
//
// It returns false if timeseries should be skipped
func (ctx *InsertCtx) TryPrepareLabels() bool {
	if len(ctx.Labels) == 0 {
		return false
	}
	if timeserieslimits.Enabled() && timeserieslimits.IsExceeding(ctx.Labels) {
		return false
	}
	ctx.SortLabelsIfNeeded()

	return true
}

// WriteDataPointExt writes (timestamp, value) with the given metricNameRaw and labels into ctx buffer.
//
// caller must invoke TryPrepareLabels before using this function
//
// It returns metricNameRaw for the given labels if len(metricNameRaw) == 0.
func (ctx *InsertCtx) WriteDataPointExt(metricNameRaw []byte, at *auth.Token, labels []prompb.Label, timestamp int64, value float64) ([]byte, error) {
	if len(metricNameRaw) == 0 {
		metricNameRaw = ctx.marshalMetricNameRaw(nil, at, labels)
	}
	err := ctx.addRow(metricNameRaw, timestamp, value)
	if len(ctx.metricNameBuf) == 0 {
		metricNameRaw = nil
	}
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
		if err := ctx.flushRows(); err != nil {
			return err
		}
	}
	return nil
}

// WriteMetadata writes given prometheus protobuf  metadata into the storage.
func (ctx *InsertCtx) WriteMetadata(at *auth.Token, mmpbs []prompb.MetricMetadata) (int, error) {
	if len(mmpbs) == 0 {
		return 0, nil
	}
	mms := ctx.mms
	mms = slicesutil.SetLength(mms, len(mmpbs))
	var cnt int
	for _, mmpb := range mmpbs {
		if timeserieslimits.IsMetricMetadataExceeding(&mmpb) {
			continue
		}
		atLocal := ctx.GetLocalAuthTokenForMetadata(at, &mmpb)
		mm := &mms[cnt]
		if atLocal != nil {
			mm.AccountID = atLocal.AccountID
			mm.ProjectID = atLocal.ProjectID
		}
		mm.MetricFamilyName = bytesutil.ToUnsafeBytes(mmpb.MetricFamilyName)
		mm.Help = bytesutil.ToUnsafeBytes(mmpb.Help)
		mm.Type = mmpb.Type
		mm.Unit = bytesutil.ToUnsafeBytes(mmpb.Unit)
		cnt++
	}
	mms = mms[:cnt]
	ctx.mms = mms

	err := vmInsertAPI.WriteMetadata(mms)
	if err != nil {
		return 0, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf("cannot store metrics metadata: %w", err),
			StatusCode: http.StatusServiceUnavailable,
		}
	}
	return len(mms), nil
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

// FlushBufs flushes buffered rows to the underlying storage.
func (ctx *InsertCtx) FlushBufs() error {
	defer ctx.Reset(0)
	return ctx.flushRows()
}

func (ctx *InsertCtx) flushRows() error {
	if len(ctx.mrs) == 0 {
		return nil
	}
	ingestionRateLimiter.Register(len(ctx.mrs))

	// There is no need in limiting the number of concurrent calls to vmstorage.AddRows() here,
	// since the number of concurrent FlushBufs() calls should be already limited via writeconcurrencylimiter
	// used at every stream.Parse() call under lib/protoparser/*

	if vmInsertAPI.IsReadOnly() {
		return &httpserver.ErrorWithStatusCode{
			Err:        storage.ErrReadOnly,
			StatusCode: http.StatusServiceUnavailable,
		}
	}
	err := vmInsertAPI.WriteRows(ctx.mrs)
	for i := range ctx.mrs {
		cleanMetricRow(&ctx.mrs[i])
	}
	ctx.mrs = ctx.mrs[:0]
	ctx.metricNameBuf = ctx.metricNameBuf[:0]
	if err == nil {
		return nil
	}
	return &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf("cannot store metrics: %w", err),
		StatusCode: http.StatusServiceUnavailable,
	}
}

// GetLocalAuthToken obtains auth.Token from context labels vm_account_id and vm_project_id if at is nil.
//
// At is returned as is if it isn't nil.
//
// The vm_account_id and vm_project_id labels are automatically removed from the ctx.
func (ctx *InsertCtx) GetLocalAuthToken(at *auth.Token) *auth.Token {
	if at != nil {
		return at
	}
	accountID := uint32(0)
	projectID := uint32(0)
	tmpLabels := ctx.Labels[:0]
	for _, label := range ctx.Labels {
		switch string(label.Name) {
		case "vm_account_id":
			accountID = parseUint32(label.Value)
			continue
		case "vm_project_id":
			projectID = parseUint32(label.Value)
			continue
		// do not remove labels from labelSet for backward-compatibility
		// previous realisation kept it
		case "VictoriaMetrics_AccountID":
			accountID = parseUint32(label.Value)
		case "VictoriaMetrics_ProjectID":
			projectID = parseUint32(label.Value)
		}
		tmpLabels = append(tmpLabels, label)
	}
	cleanLabels := ctx.Labels[len(tmpLabels):]
	for i := range cleanLabels {
		cleanLabels[i] = prompb.Label{}
	}
	ctx.Labels = tmpLabels
	ctx.at.Set(accountID, projectID)
	return &ctx.at
}

// GetLocalAuthTokenForMetadata obtains auth.Token from given metrics metadata if at is nil.
//
// At is returned as is if it isn't nil.
func (ctx *InsertCtx) GetLocalAuthTokenForMetadata(at *auth.Token, mm *prompb.MetricMetadata) *auth.Token {
	if at != nil {
		return at
	}

	ctx.at.Set(mm.AccountID, mm.ProjectID)
	return &ctx.at
}

func parseUint32(s string) uint32 {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}
