package datadogutil

import (
	"flag"
	"regexp"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var (
	// MaxInsertRequestSize is the maximum request size is defined at https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
	MaxInsertRequestSize = flagutil.NewBytes("datadog.maxInsertRequestSize", 64*1024*1024, "The maximum size in bytes of a single DataDog POST request to /datadog/api/v2/series")

	// SanitizeMetricName controls sanitizing metric names ingested via DataDog protocols.
	//
	// If all metrics in Datadog have the same naming schema as custom metrics, then the following rules apply:
	// https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics
	// But there's some hidden behaviour. In addition to what it states in the docs, the following is also done:
	// - Consecutive underscores are replaced with just one underscore
	// - Underscore immediately before or after a dot are removed
	SanitizeMetricName = flag.Bool("datadog.sanitizeMetricName", true, "Sanitize metric names for the ingested DataDog data to comply with DataDog behaviour described at "+
		"https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics")
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

// SanitizeName performs DataDog-compatible sanitizing for metric names
//
// See https://docs.datadoghq.com/metrics/custom_metrics/#naming-custom-metrics
func SanitizeName(name string) string {
	return namesSanitizer.Transform(name)
}

var namesSanitizer = bytesutil.NewFastStringTransformer(func(s string) string {
	s = unsupportedDatadogChars.ReplaceAllLiteralString(s, "_")
	s = multiUnderscores.ReplaceAllLiteralString(s, "_")
	s = underscoresWithDots.ReplaceAllLiteralString(s, ".")
	return s
})

var (
	unsupportedDatadogChars = regexp.MustCompile(`[^0-9a-zA-Z_\.]+`)
	multiUnderscores        = regexp.MustCompile(`_+`)
	underscoresWithDots     = regexp.MustCompile(`_?\._?`)
)
