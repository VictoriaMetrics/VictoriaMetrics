package datadogsketches

import (
	"math"
	"testing"
)

func TestPointsForQuantile(t *testing.T) {
	f := func(d *Dogsketch, q float64, vExpected float64) {
		t.Helper()
		v := d.valueForQuantile(q)
		if math.Abs(v-vExpected) > 0.4 {
			t.Fatalf("unexpected value; got %v; want %v", v, vExpected)
		}
	}
	sketches := &Dogsketch{
		Min: 8.0,
		Max: 20.0,
		Cnt: 17,
		N:   []uint32{0x0, 0x0, 0x1, 0x0, 0x1, 0x4, 0x6, 0x1, 0x2, 0x0, 0x1, 0x0, 0x1},
		K:   []int32{0, 1472, 1473, 1479, 1480, 1503, 1504, 1512, 1513, 1514, 1515, 1531, 1532},
	}
	f(sketches, 0.1, 8.96)
	f(sketches, 0.5, 13.01)
	f(sketches, 0.75, 14.96)
	f(sketches, 0.9, 14.96)
	f(sketches, 0.95, 15.43)
	f(sketches, 0.99, 15.43)
}
