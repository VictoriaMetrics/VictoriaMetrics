package stream

import (
	"flag"
	"slices"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

var (
	usePrometheusNaming = flag.Bool("opentelemetry.usePrometheusNaming", false, "Whether to convert metric names and labels into Prometheus-compatible format for the metrics ingested "+
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

// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_label.go#L26
func sanitizeLabelName(labelName string) string {
	if !*usePrometheusNaming {
		return labelName
	}
	return sanitizePrometheusLabelName(labelName)
}

func sanitizePrometheusLabelName(labelName string) string {
	if len(labelName) == 0 {
		return ""
	}
	labelName = promrelabel.SanitizeLabelName(labelName)
	if labelName[0] >= '0' && labelName[0] <= '9' {
		return "key_" + labelName
	} else if strings.HasPrefix(labelName, "_") && !strings.HasPrefix(labelName, "__") {
		return "key" + labelName
	}
	return labelName
}

// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/b8655058501bed61a06bb660869051491f46840b/pkg/translator/prometheus/normalize_name.go#L83
func sanitizeMetricName(m *pb.Metric) string {
	if !*usePrometheusNaming {
		return m.Name
	}
	return sanitizePrometheusMetricName(m)
}

func sanitizePrometheusMetricName(m *pb.Metric) string {
	nameTokens := promrelabel.SplitMetricNameToTokens(m.Name)

	unitTokens := strings.SplitN(m.Unit, "/", 2)
	if len(unitTokens) > 0 {
		mainUnit := strings.TrimSpace(unitTokens[0])
		if mainUnit != "" && !strings.ContainsAny(mainUnit, "{}") {
			if u, ok := unitMap[mainUnit]; ok {
				mainUnit = u
			}
			if mainUnit != "" && !slices.Contains(nameTokens, mainUnit) {
				nameTokens = append(nameTokens, mainUnit)
			}
		}

		if len(unitTokens) > 1 {
			perUnit := strings.TrimSpace(unitTokens[1])
			if perUnit != "" && !strings.ContainsAny(perUnit, "{}") {
				if u, ok := perUnitMap[perUnit]; ok {
					perUnit = u
				}
				if perUnit != "" && !slices.Contains(nameTokens, perUnit) {
					nameTokens = append(nameTokens, "per", perUnit)
				}
			}
		}
	}

	if m.Sum != nil && m.Sum.IsMonotonic {
		nameTokens = moveOrAppend(nameTokens, "total")
	} else if m.Unit == "1" && m.Gauge != nil {
		nameTokens = moveOrAppend(nameTokens, "ratio")
	}
	return strings.Join(nameTokens, "_")
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
