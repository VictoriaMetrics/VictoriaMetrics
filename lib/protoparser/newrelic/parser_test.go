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
