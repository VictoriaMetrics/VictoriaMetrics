package influx

import (
	"flag"
	"io"
	"net/http"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	measurementFieldSeparator = flag.String("influxMeasurementFieldSeparator", "_", "Separator for '{measurement}{separator}{field_name}' metric name when inserted via InfluxDB line protocol")
	skipSingleField           = flag.Bool("influxSkipSingleField", false, "Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metric name if InfluxDB line contains only a single field")
	skipMeasurement           = flag.Bool("influxSkipMeasurement", false, "Uses '{field_name}' as a metric name while ignoring '{measurement}' and '-influxMeasurementFieldSeparator'")
	dbLabel                   = flag.String("influxDBLabel", "db", "Default label for the DB name sent over '?db={db_name}' query parameter")
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="influx"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="influx"}`)
)

// InsertHandlerForReader processes remote write for influx line protocol.
//
// See https://github.com/influxdata/telegraf/tree/master/plugins/inputs/socket_listener/
func InsertHandlerForReader(r io.Reader) error {
	return stream.Parse(r, false, "", "", func(db string, rows []parser.Row) error {
		return insertRows(db, rows, nil)
	})
}

// InsertHandlerForHTTP processes remote write for influx line protocol.
//
// See https://github.com/influxdata/influxdb/blob/4cbdc197b8117fee648d62e2e5be75c6575352f0/tsdb/README.md
func InsertHandlerForHTTP(req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	isGzipped := req.Header.Get("Content-Encoding") == "gzip"
	q := req.URL.Query()
	precision := q.Get("precision")
	// Read db tag from https://docs.influxdata.com/influxdb/v1.7/tools/api/#write-http-endpoint
	db := q.Get("db")
	return stream.Parse(req.Body, isGzipped, precision, db, func(db string, rows []parser.Row) error {
		return insertRows(db, rows, extraLabels)
	})
}

func insertRows(db string, rows []parser.Row, extraLabels []prompbmarshal.Label) error {
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
		rowsTotal += len(r.Fields)
		ic.Labels = ic.Labels[:0]
		hasDBKey := false
		for j := range r.Tags {
			tag := &r.Tags[j]
			if tag.Key == *dbLabel {
				hasDBKey = true
			}
			ic.AddLabel(tag.Key, tag.Value)
		}
		if !hasDBKey {
			ic.AddLabel(*dbLabel, db)
		}
		for j := range extraLabels {
			label := &extraLabels[j]
			ic.AddLabel(label.Name, label.Value)
		}
		ctx.metricGroupBuf = ctx.metricGroupBuf[:0]
		if !*skipMeasurement {
			ctx.metricGroupBuf = append(ctx.metricGroupBuf, r.Measurement...)
		}
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1139
		skipFieldKey := len(r.Measurement) > 0 && len(r.Fields) == 1 && *skipSingleField
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
				ic.SortLabelsIfNeeded()
				if err := ic.WriteDataPoint(nil, ic.Labels, r.Timestamp, f.Value); err != nil {
					return err
				}
			}
		} else {
			ic.SortLabelsIfNeeded()
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
				if err := ic.WriteDataPoint(ctx.metricNameBuf, ic.Labels[len(ic.Labels)-1:], r.Timestamp, f.Value); err != nil {
					return err
				}
			}
		}
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
		originLabels[i] = prompb.Label{}
	}
	ctx.originLabels = originLabels[:0]
}

func getPushCtx() *pushCtx {
	if v := pushCtxPool.Get(); v != nil {
		return v.(*pushCtx)
	}
	return &pushCtx{}
}

func putPushCtx(ctx *pushCtx) {
	ctx.reset()
	pushCtxPool.Put(ctx)
}

var pushCtxPool sync.Pool
