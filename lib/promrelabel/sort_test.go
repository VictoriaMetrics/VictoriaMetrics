package promrelabel

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestSortLabels(t *testing.T) {
	labels := []prompbmarshal.Label{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "aa",
			Value: "bb",
		},
		{
			Name:  "ba",
			Value: "zz",
		},
	}
	labelsExpected := []prompbmarshal.Label{
		{
			Name:  "aa",
			Value: "bb",
		},
		{
			Name:  "ba",
			Value: "zz",
		},
		{
			Name:  "foo",
			Value: "bar",
		},
	}
	SortLabels(labels)
	if !reflect.DeepEqual(labels, labelsExpected) {
		t.Fatalf("unexpected sorted labels; got\n%v\nwant\n%v", labels, labelsExpected)
	}
}
