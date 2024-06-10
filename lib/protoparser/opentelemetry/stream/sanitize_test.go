package stream

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func TestSanitizePrometheusLabelName(t *testing.T) {
	f := func(labelName, expectedResult string) {
		t.Helper()

		result := sanitizePrometheusLabelName(labelName)
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
	f := func(m *pb.Metric, expectedResult string) {
		t.Helper()

		result := sanitizePrometheusMetricName(m)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %q; want %q", result, expectedResult)
		}
	}

	f(&pb.Metric{}, "")

	f(&pb.Metric{
		Name: "foo",
	}, "foo")

	f(&pb.Metric{
		Name: "foo",
		Unit: "s",
	}, "foo_seconds")

	f(&pb.Metric{
		Name: "foo_seconds",
		Unit: "s",
	}, "foo_seconds")

	f(&pb.Metric{
		Name: "foo",
		Sum: &pb.Sum{
			IsMonotonic: true,
		},
	}, "foo_total")

	f(&pb.Metric{
		Name: "foo_total",
		Sum: &pb.Sum{
			IsMonotonic: true,
		},
	}, "foo_total")

	f(&pb.Metric{
		Name: "foo",
		Sum: &pb.Sum{
			IsMonotonic: true,
		},
		Unit: "s",
	}, "foo_seconds_total")

	f(&pb.Metric{
		Name: "foo_seconds",
		Sum: &pb.Sum{
			IsMonotonic: true,
		},
		Unit: "s",
	}, "foo_seconds_total")

	f(&pb.Metric{
		Name: "foo_total",
		Sum: &pb.Sum{
			IsMonotonic: true,
		},
		Unit: "s",
	}, "foo_seconds_total")

	f(&pb.Metric{
		Name: "foo_seconds_total",
		Sum: &pb.Sum{
			IsMonotonic: true,
		},
		Unit: "s",
	}, "foo_seconds_total")

	f(&pb.Metric{
		Name: "foo_total_seconds",
		Sum: &pb.Sum{
			IsMonotonic: true,
		},
		Unit: "s",
	}, "foo_seconds_total")

	f(&pb.Metric{
		Name:  "foo",
		Gauge: &pb.Gauge{},
		Unit:  "1",
	}, "foo_ratio")

	f(&pb.Metric{
		Name: "foo",
		Unit: "m/s",
	}, "foo_meters_per_second")

	f(&pb.Metric{
		Name: "foo_second",
		Unit: "m/s",
	}, "foo_second_meters")

	f(&pb.Metric{
		Name: "foo_meters",
		Unit: "m/s",
	}, "foo_meters_per_second")
}
