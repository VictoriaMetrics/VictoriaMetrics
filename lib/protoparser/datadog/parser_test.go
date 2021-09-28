package datadog

import (
	"reflect"
	"testing"
)

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
