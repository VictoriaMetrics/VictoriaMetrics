package promrelabel

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

// MustParseMetricWithLabels parses s, which can have the form `metric{labels}`.
//
// This function is indended mostly for tests.
func MustParseMetricWithLabels(metricWithLabels string) []prompbmarshal.Label {
	stripDummyMetric := false
	if strings.HasPrefix(metricWithLabels, "{") {
		// Add a dummy metric name, since the parser needs it
		metricWithLabels = "dummy_metric" + metricWithLabels
		stripDummyMetric = true
	}
	// add a value to metricWithLabels, so it could be parsed by prometheus protocol parser.
	s := metricWithLabels + " 123"
	var rows prometheus.Rows
	var err error
	rows.UnmarshalWithErrLogger(s, func(s string) {
		err = fmt.Errorf("error during metric parse: %s", s)
	})
	if err != nil {
		logger.Panicf("BUG: cannot parse %q: %s", metricWithLabels, err)
	}
	if len(rows.Rows) != 1 {
		logger.Panicf("BUG: unexpected number of rows parsed; got %d; want 1", len(rows.Rows))
	}
	r := rows.Rows[0]
	var lfs []prompbmarshal.Label
	if !stripDummyMetric {
		lfs = append(lfs, prompbmarshal.Label{
			Name:  "__name__",
			Value: r.Metric,
		})
	}
	for _, tag := range r.Tags {
		lfs = append(lfs, prompbmarshal.Label{
			Name:  tag.Key,
			Value: tag.Value,
		})
	}
	return lfs
}
