package pushgateway

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmagent/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	parserCommon "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	parser "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/tenantmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rowsInserted       = metrics.NewCounter(`vmagent_rows_inserted_total{type="pushgateway"}`)
	rowsTenantInserted = tenantmetrics.NewCounterMap(`vmagent_tenant_inserted_rows_total{type="pushgateway"}`)
	rowsPerInsert      = metrics.NewHistogram(`vmagent_rows_per_insert{type="pushgateway"}`)
)

// InsertHandler processes `/api/v1/pushgateway` request.
func InsertHandler(at *auth.Token, req *http.Request) error {
	// Pushgateway endpoint is of the style: /metrics/job/<JOB_NAME>{/<LABEL_NAME>/<LABEL_VALUE>}
	// Source:https://github.com/prometheus/pushgateway#url
	pushgatewayPath := strings.TrimSuffix(strings.Replace(req.URL.Path, "/api/v1/pushgateway", "", 1), "/")
	pathLabels, err := extractLabelsFromPath(pushgatewayPath)
	if err != nil {
		return err
	}
	extraLabels, err := parserCommon.GetExtraLabels(req)
	if err != nil {
		return err
	}
	defaultTimestamp, err := parserCommon.GetTimestamp(req)
	if err != nil {
		return err
	}
	return writeconcurrencylimiter.Do(func() error {
		isGzipped := req.Header.Get("Content-Encoding") == "gzip"
		return parser.ParseStream(req.Body, defaultTimestamp, isGzipped, func(rows []parser.Row) error {
			return insertRows(at, rows, append(pathLabels, extraLabels...))
		}, nil)
	})
}

func extractLabelsFromPath(pushgatewayPath string) ([]prompbmarshal.Label, error) {
	// Parsing Pushgateway path which is of the style: /metrics/job/<JOB_NAME>{/<LABEL_NAME>/<LABEL_VALUE>}
	// With an arbitrary number of /<LABEL_NAME>/<LABEL_VALUE> pairs
	// Source:https://github.com/prometheus/pushgateway#url
	var result []prompbmarshal.Label
	if !strings.HasPrefix(pushgatewayPath, "/metric/job/") {
		return nil, fmt.Errorf("pushgateway endpoint format is incorrect. Expected /metrics/job/<JOB_NAME>{/<LABEL_NAME>/<LABEL_VALUE>}, got %q ", pushgatewayPath)
	}
	labelsString := strings.Replace(pushgatewayPath, "/metric/job/", "", 1)
	labelsSlice := strings.Split(labelsString, "/")
	if len(labelsSlice) == 1 && labelsSlice[0] == "" {
		return nil, fmt.Errorf("pushgateway path has to contain a job name after /job/. Expected /metrics/job/<JOB_NAME>{/<LABEL_NAME>/<LABEL_VALUE>}, got %q ", pushgatewayPath)
	}

	//The first value that comes after /metrics/job/JOB_NAME gives origin to a label with key "job" and value "JOB_NAME"
	result = append(result, prompbmarshal.Label{
		Name:  "job",
		Value: labelsSlice[0],
	})

	// We expect the number of items to be odd.
	// The first item is the job label and after that is key/value pairs
	if len(labelsSlice)%2 == 0 {
		return nil, fmt.Errorf("number of label key/pair passed via pushgateway endpoint format does not match")
	}

	// We start at 1, since index 0 was the job label value, and we jump every 2 - first item is the key, second is the value.
	for i := 1; i < len(labelsSlice); i = i + 2 {
		result = append(result, prompbmarshal.Label{
			Name:  labelsSlice[i],
			Value: labelsSlice[i+1],
		})
	}
	return result, nil
}

func insertRows(at *auth.Token, rows []parser.Row, extraLabels []prompbmarshal.Label) error {
	ctx := common.GetPushCtx()
	defer common.PutPushCtx(ctx)

	tssDst := ctx.WriteRequest.Timeseries[:0]
	labels := ctx.Labels[:0]
	samples := ctx.Samples[:0]
	for i := range rows {
		r := &rows[i]
		labelsLen := len(labels)
		labels = append(labels, prompbmarshal.Label{
			Name:  "__name__",
			Value: r.Metric,
		})
		for j := range r.Tags {
			tag := &r.Tags[j]
			labels = append(labels, prompbmarshal.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		labels = append(labels, extraLabels...)
		samples = append(samples, prompbmarshal.Sample{
			Value:     r.Value,
			Timestamp: r.Timestamp,
		})
		tssDst = append(tssDst, prompbmarshal.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: samples[len(samples)-1:],
		})
	}
	ctx.WriteRequest.Timeseries = tssDst
	ctx.Labels = labels
	ctx.Samples = samples
	remotewrite.Push(at, &ctx.WriteRequest)
	rowsInserted.Add(len(rows))
	if at != nil {
		rowsTenantInserted.Get(at).Add(len(rows))
	}
	rowsPerInsert.Update(float64(len(rows)))
	return nil
}
