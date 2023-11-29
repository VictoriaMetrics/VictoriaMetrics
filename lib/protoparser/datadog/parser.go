package datadog

import (
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// SplitTag splits DataDog tag into tag name and value.
//
// See https://docs.datadoghq.com/getting_started/tagging/#define-tags
func SplitTag(tag string) (string, string) {
	n := strings.IndexByte(tag, ':')
	if n < 0 {
		// No tag value.
		return tag, "no_label_value"
	}
	return tag[:n], tag[n+1:]
}

// Request represents DataDog submit metrics request
//
// See https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
type Request interface {
	Extract(func(prompbmarshal.TimeSeries) error, func(string) string) error
	Unmarshal([]byte) error
}
