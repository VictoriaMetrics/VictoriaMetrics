package newrelic

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic"
)

// InsertHandlerForHTTP processes remote write for DataDog POST /api/v1/series request.
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func InsertHandlerForHTTP(req *http.Request) error {
	// extraLabels, err := parserCommon.GetExtraLabels(req)
	// if err != nil {
	// 	return err
	// }
	ce := req.Header.Get("Content-Encoding")
	return newrelic.Parse(req.Body, ce, func(series []newrelic.Series) error {
		return nil
	})
}
