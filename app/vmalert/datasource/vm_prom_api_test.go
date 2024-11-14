package datasource

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestPromInstant_UnmarshalPositive(t *testing.T) {
	f := func(data string, exp []Metric) {
		t.Helper()
		var pi promInstant
		err := pi.Unmarshal([]byte(data))
		if err != nil {
			t.Fatalf("unexpected unmarshal err %v; \n %v", err, string(data))
		}
		got, _ := pi.metrics()
		if !reflect.DeepEqual(got, exp) {
			t.Fatalf("expected to get:\n%v\ngot instead:\n%v", exp, got)
		}
	}

	f(`[{"metric":{"__name__":"up"},"value":[1583780000,"42"]}]`, []Metric{
		{
			Labels:     []prompbmarshal.Label{{Name: "__name__", Value: "up"}},
			Timestamps: []int64{1583780000},
			Values:     []float64{42},
		},
	})
	f(`[
{"metric":{"__name__":"up"},"value":[1583780000,"42"]},
{"metric":{"__name__":"foo"},"value":[1583780001,"7"]},
{"metric":{"__name__":"baz", "instance":"bar"},"value":[1583780002,"8"]}]`, []Metric{
		{
			Labels:     []prompbmarshal.Label{{Name: "__name__", Value: "up"}},
			Timestamps: []int64{1583780000},
			Values:     []float64{42},
		},
		{
			Labels:     []prompbmarshal.Label{{Name: "__name__", Value: "foo"}},
			Timestamps: []int64{1583780001},
			Values:     []float64{7},
		},
		{
			Labels:     []prompbmarshal.Label{{Name: "__name__", Value: "baz"}, {Name: "instance", Value: "bar"}},
			Timestamps: []int64{1583780002},
			Values:     []float64{8},
		},
	})
}

func TestPromInstant_UnmarshalNegative(t *testing.T) {
	f := func(data string) {
		t.Helper()
		var pi promInstant
		err := pi.Unmarshal([]byte(data))
		if err == nil {
			t.Fatalf("expected to get an error; got nil instead")
		}
	}
	f(``)
	f(`foo`)
	f(`[{"metric":{"__name__":"up"},"value":[1583780000,"42"]},`)
	f(`[{"metric":{"__name__"},"value":[1583780000,"42"]},`)
	// no `metric` object
	f(`[{"value":[1583780000,"42"]}]`)
	// no `value` object
	f(`[{"metric":{"__name__":"up"}}]`)
	// less than 2 values in `value` object
	f(`[{"metric":{"__name__":"up"},"value":["42"]}]`)
	f(`[{"metric":{"__name__":"up"},"value":[1583780000]}]`)
	// non-numeric sample value
	f(`[{"metric":{"__name__":"up"},"value":[1583780000,"foo"]}]`)
}
