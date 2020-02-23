package influx

import (
	"flag"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	measurementFieldSeparator = flag.String("influxMeasurementFieldSeparator", "_", "Separator for '{measurement}{separator}{field_name}' metric name when inserted via Influx line protocol")
	skipSingleField           = flag.Bool("influxSkipSingleField", false, "Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metic name if Influx line contains only a single field")
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="influx"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="influx"}`)
)

// InsertHandler processes remote write for influx line protocol.
//
// See https://github.com/influxdata/influxdb/blob/4cbdc197b8117fee648d62e2e5be75c6575352f0/tsdb/README.md
func InsertHandler(req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(req, insertRows)
	})
}

func insertRows(db string, rows []parser.Row) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)

	rowsLen := 0
	for i := range rows {
		rowsLen += len(rows[i].Fields)
	}
	ic := &ctx.Common
	ic.Reset(rowsLen)
	rowsTotal := 0
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		hasDBLabel := false
		for j := range r.Tags {
			tag := &r.Tags[j]
			if tag.Key == "db" {
				hasDBLabel = true
			}
			ic.AddLabel(tag.Key, tag.Value)
		}
		if len(db) > 0 && !hasDBLabel {
			ic.AddLabel("db", db)
		}
		ctx.metricNameBuf = storage.MarshalMetricNameRaw(ctx.metricNameBuf[:0], ic.Labels)
		ctx.metricGroupBuf = append(ctx.metricGroupBuf[:0], r.Measurement...)
		skipFieldKey := len(r.Fields) == 1 && *skipSingleField
		if len(ctx.metricGroupBuf) > 0 && !skipFieldKey {
			ctx.metricGroupBuf = append(ctx.metricGroupBuf, *measurementFieldSeparator...)
		}
		metricGroupPrefixLen := len(ctx.metricGroupBuf)
		for j := range r.Fields {
			f := &r.Fields[j]
			if !skipFieldKey {
				ctx.metricGroupBuf = append(ctx.metricGroupBuf[:metricGroupPrefixLen], f.Key...)
			}
			metricGroup := bytesutil.ToUnsafeString(ctx.metricGroupBuf)
			ic.Labels = ic.Labels[:0]
			ic.AddLabel("", metricGroup)
			ic.WriteDataPoint(ctx.metricNameBuf, ic.Labels[:1], r.Timestamp, f.Value)
		}
		rowsTotal += len(r.Fields)
	}
	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ic.FlushBufs()
}

type pushCtx struct {
	Common         common.InsertCtx
	metricNameBuf  []byte
	metricGroupBuf []byte
}

func (ctx *pushCtx) reset() {
	ctx.Common.Reset(0)
	ctx.metricNameBuf = ctx.metricNameBuf[:0]
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
