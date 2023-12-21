package datadogv2

import (
	"reflect"
	"testing"
)

func TestRequestUnmarshalJSONFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var req Request
		if err := UnmarshalJSON(&req, []byte(s)); err == nil {
			t.Fatalf("expecting non-nil error for Unmarshal(%q)", s)
		}
	}
	f("")
	f("foobar")
	f(`{"series":123`)
	f(`1234`)
	f(`[]`)
}

func TestRequestUnmarshalJSONSuccess(t *testing.T) {
	f := func(s string, reqExpected *Request) {
		t.Helper()
		var req Request
		if err := UnmarshalJSON(&req, []byte(s)); err != nil {
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
      "metric": "system.load.1",
      "type": 0,
      "points": [
        {
          "timestamp": 1636629071,
          "value": 0.7
        }
      ],
      "resources": [
        {
          "name": "dummyhost",
          "type": "host"
        }
      ],
      "source_type_name": "kubernetes",
      "tags": ["environment:test"]
    }
  ]
}
`, &Request{
		Series: []Series{{
			Metric: "system.load.1",
			Points: []Point{
				{
					Timestamp: 1636629071,
					Value:     0.7,
				},
			},
			Resources: []Resource{
				{
					Name: "dummyhost",
					Type: "host",
				},
			},
			SourceTypeName: "kubernetes",
			Tags: []string{
				"environment:test",
			},
		}},
	})
}
