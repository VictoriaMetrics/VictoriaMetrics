package datadog

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestRequestUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		req := new(Request)
		if err := req.Unmarshal([]byte(s)); err == nil {
			t.Fatalf("expecting non-nil error for Unmarshal(%q)", s)
		}
	}
	f("")
	f("foobar")
	f(`{"series":123`)
	f(`1234`)
	f(`[]`)
}

func TestRequestExtract(t *testing.T) {
	fn := func(s []byte, reqExpected *Request, samplesExp int) {
		t.Helper()
		req := new(Request)
		if err := req.Unmarshal(s); err != nil {
			t.Fatalf("unexpected error in Unmarshal(%q): %s", s, err)
		}
		if !reflect.DeepEqual(req, reqExpected) {
			t.Fatalf("unexpected row;\ngot\n%+v\nwant\n%+v", req, reqExpected)
		}

		var samplesTotal int
		cb := func(ts prompbmarshal.TimeSeries) error {
			samplesTotal += len(ts.Samples)
			return nil
		}
		sanitizeFn := func(name string) string {
			return name
		}
		if err := req.Extract(cb, sanitizeFn); err != nil {
			t.Fatalf("error when extracting data: %s", err)
		}

		if samplesTotal != samplesExp {
			t.Fatalf("expected to extract %d samples; got %d", samplesExp, samplesTotal)
		}

	}
	fn([]byte("{}"), new(Request), 0)

	fn([]byte(`
{
  "series": [
    {
      "interval": 20,
      "metric": "system.load.1",
			"resources": [
				{
					"name": "test.example.com",
					"type": "host"
				}, {
					"name": "/dev/sda",
					"type": "device"
				}
			],
			"points": [{
				"timestamp": 1575317847,
				"value": 0.5
			},{
				"timestamp": 1575317848,
				"value": 0.6
			}],
      "tags": [
        "environment:test"
      ],
      "type": "rate"
    }
  ]
}
`), &Request{
		Series: []series{{
			Metric: "system.load.1",
			Resources: []resource{{
				Name: "test.example.com",
				Type: "host",
			}, {
				Name: "/dev/sda",
				Type: "device",
			}},
			Points: []point{{
				Timestamp: 1575317847,
				Value:     0.5,
			}, {
				Timestamp: 1575317848,
				Value:     0.6,
			}},
			Tags: []string{
				"environment:test",
			},
		}},
	}, 2)
}
