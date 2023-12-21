package datadogv1

import (
	"reflect"
	"testing"
)

func TestRequestUnmarshalMissingHost(t *testing.T) {
	// This tests https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3432
	req := Request{
		Series: []Series{{
			Host:   "prev-host",
			Device: "prev-device",
		}},
	}
	data := `
{
  "series": [
    {
      "metric": "system.load.1",
      "points": [[
        1575317847,
        0.5
      ]]
    }
  ]
}`
	if err := req.Unmarshal([]byte(data)); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	reqExpected := Request{
		Series: []Series{{
			Metric: "system.load.1",
			Points: []Point{{
				1575317847,
				0.5,
			}},
		}},
	}
	if !reflect.DeepEqual(&req, &reqExpected) {
		t.Fatalf("unexpected request parsed;\ngot\n%+v\nwant\n%+v", req, reqExpected)
	}
}

func TestRequestUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var req Request
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

func TestRequestUnmarshalSuccess(t *testing.T) {
	f := func(s string, reqExpected *Request) {
		t.Helper()
		var req Request
		if err := req.Unmarshal([]byte(s)); err != nil {
			t.Fatalf("unexpected error in Unmarshal(%q): %s", s, err)
		}
		if !reflect.DeepEqual(&req, reqExpected) {
			t.Fatalf("unexpected row;\ngot\n%+v\nwant\n%+v", &req, reqExpected)
		}
	}
	f("{}", &Request{})
	f(`
{
  "series": [
    {
      "host": "test.example.com",
      "interval": 20,
      "metric": "system.load.1",
      "device": "/dev/sda",
      "points": [[
        1575317847,
        0.5
      ]],
      "tags": [
        "environment:test"
      ],
      "type": "rate"
    }
  ]
}
`, &Request{
		Series: []Series{{
			Host:   "test.example.com",
			Metric: "system.load.1",
			Device: "/dev/sda",
			Points: []Point{{
				1575317847,
				0.5,
			}},
			Tags: []string{
				"environment:test",
			},
		}},
	})
}
