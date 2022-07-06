package datadog

import (
	"reflect"
	"testing"
)

func TestSplitTag(t *testing.T) {
	f := func(s, nameExpected, valueExpected string) {
		t.Helper()
		name, value := SplitTag(s)
		if name != nameExpected {
			t.Fatalf("unexpected name obtained from %q; got %q; want %q", s, name, nameExpected)
		}
		if value != valueExpected {
			t.Fatalf("unexpected value obtained from %q; got %q; want %q", s, value, valueExpected)
		}
	}
	f("", "", "no_label_value")
	f("foo", "foo", "no_label_value")
	f("foo:bar", "foo", "bar")
	f(":bar", "", "bar")
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
