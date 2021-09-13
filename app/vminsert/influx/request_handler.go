package influx

import (
	"flag"
	"io"
	"net/http"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
)

var (
	measurementFieldSeparator = flag.String("influxMeasurementFieldSeparator", "_", "Separator for '{measurement}{separator}{field_name}' metric name when inserted via InfluxDB line protocol")
	skipSingleField           = flag.Bool("influxSkipSingleField", false, "Uses '{measurement}' instead of '{measurement}{separator}{field_name}' for metic name if InfluxDB line contains only a single field")
	skipMeasurement           = flag.Bool("influxSkipMeasurement", false, "Uses '{field_name}' as a metric name while ignoring '{measurement}' and '-influxMeasurementFieldSeparator'")
)

var (
	rowsInserted       = metrics.NewCounter(`vm_rows_inserted_total{type="influx"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vm_tenant_inserted_rows_total{type="influx"}`)
	rowsPerInsert      = metrics.NewHistogram(`vm_rows_per_insert{type="influx"}`)
)

// InsertHandlerForReader processes remote write for influx line protocol.
//
// See https://github.com/influxdata/telegraf/tree/master/plugins/inputs/socket_listener/
func InsertHandlerForReader(at *auth.Token, r io.Reader) error {
	return writeconcurrencylimiter.Do(func() error {
		return parser.ParseStream(r, false, "", "", func(db string, rows []parser.Row) error {
			return insertRows(at, db, rows, nil, true)
		})
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
	return writeconcurrencylimiter.Do(func() error {
		isGzipped := req.Header.Get("Content-Encoding") == "gzip"
		q := req.URL.Query()
		precision := q.Get("precision")
		// Read db tag from https://docs.influxdata.com/influxdb/v1.7/tools/api/#write-http-endpoint
		db := q.Get("db")
		return parser.ParseStream(req.Body, isGzipped, precision, db, func(db string, rows []parser.Row) error {
			return insertRows(at, db, rows, extraLabels, false)
		})
	})
}

func insertRows(at *auth.Token, db string, rows []parser.Row, extraLabels []prompbmarshal.Label, mayOverrideAccountProjectID bool) error {
	ctx := getPushCtx()
	defer putPushCtx(ctx)

	ic := &ctx.Common
	ic.Reset() // This line is required for initializing ic internals.
	rowsTotal := 0
	atCopy := *at
	hasRelabeling := relabel.HasRelabeling()
	for i := range rows {
		r := &rows[i]
		rowsTotal += len(r.Fields)
		ic.Labels = ic.Labels[:0]
		hasDBKey := false
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
				hasDBKey = true
			}
			ic.AddLabel(tag.Key, tag.Value)
		}
		if !hasDBKey {
			ic.AddLabel("db", db)
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
			ic.MetricNameBuf = storage.MarshalMetricNameRaw(ic.MetricNameBuf[:0], atCopy.AccountID, atCopy.ProjectID, nil)
			metricNameBufLen := len(ic.MetricNameBuf)
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
				ic.MetricNameBuf = ic.MetricNameBuf[:metricNameBufLen]
				ic.SortLabelsIfNeeded()
				for i := range ic.Labels {
					ic.MetricNameBuf = storage.MarshalMetricLabelRaw(ic.MetricNameBuf, &ic.Labels[i])
				}
				storageNodeIdx := ic.GetStorageNodeIdx(&atCopy, ic.Labels)
				if err := ic.WriteDataPointExt(&atCopy, storageNodeIdx, ic.MetricNameBuf, r.Timestamp, f.Value); err != nil {
					return err
				}
			}
		} else {
			ic.SortLabelsIfNeeded()
			ic.MetricNameBuf = storage.MarshalMetricNameRaw(ic.MetricNameBuf[:0], atCopy.AccountID, atCopy.ProjectID, ic.Labels)
			metricNameBufLen := len(ic.MetricNameBuf)
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
				ic.MetricNameBuf = ic.MetricNameBuf[:metricNameBufLen]
				ic.MetricNameBuf = storage.MarshalMetricLabelRaw(ic.MetricNameBuf, &ic.Labels[len(ic.Labels)-1])
				storageNodeIdx := ic.GetStorageNodeIdx(&atCopy, ic.Labels)
				if err := ic.WriteDataPointExt(&atCopy, storageNodeIdx, ic.MetricNameBuf, r.Timestamp, f.Value); err != nil {
					return err
				}
			}
		}
	}
	rowsInserted.Add(rowsTotal)
	rowsTenantInserted.Get(&atCopy).Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ic.FlushBufs()
}

type pushCtx struct {
	Common         netstorage.InsertCtx
	metricGroupBuf []byte
	originLabels   []prompb.Label
}

func (ctx *pushCtx) reset() {
	ctx.Common.Reset()
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
var pushCtxPoolCh = make(chan *pushCtx, cgroup.AvailableCPUs())
