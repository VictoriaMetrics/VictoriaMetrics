package stream

import (
	"flag"
	"slices"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

var (
	usePrometheusNaming = flag.Bool("opentelemetry.usePrometheusNaming", false, "Whether to convert metric names and labels into Prometheus-compatible format for the metrics ingested "+
		"via OpenTelemetry protocol; see https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#sending-data-via-opentelemetry")
	convertMetricNamesToPrometheus = flag.Bool("opentelemetry.convertMetricNamesToPrometheus", false, "Whether to convert only metric names into Prometheus-compatible format for the metrics ingested "+
		"via OpenTelemetry protocol; see https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#sending-data-via-opentelemetry")
)

// unitMap is obtained from https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_name.go#L19
var unitMap = map[string]string{
	// Time
	"d":   "days",
	"h":   "hours",
	"min": "minutes",
	"s":   "seconds",
	"ms":  "milliseconds",
	"us":  "microseconds",
	"ns":  "nanoseconds",

	// Bytes
	"By":   "bytes",
	"KiBy": "kibibytes",
	"MiBy": "mebibytes",
	"GiBy": "gibibytes",
	"TiBy": "tibibytes",
	"KBy":  "kilobytes",
	"MBy":  "megabytes",
	"GBy":  "gigabytes",
	"TBy":  "terabytes",

	// SI
	"m": "meters",
	"V": "volts",
	"A": "amperes",
	"J": "joules",
	"W": "watts",
	"g": "grams",

	// Misc
	"Cel": "celsius",
	"Hz":  "hertz",
	"1":   "",
	"%":   "percent",
}

// perUnitMap is copied from https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_name.go#L58
var perUnitMap = map[string]string{
	"s":  "second",
	"m":  "minute",
	"h":  "hour",
	"d":  "day",
	"w":  "week",
	"mo": "month",
	"y":  "year",
}

type sanitizerContext struct {
	metricNameTokens []string
	metricNameBuf    []byte
	labelBuf         []byte
}

func (sctx *sanitizerContext) reset() {
	clear(sctx.metricNameTokens)
	sctx.metricNameTokens = sctx.metricNameTokens[:0]

	sctx.metricNameBuf = sctx.metricNameBuf[:0]
	sctx.labelBuf = sctx.labelBuf[:0]
}

// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_label.go#L26
//
// The returned string is valid until the next call to sanitizeLabelName.
func (sctx *sanitizerContext) sanitizeLabelName(labelName string) string {
	if !*usePrometheusNaming {
		return labelName
	}
	return sctx.sanitizePrometheusLabelName(labelName)
}

func (sctx *sanitizerContext) sanitizePrometheusLabelName(labelName string) string {
	if len(labelName) == 0 {
		return ""
	}
	labelName = promrelabel.SanitizeLabelName(labelName)
	if labelName[0] >= '0' && labelName[0] <= '9' {
		return sctx.concatLabel("key_", labelName)
	} else if strings.HasPrefix(labelName, "_") && !strings.HasPrefix(labelName, "__") {
		return sctx.concatLabel("key", labelName)
	}
	return labelName
}

func (sctx *sanitizerContext) concatLabel(a, b string) string {
	sctx.labelBuf = append(sctx.labelBuf[:0], a...)
	sctx.labelBuf = append(sctx.labelBuf, b...)
	return bytesutil.ToUnsafeString(sctx.labelBuf)
}

// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_name.go#L83
//
// The returned string is valid until the next call to sanitizeMetricName.
func (sctx *sanitizerContext) sanitizeMetricName(mm *pb.MetricMetadata) string {
	if !*usePrometheusNaming && !*convertMetricNamesToPrometheus {
		return mm.Name
	}
	return sctx.sanitizePrometheusMetricName(mm)
}

func (sctx *sanitizerContext) sanitizePrometheusMetricName(mm *pb.MetricMetadata) string {
	sctx.splitMetricNameToTokens(mm.Name)

	n := strings.IndexByte(mm.Unit, '/')

	mainUnit := mm.Unit
	perUnit := ""
	if n >= 0 {
		mainUnit = mm.Unit[:n]
		perUnit = mm.Unit[n+1:]
	}
	mainUnit = strings.TrimSpace(mainUnit)
	perUnit = strings.TrimSpace(perUnit)

	if mainUnit != "" && !strings.Contains(mainUnit, "{") {
		if u, ok := unitMap[mainUnit]; ok {
			mainUnit = u
		}
		if mainUnit != "" && !slices.Contains(sctx.metricNameTokens, mainUnit) {
			sctx.metricNameTokens = append(sctx.metricNameTokens, mainUnit)
		}
	}

	if perUnit != "" && !strings.Contains(perUnit, "{") {
		if u, ok := perUnitMap[perUnit]; ok {
			perUnit = u
		}
		if perUnit != "" && !slices.Contains(sctx.metricNameTokens, perUnit) {
			sctx.metricNameTokens = append(sctx.metricNameTokens, "per", perUnit)
		}
	}

	if mm.Type == prompb.MetricTypeCounter {
		sctx.metricNameTokens = moveOrAppend(sctx.metricNameTokens, "total")
	} else if mm.Unit == "1" && mm.Type == prompb.MetricTypeGauge {
		sctx.metricNameTokens = moveOrAppend(sctx.metricNameTokens, "ratio")
	}

	sctx.metricNameBuf = joinMetricNameTokens(sctx.metricNameBuf[:0], sctx.metricNameTokens)
	return bytesutil.ToUnsafeString(sctx.metricNameBuf)
}

func (sctx *sanitizerContext) splitMetricNameToTokens(metricName string) {
	clear(sctx.metricNameTokens)
	sctx.metricNameTokens = sctx.metricNameTokens[:0]

	s := metricName
	for len(s) > 0 {
		n := strings.IndexAny(s, "/_.-: ")
		if n < 0 {
			sctx.metricNameTokens = append(sctx.metricNameTokens, s)
			return
		}
		if n > 0 {
			sctx.metricNameTokens = append(sctx.metricNameTokens, s[:n])
		}
		s = s[n+1:]
	}
}

func joinMetricNameTokens(dst []byte, metricNameTokens []string) []byte {
	if len(metricNameTokens) == 0 {
		return dst
	}
	for _, token := range metricNameTokens {
		dst = append(dst, token...)
		dst = append(dst, '_')
	}
	return dst[:len(dst)-1]
}

func moveOrAppend(tokens []string, value string) []string {
	for i := range tokens {
		if tokens[i] == value {
			tokens = append(tokens[:i], tokens[i+1:]...)
			break
		}
	}
	return append(tokens, value)
}
