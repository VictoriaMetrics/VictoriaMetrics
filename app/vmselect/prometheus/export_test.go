package prometheus

import (
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

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

	f([]string{"__value__"}, "__value__\n")
	f([]string{"__timestamp__"}, "__timestamp__\n")
	f([]string{"__timestamp__:rfc3339"}, "__timestamp__:rfc3339\n")
	f([]string{"__name__"}, "__name__\n")
	f([]string{"job"}, "job\n")

	f([]string{"__timestamp__:rfc3339", "__value__"}, "__timestamp__:rfc3339,__value__\n")
	f([]string{"__value__", "__timestamp__"}, "__value__,__timestamp__\n")
	f([]string{"job", "instance"}, "job,instance\n")

	f([]string{"__name__", "__value__", "__timestamp__:unix_s"}, "__name__,__value__,__timestamp__:unix_s\n")
	f([]string{"job", "instance", "__value__", "__timestamp__:unix_ms"}, "job,instance,__value__,__timestamp__:unix_ms\n")
	f([]string{"__timestamp__:custom:2006-01-02", "__value__", "host", "dc", "env"},
		"__timestamp__:custom:2006-01-02,__value__,host,dc,env\n")

	// duplicate fields
	f([]string{"__value__", "__value__"}, "__value__,__value__\n")
	f([]string{"__timestamp__", "__timestamp__:rfc3339"}, "__timestamp__,__timestamp__:rfc3339\n")
}

func TestExportCSVLine(t *testing.T) {
	localBak := time.Local
	time.Local = time.UTC
	defer func() { time.Local = localBak }()

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
	f(mn, []int64{1704067200000}, []float64{1}, []string{"__timestamp__:rfc3339"}, "2024-01-01T00:00:00Z\n")

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
