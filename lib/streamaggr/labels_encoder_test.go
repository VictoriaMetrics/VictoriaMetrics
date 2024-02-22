package streamaggr

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestEncoderEncodeDecode(t *testing.T) {
	le := &labelsEncoder{}
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
		s := string(le.encode(bb, labels))
		fmt.Println("-------- cardinality", metrics*series)
		newK, regK := len(s), len(marshalLabelsFast(nil, labels))
		fmt.Println("compressed key len", newK)
		fmt.Println("regular key len", regK)
		fmt.Println("ratio", float64(newK)/float64(regK))
		got, err := le.decode(nil, s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(labels, got) {
			fmt.Println(labels)
			fmt.Println(got)
			t.Fatal()
		}
	}

	fn(1, 1)
	fn(1, 10)
	fn(1, 100)
	fn(10, 100)
	fn(100, 100)
	fn(100, 10)
	fn(100, 1)
}
