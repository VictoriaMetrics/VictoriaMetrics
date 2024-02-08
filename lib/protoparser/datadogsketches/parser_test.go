package datadogsketches

import (
	"math"
	"testing"
)

func TestDogsketchQuantile(t *testing.T) {
	f := func(d *Dogsketch, q float64, vExpected float64) {
		t.Helper()
		v := d.quantile(q)
		if math.Abs(v-vExpected) > 0.01 {
			t.Fatalf("unexpected value; got %v; want %v", v, vExpected)
		}
	}
	sketches := &Dogsketch{
		Min: 8.0,
		Max: 21.0,
		Cnt: 17,
		N:   []uint32{0x0, 0x0, 0x1, 0x0, 0x1, 0x4, 0x6, 0x1, 0x2, 0x0, 0x1, 0x0, 0x1},
		K:   []int32{0, 1472, 1473, 1479, 1480, 1503, 1504, 1512, 1513, 1514, 1515, 1531, 1532},
	}
	f(sketches, 0, 8)
	f(sketches, 0.1, 12.91)
	f(sketches, 0.5, 13.18)
	f(sketches, 0.75, 14.84)
	f(sketches, 0.9, 15.19)
	f(sketches, 0.95, 15.55)
	f(sketches, 0.99, 20.24)
	f(sketches, 1, 21)
}
