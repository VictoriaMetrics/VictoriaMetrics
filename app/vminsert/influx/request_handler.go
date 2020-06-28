package influx

import (
	"flag"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

var (
	measurementFieldSeparator = flag.String("influxMeasurementFieldSeparator", "_", "Separator for '{measurement}{separator}{field_name}' metric name when inserted via Influx line protocol")
	skipSingleField           = flag.Bool("influxSkipSingleField", false, "Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metic name if Influx line contains only a single field")
)

var (
	rowsInserted  = tenantmetrics.NewCounterMap(`vm_rows_inserted_total{type="influx"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="influx"}`)
)

// InsertHandlerForReader processes remote write for influx line protocol.
//
// See https://github.com/influxdata/telegraf/tree/master/plugins/inputs/socket_listener/
func InsertHandlerForReader(at *auth.Token, r io.Reader) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(r, false, "", "", func(db string, rows []parser.Row) error {
			return insertRows(at, db, rows, true)
		})
	})
}

// InsertHandlerForHTTP processes remote write for influx line protocol.
//
// See https://github.com/influxdata/influxdb/blob/4cbdc197b8117fee648d62e2e5be75c6575352f0/tsdb/README.md
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		isGzipped := req.Header.Get("Content-Encoding") == "gzip"
		q := req.URL.Query()
		precision := q.Get("precision")
		// Read db tag from https://docs.influxdata.com/influxdb/v1.7/tools/api/#write-http-endpoint
		db := q.Get("db")
		return parser.ParseStream(req.Body, isGzipped, precision, db, func(db string, rows []parser.Row) error {
			return insertRows(at, db, rows, false)
		})
	})
}

func insertRows(at *auth.Token, db string, rows []parser.Row, mayOverrideAccountProjectID bool) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)

	ic := &ctx.Common
	ic.Reset() // This line is required for initializing ic internals.
	rowsTotal := 0
	atCopy := *at
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		hasDBLabel := false
		for j := range r.Tags {
			tag := &r.Tags[j]
			if mayOverrideAccountProjectID {
				// Multi-tenancy support via custom tags.
				if tag.Key == "VictoriaMetrics_AccountID" {
					atCopy.AccountID = uint32(fastfloat.ParseUint64BestEffort(tag.Value))
				}
				if tag.Key == "VictoriaMetrics_ProjectID" {
					atCopy.ProjectID = uint32(fastfloat.ParseUint64BestEffort(tag.Value))
				}
			}
			if tag.Key == "db" {
				hasDBLabel = true
			}
			ic.AddLabel(tag.Key, tag.Value)
		}
		if len(db) > 0 && !hasDBLabel {
			ic.AddLabel("db", db)
		}
		ic.MetricNameBuf = storage.MarshalMetricNameRaw(ic.MetricNameBuf[:0], atCopy.AccountID, atCopy.ProjectID, ic.Labels)
		metricNameBufLen := len(ic.MetricNameBuf)
		ctx.metricGroupBuf = append(ctx.metricGroupBuf[:0], r.Measurement...)
		skipFieldKey := len(r.Fields) == 1 && *skipSingleField
		if len(ctx.metricGroupBuf) > 0 && !skipFieldKey {
			ctx.metricGroupBuf = append(ctx.metricGroupBuf, *measurementFieldSeparator...)
		}
		metricGroupPrefixLen := len(ctx.metricGroupBuf)
		ic.AddLabel("", "placeholder")
		placeholderLabel := &ic.Labels[len(ic.Labels)-1]
		for j := range r.Fields {
			f := &r.Fields[j]
			if !skipFieldKey {
				ctx.metricGroupBuf = append(ctx.metricGroupBuf[:metricGroupPrefixLen], f.Key...)
			}
			metricGroup := bytesutil.ToUnsafeString(ctx.metricGroupBuf)
			ic.Labels = ic.Labels[:len(ic.Labels)-1]
			ic.AddLabel("", metricGroup)
			ic.MetricNameBuf = storage.MarshalMetricLabelRaw(ic.MetricNameBuf[:metricNameBufLen], placeholderLabel)
			storageNodeGroupIDs := ic.GetStorageNodeGroupIds(&atCopy, ic.Labels)
			for _, storageNodeGroupID := range storageNodeGroupIDs {
				if err := ic.WriteDataPointExt(at, storageNodeGroupID, ic.MetricNameBuf, r.Timestamp, f.Value); err != nil {
					if ic.AllReplicationGroupsFailed(storageNodeGroupID.Group) {
						logger.Errorf("All replication groups have failed")
						return err
					}
					logger.Errorf("Replciation group down: %s error: %s", storageNodeGroupID.Group, err)
				}
			}
		}
		rowsTotal += len(r.Fields)
	}
	rowsInserted.Get(&atCopy).Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ic.FlushBufs()
}

type pushCtx struct {
	Common         netstorage.InsertCtx
	metricGroupBuf []byte
}

func (ctx *pushCtx) reset() {
	ctx.Common.Reset()
	ctx.metricGroupBuf = ctx.metricGroupBuf[:0]
}

func getPushCtx() *pushCtx {
	select {
	case ctx := <-pushCtxPoolCh:
		return ctx
	default:
		if v := pushCtxPool.Get(); v != nil {
			return v.(*pushCtx)
		}
		return &pushCtx{}
	}
}

func putPushCtx(ctx *pushCtx) {
	ctx.reset()
	select {
	case pushCtxPoolCh <- ctx:
	default:
		pushCtxPool.Put(ctx)
	}
}

var pushCtxPool sync.Pool
var pushCtxPoolCh = make(chan *pushCtx, runtime.GOMAXPROCS(-1))
