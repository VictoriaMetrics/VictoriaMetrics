package streamaggr

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"reflect"
	"testing"
)

func TestCompress(t *testing.T) {
	bm := &bimap{}
	fn := func(metrics, series int) {
		labels := make([]prompbmarshal.Label, 0)
		for i := 0; i < metrics; i++ {
			for j := 0; j < series; j++ {
				l := prompbmarshal.Label{
					Name:  fmt.Sprintf("key%d", i),
					Value: fmt.Sprintf("value%d", j),
				}
				labels = append(labels, l)
			}
		}
		bb := make([]byte, 0)
		s := string(bm.compress(bb, labels))
		fmt.Println("-------- cardinality", metrics*series)
		newK, regK := len(s), len(marshalLabelsFast(nil, labels))
		fmt.Println("compressed key len", newK)
		fmt.Println("regular key len", regK)
		fmt.Println("ratio", float64(newK)/float64(regK))
		tmpLs := promutils.GetLabels()
		got := bm.decompress(tmpLs, s)
		if !reflect.DeepEqual(labels, got.Labels) {
			fmt.Println(labels)
			fmt.Println(got.Labels)
			t.Fatal()
		}
		promutils.PutLabels(tmpLs)
	}

	fn(1, 1)
	fn(1, 10)
	fn(1, 100)
	fn(10, 100)
	fn(100, 100)
	fn(100, 10)
	fn(100, 1)
}
