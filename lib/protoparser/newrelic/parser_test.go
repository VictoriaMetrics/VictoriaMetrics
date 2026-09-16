package newrelic

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		var r Rows
		if err := r.Unmarshal([]byte(data)); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// Empty JSON
	f("")

	// Invalid JSON
	f("123")
	f("[foo]")
	f(`{"foo":123}`)
}

func TestRowsUnmarshalWithCallback(t *testing.T) {
	request := `[{"Events":[
		{"tag":"12345"},
		{"tag":"67890"},
		{"tag":"abcde"}
	]}]`

	var r Rows
	var batches []string
	callback := func(rows []Row) error {
		batches = append(batches, rowsToString(rows))
		return nil
	}
	rowSize := (&Row{
		Tags: []Tag{{
			Key:   []byte("tag"),
			Value: []byte("12345"),
		}},
	}).sizeBytes()
	if err := r.unmarshal([]byte(request), rowSize, callback); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedBatches := []string{
		"tags={tag=\"12345\"}, samples=, timestamp=0\ntags={tag=\"67890\"}, samples=, timestamp=0",
		"tags={tag=\"abcde\"}, samples=, timestamp=0",
	}
	if !reflect.DeepEqual(batches, expectedBatches) {
		t.Fatalf("unexpected batches\ngot\n%q\nwant\n%q", batches, expectedBatches)
	}
}

