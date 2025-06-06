package prometheus

import (
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

/*
	See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/8352#issuecomment-2682078886
	original https://github.com/VictoriaMetrics/VictoriaMetrics/blob/10abdf34ab082a61f40fd7df24a9df165f4f8b59/vendor/github.com/prometheus/common/model/metric.go#L63
*/

type EscapingScheme int

const (
	// NoEscaping indicates that a name will not be escaped. Unescaped names that
	// do not conform to the legacy validity check will use a new exposition
	// format syntax that will be officially standardized in future versions.
	NoEscaping EscapingScheme = iota

	// UnderscoreEscaping replaces all legacy-invalid characters with underscores.
	UnderscoreEscaping
)

const (
	// EscapingKey is the key in an Accept or Content-Type header that defines how
	// metric and label names that do not conform to the legacy character
	// requirements should be escaped when being scraped by a legacy prometheus
	// system. If a system does not explicitly pass an escaping parameter in the
	// Accept header, the default NameEscapingScheme will be used.
	EscapingKey = "escaping"

	// AllowUTF8 Possible values for Escaping Key:
	AllowUTF8 = "allow-utf-8" // No escaping required.
)

func ParseEscapingScheme(header http.Header) EscapingScheme {
	acceptValue := header.Get("Accept")
	if len(acceptValue) == 0 {
		return UnderscoreEscaping
	}

	parts := strings.Split(acceptValue, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			if key == EscapingKey {
				if value == AllowUTF8 {
					return NoEscaping
				}
				return UnderscoreEscaping
			}
		}
	}

	return UnderscoreEscaping
}

func isValidLegacyRune(b rune, i int) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_' || b == ':' || (b >= '0' && b <= '9' && i > 0)
}

func IsValidLegacyMetricName(n string) bool {
	if len(n) == 0 {
		return false
	}
	for i, b := range n {
		if !isValidLegacyRune(b, i) {
			return false
		}
	}
	return true
}

func EscapeName(name string, scheme EscapingScheme) string {
	if len(name) == 0 {
		return name
	}
	var escaped strings.Builder
	switch scheme {
	case NoEscaping:
		return name
	case UnderscoreEscaping:
		if IsValidLegacyMetricName(name) {
			return name
		}
		for i, b := range name {
			if isValidLegacyRune(b, i) {
				escaped.WriteRune(b)
			} else {
				escaped.WriteRune('_')
			}
		}
		return escaped.String()
	default:
		logger.Panicf("BUG: invalid escaping scheme %d", scheme)
		return ""
	}
}
