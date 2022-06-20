package seriesupdate

import (
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="update_series"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="update_series"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="update_series"}`)
)

// Returns local unique generationID.
func generateUniqueGenerationID() []byte {
	nextID := time.Now().UnixNano()
	return []byte(strconv.FormatInt(nextID, 10))
}

// InsertHandler processes `/api/v1/update/series` request.
//
func InsertHandler(at *auth.Token, req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		isGzipped := req.Header.Get("Content-Encoding") == "gzip"
		return parser.ParseStream(req.Body, isGzipped, func(rows []parser.Row) error {
			return insertRows(at, rows)
		})
	})
}

func insertRows(at *auth.Token, rows []parser.Row) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	rowsTotal := 0
	generationID := generateUniqueGenerationID()
	for i := range rows {
		r := &rows[i]
		rowsTotal += len(r.Values)
		ctx.Labels = ctx.Labels[:0]
		for j := range r.Tags {
			tag := &r.Tags[j]
			ctx.AddLabelBytes(tag.Key, tag.Value)
		}

		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		// there is no need in relabeling and extra_label adding
		// since modified series already passed this phase during ingestion,
		// and it may lead to unexpected result for user.
		ctx.AddLabelBytes([]byte(`__generation_id`), generationID)
		ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], at.AccountID, at.ProjectID, ctx.Labels)
		values := r.Values
		timestamps := r.Timestamps
		if len(timestamps) != len(values) {
			logger.Panicf("BUG: len(timestamps)=%d must match len(values)=%d", len(timestamps), len(values))
		}
		storageNodeIdx := ctx.GetStorageNodeIdx(at, ctx.Labels)
		for j, value := range values {
			timestamp := timestamps[j]
			if err := ctx.WriteDataPointExt(at, storageNodeIdx, ctx.MetricNameBuf, timestamp, value); err != nil {
				return err
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsTenantInserted.Get(at).Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
