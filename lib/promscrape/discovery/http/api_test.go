package http

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseAPIResponse(t *testing.T) {
	f := func(data, path string, resultExpected []httpGroupTarget) {
		t.Helper()

		result, err := parseAPIResponse([]byte(data), path)
		if err != nil {
			t.Fatalf("parseAPIResponse() error: %s", err)
		}
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	// parse ok
	data := `[
                {"targets": ["http://target-1:9100","http://target-2:9150"],
                "labels": {"label-1":"value-1"} }
                ]`
	path := "/ok"
	resultExpected := []httpGroupTarget{
		{
			Labels:  promutils.NewLabelsFromMap(map[string]string{"label-1": "value-1"}),
			Targets: []string{"http://target-1:9100", "http://target-2:9150"},
		},
	}
	f(data, path, resultExpected)
}
