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
			}},
			Tags: []string{
				"environment:test",
			},
		}},
	})
}
