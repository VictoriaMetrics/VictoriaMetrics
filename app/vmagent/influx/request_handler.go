package influx

import (
	"flag"
	"io"
	"net/http"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	measurementFieldSeparator = flag.String("influxMeasurementFieldSeparator", "_", "Separator for '{measurement}{separator}{field_name}' metric name when inserted via InfluxDB line protocol")
	skipSingleField           = flag.Bool("influxSkipSingleField", false, "Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metric name if InfluxDB line contains only a single field")
	skipMeasurement           = flag.Bool("influxSkipMeasurement", false, "Uses '{field_name}' as a metric name while ignoring '{measurement}' and '-influxMeasurementFieldSeparator'")
	dbLabel                   = flag.String("influxDBLabel", "db", "Default label for the DB name sent over '?db={db_name}' query parameter")
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="influx"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="influx"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="influx"}`)
)

// InsertHandlerForReader processes remote write for influx line protocol.
//
// See https://github.com/influxdata/telegraf/tree/master/plugins/inputs/socket_listener/
func InsertHandlerForReader(at *auth.Token, r io.Reader, isGzipped bool) error {
	return stream.Parse(r, isGzipped, "", "", func(db string, rows []parser.Row) error {
		return insertRows(at, db, rows, nil)
	})
}

// InsertHandlerForHTTP processes remote write for influx line protocol.
//
// See https://github.com/influxdata/influxdb/blob/4cbdc197b8117fee648d62e2e5be75c6575352f0/tsdb/README.md
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
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
		return insertRows(at, db, rows, extraLabels)
	})
}

func insertRows(at *auth.Token, db string, rows []parser.Row, extraLabels []prompbmarshal.Label) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)

	rowsTotal := 0
	tssDst := ctx.ctx.WriteRequest.Timeseries[:0]
	labels := ctx.ctx.Labels[:0]
	samples := ctx.ctx.Samples[:0]
	commonLabels := ctx.commonLabels[:0]
	buf := ctx.buf[:0]
	for i := range rows {
		r := &rows[i]
		rowsTotal += len(r.Fields)
		commonLabels = commonLabels[:0]
		hasDBKey := false
		for j := range r.Tags {
			tag := &r.Tags[j]
			if tag.Key == *dbLabel {
				hasDBKey = true
			}
			commonLabels = append(commonLabels, prompbmarshal.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		if len(db) > 0 && !hasDBKey {
			commonLabels = append(commonLabels, prompbmarshal.Label{
				Name:  *dbLabel,
				Value: db,
			})
		}
		commonLabels = append(commonLabels, extraLabels...)
		ctx.metricGroupBuf = ctx.metricGroupBuf[:0]
		if !*skipMeasurement {
			ctx.metricGroupBuf = append(ctx.metricGroupBuf, r.Measurement...)
		}
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1139
		skipFieldKey := len(r.Measurement) > 0 && len(r.Fields) == 1 && *skipSingleField
		if len(ctx.metricGroupBuf) > 0 && !skipFieldKey {
			ctx.metricGroupBuf = append(ctx.metricGroupBuf, *measurementFieldSeparator...)
		}
		for j := range r.Fields {
			f := &r.Fields[j]
			bufLen := len(buf)
			buf = append(buf, ctx.metricGroupBuf...)
			if !skipFieldKey {
				buf = append(buf, f.Key...)
			}
			metricGroup := bytesutil.ToUnsafeString(buf[bufLen:])
			labelsLen := len(labels)
			labels = append(labels, prompbmarshal.Label{
				Name:  "__name__",
				Value: metricGroup,
			})
			labels = append(labels, commonLabels...)
			samples = append(samples, prompbmarshal.Sample{
				Timestamp: r.Timestamp,
				Value:     f.Value,
			})
			tssDst = append(tssDst, prompbmarshal.TimeSeries{
				Labels:  labels[labelsLen:],
				Samples: samples[len(samples)-1:],
			})
		}
	}
	ctx.buf = buf
	ctx.ctx.WriteRequest.Timeseries = tssDst
	ctx.ctx.Labels = labels
	ctx.ctx.Samples = samples
	ctx.commonLabels = commonLabels
	if !remotewrite.TryPush(at, &ctx.ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	rowsInserted.Add(rowsTotal)
	if at != nil {
		rowsTenantInserted.Get(at).Add(rowsTotal)
	}
	rowsPerInsert.Update(float64(rowsTotal))

	return nil
}

type pushCtx struct {
	ctx            common.PushCtx
	commonLabels   []prompbmarshal.Label
	metricGroupBuf []byte
	buf            []byte
}

func (ctx *pushCtx) reset() {
	ctx.ctx.Reset()

	promrelabel.CleanLabels(ctx.commonLabels)
	ctx.commonLabels = ctx.commonLabels[:0]

	ctx.metricGroupBuf = ctx.metricGroupBuf[:0]
	ctx.buf = ctx.buf[:0]
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
