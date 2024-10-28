package stream

import (
	"bytes"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
)

func TestDetectTimestamp(t *testing.T) {
	tsDefault := int64(123)
	f := func(ts, tsExpected int64) {
		t.Helper()
		tsResult := detectTimestamp(ts, tsDefault)
		if tsResult != tsExpected {
			t.Fatalf("unexpected timestamp for detectTimestamp(%d, %d); got %d; want %d", ts, tsDefault, tsResult, tsExpected)
		}
	}
	f(0, tsDefault)
	f(1, 1e3)
	f(1e7, 1e10)
	f(1e8, 1e11)
	f(1e9, 1e12)
	f(1e10, 1e13)
	f(1e11, 1e11)
	f(1e12, 1e12)
	f(1e13, 1e13)
	f(1e14, 1e11)
	f(1e15, 1e12)
	f(1e16, 1e13)
	f(1e17, 1e11)
	f(1e18, 1e12)
}

func TestParseStream(t *testing.T) {
	common.StartUnmarshalWorkers()
	defer common.StopUnmarshalWorkers()

	f := func(data string, rowsExpected []influx.Row, isStreamMode bool, badData bool) {
		t.Helper()

		var wg sync.WaitGroup
		wg.Add(len(rowsExpected))
		buf := bytes.NewBuffer([]byte(data))

		var rowsMu sync.Mutex
		rows := make([]influx.Row, 0, len(rowsExpected))

		cb := func(_ string, rs []influx.Row) error {
			for _, r := range rs {
				rowsMu.Lock()
				rows = append(rows, influx.Row{
					Measurement: r.Measurement,
					Tags:        append(make([]influx.Tag, 0, len(r.Tags)), r.Tags...),
					Fields:      append(make([]influx.Field, 0, len(r.Fields)), r.Fields...),
					Timestamp:   r.Timestamp,
				})
				rowsMu.Unlock()
				wg.Done()
			}
			return nil
		}

		err := Parse(buf, isStreamMode, false, "ns", "test", cb)
		wg.Wait()

		if badData && !isStreamMode && err == nil {
			t.Fatalf("expected error on bad data in batch mode")
		}

		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Measurement < rows[j].Measurement
		})
		if !reflect.DeepEqual(rows, rowsExpected) {
			t.Fatalf("unexpected rows;\ngot\n%+v\nwant\n%+v", rows, rowsExpected)
		}
	}
	goodData := `foo1,location=us-midwest1 temperature=81 1727879909390000000
foo2,location=us-midwest2 temperature=82 1727879909390000000
foo3,location=us-midwest3 temperature=83 1727879909390000000`
	goodDataParsed := []influx.Row{
		{
			Measurement: "foo1",
			Tags:        []influx.Tag{{Key: "location", Value: "us-midwest1"}},
			Fields:      []influx.Field{{Key: "temperature", Value: 81}},
			Timestamp:   1727879909390,
		}, {
			Measurement: "foo2",
			Tags:        []influx.Tag{{Key: "location", Value: "us-midwest2"}},
			Fields:      []influx.Field{{Key: "temperature", Value: 82}},
			Timestamp:   1727879909390,
		}, {
			Measurement: "foo3",
			Tags:        []influx.Tag{{Key: "location", Value: "us-midwest3"}},
			Fields:      []influx.Field{{Key: "temperature", Value: 83}},
			Timestamp:   1727879909390,
		}}

	//batch mode
	f(goodData, goodDataParsed, false, false)
	//stream mode
	f(goodData, goodDataParsed, true, false)

	badData := `foo1,location=us-midwest1 temperature=81 1727879909390000000
foo2, ,location=us-midwest2 temperature=82 1727879909390000000
foo3,location=us-midwest3 temperature=83 1727879909390000000`
	badDataParsed := []influx.Row{{
		Measurement: "foo1",
		Tags:        []influx.Tag{{Key: "location", Value: "us-midwest1"}},
		Fields:      []influx.Field{{Key: "temperature", Value: 81}},
		Timestamp:   1727879909390,
	}, {
		Measurement: "foo3",
		Tags:        []influx.Tag{{Key: "location", Value: "us-midwest3"}},
		Fields:      []influx.Field{{Key: "temperature", Value: 83}},
		Timestamp:   1727879909390,
	}}

	// batch mode with errors
	f(badData, []influx.Row{}, false, true)
	// stream mode with errors
	f(badData, badDataParsed, true, false)
}
