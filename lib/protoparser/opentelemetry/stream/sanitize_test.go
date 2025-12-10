package stream

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func TestSanitizePrometheusLabelName(t *testing.T) {
	f := func(labelName, expectedResult string) {
		t.Helper()

		var sctx sanitizerContext
		result := sctx.sanitizePrometheusLabelName(labelName)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %q; want %q", result, expectedResult)
		}
	}

	f("", "")
	f("foo", "foo")
	f("foo_bar/baz:abc", "foo_bar_baz_abc")
	f("1foo", "key_1foo")
	f("_foo", "key_foo")
	f("__bar", "__bar")
}

func TestSanitizePrometheusMetricName(t *testing.T) {
	f := func(metricName, unit string, metricType prompb.MetricType, expectedResult string) {
		t.Helper()

		var sctx sanitizerContext
		mm := pb.MetricMetadata{
			Name: metricName,
			Unit: unit,
			Type: metricType,
		}
		result := sctx.sanitizePrometheusMetricName(&mm)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %q; want %q", result, expectedResult)
		}
	}

	f("", "", prompb.MetricTypeUnknown, "")
	f("foo", "", prompb.MetricTypeUnknown, "foo")
	f("foo", "s", prompb.MetricTypeUnknown, "foo_seconds")
	f("foo_seconds", "s", prompb.MetricTypeUnknown, "foo_seconds")
	f("foo", "", prompb.MetricTypeCounter, "foo_total")
	f("foo_total", "", prompb.MetricTypeCounter, "foo_total")
	f("foo", "s", prompb.MetricTypeCounter, "foo_seconds_total")
	f("foo_seconds", "s", prompb.MetricTypeCounter, "foo_seconds_total")
	f("foo_total", "s", prompb.MetricTypeCounter, "foo_seconds_total")
	f("foo_seconds_total", "s", prompb.MetricTypeCounter, "foo_seconds_total")
	f("foo_total_seconds", "s", prompb.MetricTypeCounter, "foo_seconds_total")
	f("foo", "1", prompb.MetricTypeGauge, "foo_ratio")
	f("foo", "m/s", prompb.MetricTypeUnknown, "foo_meters_per_second")
	f("foo_second", "m/s", prompb.MetricTypeUnknown, "foo_second_meters")
	f("foo_meters", "m/s", prompb.MetricTypeUnknown, "foo_meters_per_second")
}
