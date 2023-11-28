package datadog

import (
	"reflect"
	"testing"
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

func unmarshalRequestValidator(t *testing.T, s []byte, reqExpected *Request) {
	t.Helper()
	req := new(Request)
	if err := req.Unmarshal(s); err != nil {
		t.Fatalf("unexpected error in Unmarshal(%q): %s", s, err)
	}
	if !reflect.DeepEqual(req, reqExpected) {
		t.Fatalf("unexpected row;\ngot\n%+v\nwant\n%+v", req, reqExpected)
	}
}

func TestRequestUnmarshalSuccess(t *testing.T) {
	unmarshalRequestValidator(
		t, []byte("{}"), new(Request),
	)
	unmarshalRequestValidator(t, []byte(`
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
`), &Request{
		Series: []series{{
			Host:   "test.example.com",
			Metric: "system.load.1",
			Device: "/dev/sda",
			Points: []point{{
				1575317847,
				0.5,
			}},
			Tags: []string{
				"environment:test",
			},
		}},
	})
}
