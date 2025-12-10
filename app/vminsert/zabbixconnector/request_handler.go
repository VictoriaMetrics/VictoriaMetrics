package zabbixconnector

import (
	"net/http"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/zabbixconnector"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/zabbixconnector/stream"
)

var (
	rowsInserted  = metrics.NewCounter(`vm_rows_inserted_total{type="zabbixconnector"}`)
	rowsPerInsert = metrics.NewHistogram(`vm_rows_per_insert{type="zabbixconnector"}`)
)

// InsertHandlerForHTTP processes remote write for ZabbixConnector POST /zabbixconnector/v1/history request.
func InsertHandlerForHTTP(req *http.Request) error {
	extraLabels, err := protoparserutil.GetExtraLabels(req)
	if err != nil {
		return err
	}
	encoding := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, encoding, func(rows []zabbixconnector.Row) error {
		return insertRows(rows, extraLabels)
	})
}

func insertRows(rows []zabbixconnector.Row, extraLabels []prompb.Label) error {
	ctx := common.GetInsertCtx()
	defer common.PutInsertCtx(ctx)

	rowsTotal := len(rows)
	ctx.Reset(rowsTotal)
	hasRelabeling := relabel.HasRelabeling()
	for i := range rows {
		r := &rows[i]
		ctx.Labels = ctx.Labels[:0]
		for k := range r.Tags {
			t := &r.Tags[k]
			ctx.AddLabelBytes(t.Key, t.Value)
		}
		for k := range extraLabels {
			label := &extraLabels[k]
			ctx.AddLabel(label.Name, label.Value)
		}
		if hasRelabeling {
			ctx.ApplyRelabeling()
		}
		if len(ctx.Labels) == 0 {
			// Skip metric without labels.
			continue
		}
		ctx.SortLabelsIfNeeded()
		if err := ctx.WriteDataPoint(nil, ctx.Labels, r.Timestamp, r.Value); err != nil {
			return err
		}
	}

	rowsInserted.Add(rowsTotal)
	rowsPerInsert.Update(float64(rowsTotal))
	return ctx.FlushBufs()
}
