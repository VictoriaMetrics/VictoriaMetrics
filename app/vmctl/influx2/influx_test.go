package influx2

import (
	"testing"
)

func TestExtractTags(t *testing.T) {
	f := func(input map[string]interface{}, wantNames []string) {
		t.Helper()
		tags := extractTags(input)
		got := make(map[string]bool)
		for _, lp := range tags {
			got[lp.Name] = true
		}
		for _, name := range wantNames {
			if !got[name] {
				t.Fatalf("expected tag %q in result, got %v", name, tags)
			}
		}
		// Make sure no system columns leaked through.
		systemCols := []string{"_time", "_value", "_field", "_measurement", "_start", "_stop", "result", "table"}
		for _, sc := range systemCols {
			if got[sc] {
				t.Fatalf("system column %q should not appear in extracted tags", sc)
			}
		}
	}

	// Only system columns — result should be empty.
	f(map[string]interface{}{
		"_time":        "2021-01-01T00:00:00Z",
		"_value":       83.7,
		"_field":       "usage_idle",
		"_measurement": "cpu",
		"_start":       "...",
		"_stop":        "...",
		"result":       "_result",
		"table":        0,
	}, nil)

	// Mix of system columns and real tags — only real tags should come through.
	f(map[string]interface{}{
		"_time":        "2021-01-01T00:00:00Z",
		"_value":       83.7,
		"_field":       "usage_idle",
		"_measurement": "cpu",
		"host":         "server01",
		"region":       "us-east",
	}, []string{"host", "region"})
}

func TestTimeRange_Defaults(t *testing.T) {
	c := &Client{filter: Filter{}}
	start, stop := c.timeRange()

	// Default start should be InfluxDB's minimum timestamp.
	wantStart := "1677-09-21T00:12:43.145224194Z"
	if start != wantStart {
		t.Fatalf("got start %q; want %q", start, wantStart)
	}
	// Default stop should be the Flux now() function.
	if stop != "now()" {
		t.Fatalf("got stop %q; want %q", stop, "now()")
	}
}

func TestTimeRange_WithFilter(t *testing.T) {
	c := &Client{filter: Filter{
		TimeStart: "2021-01-01T00:00:00Z",
		TimeEnd:   "2022-01-01T00:00:00Z",
	}}
	start, stop := c.timeRange()

	if start != "2021-01-01T00:00:00Z" {
		t.Fatalf("got start %q; want %q", start, "2021-01-01T00:00:00Z")
	}
	if stop != "2022-01-01T00:00:00Z" {
		t.Fatalf("got stop %q; want %q", stop, "2022-01-01T00:00:00Z")
	}
}