func TestRowsUnmarshalWithCallbackCountsRowOverhead(t *testing.T) {
	request := `[{"Events":[{"a":1},{"a":1},{"a":1},{"a":1},{"a":1}]}]`
	rowSize := (&Row{
		Samples: []Sample{{
			Name: []byte("a"),
		}},
	}).sizeBytes()

	var r Rows
	var batchSizes []int
	if err := r.unmarshal([]byte(request), 2*rowSize, func(rows []Row) error {
		batchSizes = append(batchSizes, len(rows))
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expectedBatchSizes := []int{3, 2}
	if !reflect.DeepEqual(batchSizes, expectedBatchSizes) {
		t.Fatalf("unexpected batch sizes; got %v; want %v", batchSizes, expectedBatchSizes)
	}
}

func TestRowsUnmarshalWithCallbackEmptyRequest(t *testing.T) {
	var r Rows
	callbacks := 0
	if err := r.UnmarshalWithCallback([]byte("[]"), func(rows []Row) error {
		callbacks++
		if len(rows) != 0 {
			t.Fatalf("unexpected non-empty rows: %v", rows)
		}
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if callbacks != 1 {
		t.Fatalf("unexpected number of callback calls; got %d; want 1", callbacks)
	}
}

func TestRowsUnmarshalWithCallbackError(t *testing.T) {
	errCallback := fmt.Errorf("callback error")
	request := `[{"Events":[{"tag":"12345"},{"tag":"67890"},{"tag":"abcde"}]}]`

	var r Rows
	callbacks := 0
	err := r.unmarshal([]byte(request), 1, func(_ []Row) error {
		callbacks++
		return errCallback
	})
	if err != errCallback {
		t.Fatalf("unexpected error; got %v; want %v", err, errCallback)
	}
	if callbacks != 1 {
		t.Fatalf("unexpected number of callback calls; got %d; want 1", callbacks)
	}
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(data string, expectedRows []Row) {
		t.Helper()

		var r Rows
		if err := r.Unmarshal([]byte(data)); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(r.Rows, expectedRows) {
			t.Fatalf("unexpected rows parsed\ngot\n%s\nwant\n%s", rowsToString(r.Rows), rowsToString(expectedRows))
		}
	}

	// empty array
	f(`[]`, nil)

	// zero events
	f(`[
    {
      "EntityID":28257883748326179,
      "IsAgent":true,
      "Events":[],
      "ReportingAgentID":28257883748326179
    }]`, nil)

	// A single event
	f(`[{
      "EntityID":28257883748326179,
      "IsAgent":true,
      "Events":[
        {
          "eventType":"SystemSample",
          "timestamp":1690286061,
          "entityKey":"macbook-pro.local",
          "dc": "1",
          "diskWritesPerSecond":-34.21,
          "uptime":762376
        }
      ],
      "ReportingAgentID":28257883748326179
    }]`, []Row{
		{
			Tags: []Tag{
				{
					Key:   []byte("eventType"),
					Value: []byte("SystemSample"),
				},
				{
					Key:   []byte("entityKey"),
					Value: []byte("macbook-pro.local"),
				},
				{
					Key:   []byte("dc"),
					Value: []byte("1"),
				},
			},
			Samples: []Sample{
				{
					Name:  []byte("diskWritesPerSecond"),
					Value: -34.21,
				},
				{
					Name:  []byte("uptime"),
					Value: 762376,
				},
			},
			Timestamp: 1690286061000,
		},
	})

	// Multiple events
	f(`[
    {
      "EntityID":28257883748326179,
      "IsAgent":true,
      "Events":[
        {
          "eventType":"SystemSample",
          "timestamp":1690286061,
          "entityKey":"macbook-pro.local",
          "dc": "1",
          "diskWritesPerSecond":-34.21,
          "uptime":762376
        }
      ],
      "ReportingAgentID":28257883748326179
    },
    {
      "EntityID":282579,
      "IsAgent":true,
      "Events":[
        {
          "eventType":"SystemSample",
          "timestamp":1690286061,
          "entityKey":"macbook-pro.local",
	  "diskWritesPerSecond":234.34,
          "timestamp":1690286061.433,
          "uptime":762376
        },
        {
          "eventType":"ProcessSample",
          "timestamp":1690286061987,
          "uptime":1236
        }
      ],
      "ReportingAgentID":2879
    }
    ]`, []Row{
		{
			Tags: []Tag{
				{
					Key:   []byte("eventType"),
					Value: []byte("SystemSample"),
				},
				{
					Key:   []byte("entityKey"),
					Value: []byte("macbook-pro.local"),
				},
				{
					Key:   []byte("dc"),
					Value: []byte("1"),
				},
			},
			Samples: []Sample{
				{
					Name:  []byte("diskWritesPerSecond"),
					Value: -34.21,
				},
				{
					Name:  []byte("uptime"),
					Value: 762376,
				},
			},
			Timestamp: 1690286061000,
		},
		{
			Tags: []Tag{
				{
					Key:   []byte("eventType"),
					Value: []byte("SystemSample"),
				},
				{
					Key:   []byte("entityKey"),
					Value: []byte("macbook-pro.local"),
				},
			},
			Samples: []Sample{
				{
					Name:  []byte("diskWritesPerSecond"),
					Value: 234.34,
				},
				{
					Name:  []byte("uptime"),
					Value: 762376,
				},
			},
			Timestamp: 1690286061433,
		},
		{
			Tags: []Tag{
				{
					Key:   []byte("eventType"),
					Value: []byte("ProcessSample"),
				},
			},
			Samples: []Sample{
				{
					Name:  []byte("uptime"),
					Value: 1236,
				},
			},
			Timestamp: 1690286061987,
		},
	})

}

func rowsToString(rows []Row) string {
	var a []string
	for _, row := range rows {
		s := row.String()
		a = append(a, s)
	}
	return strings.Join(a, "\n")
}

func (r *Row) String() string {
	var a []string
	for _, t := range r.Tags {
		s := fmt.Sprintf("%s=%q", t.Key, t.Value)
		a = append(a, s)
	}
	tagsString := "{" + strings.Join(a, ",") + "}"
	a = a[:0]
	for _, sample := range r.Samples {
		s := fmt.Sprintf("[%s %f]", sample.Name, sample.Value)
		a = append(a, s)
	}
	samplesString := strings.Join(a, ",")
	return fmt.Sprintf("tags=%s, samples=%s, timestamp=%d", tagsString, samplesString, r.Timestamp)
}
