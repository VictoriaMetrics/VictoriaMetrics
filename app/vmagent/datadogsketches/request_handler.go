package datadogsketches

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="datadogsketches"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="datadogsketches"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="datadogsketches"}`)
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/beta/sketches request.
func InsertHandlerForHTTP(at *auth.Token, req *http.Request) error {
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	ce := req.Header.Get("Content-Encoding")
	return stream.Parse(req.Body, ce, func(sketches []*datadogsketches.Sketch) error {
		return insertRows(at, sketches, extraLabels)
	})
}

func insertRows(at *auth.Token, sketches []*datadogsketches.Sketch, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	rowsTotal := 0
	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range sketches {
		sketch := sketches[i]
		metrics := sketch.ToHistogram()
		rowsTotal += sketch.RowsCount()
		for m := range metrics {
			metric := metrics[m]
			labelsLen := len(labels)
			labels = append(labels, prompbmarshal.Label{
				Name:  "__name__",
				Value: metric.Name,
			})
			for l := range metric.Labels {
				label := metric.Labels[l]
				labels = append(labels, prompbmarshal.Label{
					Name:  label.Name,
					Value: label.Value,
				})
			}
			for _, tag := range sketch.Tags {
				name, value := datadogutils.SplitTag(tag)
				if name == "host" {
					name = "exported_host"
				}
				labels = append(labels, prompbmarshal.Label{
					Name:  name,
					Value: value,
				})
			}
			labels = append(labels, extraLabels...)
			samplesLen := len(samples)
			for p := range metric.Points {
				point := metric.Points[p]
				samples = append(samples, prompbmarshal.Sample{
					Timestamp: sketch.Dogsketches[p].Ts * 1000,
					Value:     point,
				})
			}
			tssDst = append(tssDst, prompbmarshal.TimeSeries{
				Labels:  labels[labelsLen:],
				Samples: samples[samplesLen:],
			})
		}
	}
	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples
	if !remotewrite.TryPush(at, &ctx.WriteRequest) {
		return remotewrite.ErrQueueFullHTTPRetry
	}
	rowsInserted.Add(rowsTotal)
	if at != nil {
		rowsTenantInserted.Get(at).Add(rowsTotal)
	}
	rowsPerInsert.Update(float64(rowsTotal))
	return nil
}
