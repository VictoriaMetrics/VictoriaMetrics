package prometheus

import (
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestHeaderColumnName(t *testing.T) {
	f := func(fieldName, expected string) {
		t.Helper()
		got := headerColumnName(fieldName)
		if got != expected {
			t.Fatalf("headerColumnName(%q): got %q; want %q", fieldName, got, expected)
		}
	}

	f("__value__", "value")
	f("__timestamp__", "timestamp")
	f("__timestamp__:unix_s", "timestamp")
	f("__timestamp__:unix_ms", "timestamp")
	f("__timestamp__:unix_ns", "timestamp")
	f("__timestamp__:rfc3339", "timestamp")
	f("__timestamp__:custom:2006-01-02", "timestamp")
	f("__timestamp__:custom:15:04:05", "timestamp")

	// label names pass through
	f("__name__", "__name__")
	f("job", "job")
	f("instance", "instance")
	f("le", "le")
	f("quantile", "quantile")
}

func TestExportCSVHeader(t *testing.T) {
	f := func(fieldNames []string, expected string) {
		t.Helper()
		got := ExportCSVHeader(fieldNames)
		if got != expected {
			t.Fatalf("ExportCSVHeader(%v): got %q; want %q", fieldNames, got, expected)
		}
	}

	f(nil, "")
	f([]string{}, "")

	f([]string{"__value__"}, "value\n")
	f([]string{"__timestamp__"}, "timestamp\n")
	f([]string{"__timestamp__:rfc3339"}, "timestamp\n")
	f([]string{"__name__"}, "__name__\n")
	f([]string{"job"}, "job\n")

	f([]string{"__timestamp__:rfc3339", "__value__"}, "timestamp,value\n")
	f([]string{"__value__", "__timestamp__"}, "value,timestamp\n")
	f([]string{"job", "instance"}, "job,instance\n")

	f([]string{"__name__", "__value__", "__timestamp__:unix_s"}, "__name__,value,timestamp\n")
	f([]string{"job", "instance", "__value__", "__timestamp__:unix_ms"}, "job,instance,value,timestamp\n")
	f([]string{"__timestamp__:custom:2006-01-02", "__value__", "host", "dc", "env"},
		"timestamp,value,host,dc,env\n")

	// all timestamp formats produce the same header
	f([]string{"__timestamp__:unix_s", "__value__"}, "timestamp,value\n")
	f([]string{"__timestamp__:unix_ms", "__value__"}, "timestamp,value\n")
	f([]string{"__timestamp__:unix_ns", "__value__"}, "timestamp,value\n")
	f([]string{"__timestamp__:rfc3339", "__value__"}, "timestamp,value\n")
	f([]string{"__timestamp__:custom:Mon Jan 2", "__value__"}, "timestamp,value\n")

	// duplicate fields
	f([]string{"__value__", "__value__"}, "value,value\n")
	f([]string{"__timestamp__", "__timestamp__:rfc3339"}, "timestamp,timestamp\n")
}

func TestExportCSVLine(t *testing.T) {
	f := func(mn *storage.MetricName, timestamps []int64, values []float64, fieldNames []string, expected string) {
		t.Helper()
		xb := &exportBlock{
			mn:         mn,
			timestamps: timestamps,
			values:     values,
		}
		got := ExportCSVLine(xb, fieldNames)
		if got != expected {
			t.Fatalf("ExportCSVLine: got %q; want %q", got, expected)
		}
	}

	mn := &storage.MetricName{
		MetricGroup: []byte("cpu_usage"),
		Tags: []storage.Tag{
			{Key: []byte("job"), Value: []byte("node")},
			{Key: []byte("instance"), Value: []byte("localhost:9090")},
		},
	}

	// empty inputs
	f(mn, nil, nil, []string{"__value__"}, "")
	f(mn, []int64{}, []float64{}, []string{"__value__"}, "")
	f(mn, []int64{1000}, []float64{1.5}, nil, "")
	f(mn, []int64{1000}, []float64{1.5}, []string{}, "")

	f(mn, []int64{1000}, []float64{42.5}, []string{"__value__"}, "42.5\n")
	f(mn, []int64{1704067200000}, []float64{1}, []string{"__timestamp__"}, "1704067200000\n")
	f(mn, []int64{1704067200000}, []float64{1}, []string{"__timestamp__:unix_s"}, "1704067200\n")
	f(mn, []int64{1704067200000}, []float64{1}, []string{"__timestamp__:unix_ms"}, "1704067200000\n")
	f(mn, []int64{1704067200000}, []float64{1}, []string{"__timestamp__:unix_ns"}, "1704067200000000000\n")

	// rfc3339 sanity check
	{
		xb := &exportBlock{
			mn:         mn,
			timestamps: []int64{1704067200000},
			values:     []float64{1},
		}
		got := ExportCSVLine(xb, []string{"__timestamp__:rfc3339"})
		if len(got) < len("2006-01-02T15:04:05Z\n") {
			t.Fatalf("rfc3339 output too short: %q", got)
		}
		if got[len(got)-1] != '\n' {
			t.Fatalf("rfc3339 output missing trailing newline: %q", got)
		}
	}

	f(mn, []int64{1000}, []float64{1}, []string{"__name__"}, "cpu_usage\n")
	f(mn, []int64{1000}, []float64{1}, []string{"job"}, "node\n")
	f(mn, []int64{1000}, []float64{1}, []string{"instance"}, "localhost:9090\n")
	f(mn, []int64{1000}, []float64{1}, []string{"missing_label"}, "\n")

	// multiple fields
	f(mn, []int64{1704067200000}, []float64{99.9},
		[]string{"__timestamp__:unix_s", "__value__", "job"},
		"1704067200,99.9,node\n")

	// multiple rows
	f(mn, []int64{1000, 2000}, []float64{1.1, 2.2},
		[]string{"__value__", "__timestamp__"},
		"1.1,1000\n2.2,2000\n")
	f(mn, []int64{1000, 2000, 3000}, []float64{10, 20, 30},
		[]string{"__timestamp__:unix_s", "__value__"},
		"1,10\n2,20\n3,30\n")

	// escaping for special characters in tag values
	f(&storage.MetricName{
		MetricGroup: []byte("m"),
		Tags:        []storage.Tag{{Key: []byte("desc"), Value: []byte("a,b")}},
	}, []int64{1000}, []float64{1}, []string{"desc"}, "\"a,b\"\n")

	f(&storage.MetricName{
		MetricGroup: []byte("m"),
		Tags:        []storage.Tag{{Key: []byte("desc"), Value: []byte(`say "hello"`)}},
	}, []int64{1000}, []float64{1}, []string{"desc"}, "\"say \\\"hello\\\"\"\n")

	f(&storage.MetricName{
		MetricGroup: []byte("m"),
		Tags:        []storage.Tag{{Key: []byte("desc"), Value: []byte("line1\nline2")}},
	}, []int64{1000}, []float64{1}, []string{"desc"}, "\"line1\\nline2\"\n")

	// header and data line field counts must match
	fieldNames := []string{"__name__", "job", "instance", "__value__", "__timestamp__:unix_s"}
	header := ExportCSVHeader(fieldNames)
	line := ExportCSVLine(&exportBlock{
		mn:         mn,
		timestamps: []int64{1704067200000},
		values:     []float64{99.9},
	}, fieldNames)
	headerCommas := strings.Count(header, ",")
	lineCommas := strings.Count(line, ",")
	if headerCommas != lineCommas {
		t.Fatalf("header has %d commas, data line has %d commas", headerCommas, lineCommas)
	}
	if headerCommas != len(fieldNames)-1 {
		t.Fatalf("expected %d commas in header, got %d", len(fieldNames)-1, headerCommas)
	}
}
