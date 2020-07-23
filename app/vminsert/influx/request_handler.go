package influx

import (
	"flag"
	"io"
	"net/http"
	"runtime"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	measurementFieldSeparator = flag.String("influxMeasurementFieldSeparator", "_", "Separator for '{measurement}{separator}{field_name}' metric name when inserted via Influx line protocol")
	skipSingleField           = flag.Bool("influxSkipSingleField", false, "Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metic name if Influx line contains only a single field")
	skipMeasurement           = flag.Bool("influxSkipMeasurement", false, "Uses '{field_name}' as a metric name while ignoring '{measurement}' and '-influxMeasurementFieldSeparator'")
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="influx"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="influx"}`)
)

// InsertHandlerForReader processes remote write for influx line protocol.
//
// See https://github.com/influxdata/telegraf/tree/master/plugins/inputs/socket_listener/
func InsertHandlerForReader(r io.Reader) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(r, false, "", "", insertRows)
	})
}

// InsertHandlerForHTTP processes remote write for influx line protocol.
//
// See https://github.com/influxdata/influxdb/blob/4cbdc197b8117fee648d62e2e5be75c6575352f0/tsdb/README.md
func InsertHandlerForHTTP(req *http.Request) error {
	return writeconcurrencylimiter.Do(func() error {
		isGzipped := req.Header.Get("Content-Encoding") == "gzip"
		q := req.URL.Query()
		precision := q.Get("precision")
		// Read db tag from https://docs.influxdata.com/influxdb/v1.7/tools/api/#write-http-endpoint
		db := q.Get("db")
		return parser.ParseStream(req.Body, isGzipped, precision, db, insertRows)
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
	hasRelabeling := relabel.HasRelabeling()
	for i := range rows {
		r := &rows[i]
		ic.Labels = ic.Labels[:0]
		for j := range r.Tags {
			tag := &r.Tags[j]
			if tag.Key == "db" {
				db = ""
			}
			ic.AddLabel(tag.Key, tag.Value)
		}
		ic.AddLabel("db", db)
		ctx.metricGroupBuf = ctx.metricGroupBuf[:0]
		if !*skipMeasurement {
			ctx.metricGroupBuf = append(ctx.metricGroupBuf, r.Measurement...)
		}
		skipFieldKey := len(r.Fields) == 1 && *skipSingleField
		if len(ctx.metricGroupBuf) > 0 && !skipFieldKey {
			ctx.metricGroupBuf = append(ctx.metricGroupBuf, *measurementFieldSeparator...)
		}
		metricGroupPrefixLen := len(ctx.metricGroupBuf)
		if hasRelabeling {
			ctx.originLabels = append(ctx.originLabels[:0], ic.Labels...)
			for j := range r.Fields {
				f := &r.Fields[j]
				if !skipFieldKey {
					ctx.metricGroupBuf = append(ctx.metricGroupBuf[:metricGroupPrefixLen], f.Key...)
				}
				metricGroup := bytesutil.ToUnsafeString(ctx.metricGroupBuf)
				ic.Labels = append(ic.Labels[:0], ctx.originLabels...)
				ic.AddLabel("", metricGroup)
				ic.ApplyRelabeling()
				if len(ic.Labels) == 0 {
					// Skip metric without labels.
					continue
				}
				ic.WriteDataPoint(nil, ic.Labels, r.Timestamp, f.Value)
			}
		} else {
			ctx.metricNameBuf = storage.MarshalMetricNameRaw(ctx.metricNameBuf[:0], ic.Labels)
			labelsLen := len(ic.Labels)
			for j := range r.Fields {
				f := &r.Fields[j]
				if !skipFieldKey {
					ctx.metricGroupBuf = append(ctx.metricGroupBuf[:metricGroupPrefixLen], f.Key...)
				}
				metricGroup := bytesutil.ToUnsafeString(ctx.metricGroupBuf)
				ic.Labels = ic.Labels[:labelsLen]
				ic.AddLabel("", metricGroup)
				if len(ic.Labels) == 0 {
					// Skip metric without labels.
					continue
				}
				ic.WriteDataPoint(ctx.metricNameBuf, ic.Labels[len(ic.Labels)-1:], r.Timestamp, f.Value)
			}
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
	originLabels   []prompb.Label
}

func (ctx *pushCtx) reset() {
	ctx.Common.Reset(0)
	ctx.metricNameBuf = ctx.metricNameBuf[:0]
	ctx.metricGroupBuf = ctx.metricGroupBuf[:0]

	originLabels := ctx.originLabels
	for i := range originLabels {
		label := &originLabels[i]
		label.Name = nil
		label.Value = nil
	}
	ctx.originLabels = ctx.originLabels[:0]
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
