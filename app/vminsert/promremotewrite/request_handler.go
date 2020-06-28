package promremotewrite

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted  = tenantmetrics.NewCounterMap(`vm_rows_inserted_total{type="promremotewrite"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="promremotewrite"}`)
)

// InsertHandler processes remote write for prometheus.
func InsertHandler(at *auth.Token, req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req, func(timeseries []prompb.TimeSeries) error {
			return insertRows(at, timeseries)
		})
	})
}

func insertRows(at *auth.Token, timeseries []prompb.TimeSeries) error {
	ctx := netstorage.GetInsertCtx()
	defer netstorage.PutInsertCtx(ctx)

	ctx.Reset() // This line is required for initializing ctx internals.
	rowsTotal := 0
	for i := range timeseries {
		ts := &timeseries[i]
		storageNodeGroupIDs := ctx.GetStorageNodeGroupIds(at, ts.Labels)
		ctx.MetricNameBuf = ctx.MetricNameBuf[:0]
		for i := range ts.Samples {
			r := &ts.Samples[i]
			if len(ctx.MetricNameBuf) == 0 {
				ctx.MetricNameBuf = storage.MarshalMetricNameRaw(ctx.MetricNameBuf[:0], at.AccountID, at.ProjectID, ts.Labels)
			}
			for _, storageNodeGroupID := range storageNodeGroupIDs {
				if err := ctx.WriteDataPointExt(at, storageNodeGroupID, ctx.MetricNameBuf, r.Timestamp, r.Value); err != nil {
					if ctx.AllReplicationGroupsFailed(storageNodeGroupID.Group) {
						logger.Errorf("All replication groups have failed")
						return err
					}
					logger.Errorf("Replciation group down: %s error: %s", storageNodeGroupID.Group, err)
				}
			}
		}
		rowsTotal += len(ts.Samples)
	}
	rowsInserted.Get(at).Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
