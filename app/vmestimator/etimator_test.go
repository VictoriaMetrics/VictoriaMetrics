package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmestimator/protoparser"
)

func TestGlobalEstimate(t *testing.T) {
	genCard := func(cardinality int, seed string) func(e *estimator) {
		return func(e *estimator) {
			var tss []protoparser.TimeSerie
			fpBuf := make([]byte, 8, 8+len(seed))
			for i := 0; i < cardinality; i++ {
				binary.LittleEndian.PutUint64(fpBuf[:8], uint64(i))
				fpBuf = append(fpBuf, seed...)
				tss = append(tss, protoparser.TimeSerie{
					Fingerprint: hash(fpBuf[:]),
				})

				if i%10 == 0 {
					e.insertMany(tss)
					tss = tss[:0]
				}
			}
			if len(tss) > 0 {
				e.insertMany(tss)
			}
		}
	}

	f := func(gen func(e *estimator), expMetric string) {
		t.Helper()

		cfg := EstimatorConfig{
			Interval: time.Minute * 10,
			Buckets:  5,
		}

		e, err := newEstimator(cfg)
		if err != nil {
			t.Fatalf("failed to create new estimator: %v", err)
		}
		defer e.stop()

		gen(e)

		if len(e.buckets) != cfg.Buckets {
			t.Fatalf("expected buckets length to be %d but got %d", cfg.Buckets, len(e.buckets))
		}
		for i, eb := range e.buckets {
			if len(eb.groupBy) > 0 {
				t.Fatalf("expected bucket %d groupBy length to be 0 but got %d", i, len(eb.groupBy))
			}
			if eb.groups != nil {
				t.Fatalf("expected bucket %d groups length to be 0 but got %d", i, len(eb.groups))
			}
			if eb.groupSize.Load() != 0 {
				t.Fatalf("expected bucket %d groupSize to be 0 but got %d", i, eb.groupSize.Load())
			}
		}

		buf := bytes.NewBuffer(nil)
		e.writeMetrics(buf)

		if strings.TrimSpace(buf.String()) != expMetric {
			t.Fatalf("\nexpected:\n%s\n\ngot:\n%s", expMetric, buf.String())
		}
	}

	// no previous
	f(genCard(0, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genCard(1, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 1`)
	f(genCard(10, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 10`)
	f(genCard(100, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 100`)
	f(genCard(1000, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 1000`)
	f(genCard(5000, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 4998`)
	f(genCard(10000, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 9920`)
	f(genCard(100000, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 99658`)
	f(genCard(500000, ""), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 496552`)

	// rotate once
	genRotateOnce := func(cardinality int) func(e *estimator) {
		return func(e *estimator) {
			genCard(cardinality, "")(e)
			e.rotate()
		}
	}
	f(genRotateOnce(0), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateOnce(1), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 1`)
	f(genRotateOnce(10), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 10`)
	f(genRotateOnce(100), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 100`)
	f(genRotateOnce(1000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 1000`)
	f(genRotateOnce(5000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 4998`)
	f(genRotateOnce(10000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 9920`)
	f(genRotateOnce(100000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 99658`)
	f(genRotateOnce(500000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 496552`)

	// insert, rotate insert the same
	genInsertRotateInsertSameOnce := func(cardinality int) func(e *estimator) {
		return func(e *estimator) {
			genCard(cardinality/2, "")(e)
			e.rotate()
			genCard(cardinality/2, "")(e)
		}
	}
	f(genInsertRotateInsertSameOnce(0), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genInsertRotateInsertSameOnce(1), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genInsertRotateInsertSameOnce(10), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 5`)
	f(genInsertRotateInsertSameOnce(100), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 50`)
	f(genInsertRotateInsertSameOnce(1000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 500`)
	f(genInsertRotateInsertSameOnce(5000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 2499`)
	f(genInsertRotateInsertSameOnce(10000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 4998`)
	f(genInsertRotateInsertSameOnce(100000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 49529`)
	f(genInsertRotateInsertSameOnce(200000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 99658`)

	// insert, rotate insert
	genInsertRotateInsertOnce := func(cardinality int) func(e *estimator) {
		return func(e *estimator) {
			genCard(cardinality/2, "one")(e)
			e.rotate()
			genCard(cardinality/2, "two")(e)
		}
	}
	f(genInsertRotateInsertOnce(0), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genInsertRotateInsertOnce(1), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genInsertRotateInsertOnce(10), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 10`)
	f(genInsertRotateInsertOnce(100), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 100`)
	f(genInsertRotateInsertOnce(1000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 1000`)
	f(genInsertRotateInsertOnce(5000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 5000`)
	f(genInsertRotateInsertOnce(10000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 10058`)
	f(genInsertRotateInsertOnce(100000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 99543`)
	f(genInsertRotateInsertOnce(200000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 198814`)

	// insert, rotate insert
	genRotateTwoTimes := func(cardinality int) func(e *estimator) {
		return func(e *estimator) {
			genCard(cardinality, "")(e)
			e.rotate()
			e.rotate()
		}
	}
	f(genRotateTwoTimes(0), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateTwoTimes(1), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateTwoTimes(10), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateTwoTimes(100), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateTwoTimes(1000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateTwoTimes(5000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateTwoTimes(10000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateTwoTimes(100000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
	f(genRotateTwoTimes(500000), `cardinality_estimate{interval="10m0s",group_by_keys="__global__"} 0`)
}

func TestGroupEstimate(t *testing.T) {
	genCard := func(fooCard, barCard, bazCard int, seed string) func(e *estimator) {
		return func(e *estimator) {
			var tss []protoparser.TimeSerie
			for fooI := 0; fooI < max(1, fooCard); fooI++ {
				for barI := 0; barI < max(1, barCard); barI++ {
					for bazI := 0; bazI < max(1, bazCard); bazI++ {
						ts := protoparser.TimeSerie{}
						ts.GroupLabels = append(ts.GroupLabels, protoparser.Label{Name: "__name__", Value: "the_metric_name"})
						if fooCard > 0 {
							ts.GroupLabels = append(ts.GroupLabels, protoparser.Label{Name: "foo", Value: fmt.Sprintf("%s%d", seed, fooI)})
						}
						if barCard > 0 {
							ts.GroupLabels = append(ts.GroupLabels, protoparser.Label{Name: "bar", Value: fmt.Sprintf("%s%d", seed, barI)})
						}
						if bazCard > 0 {
							ts.GroupLabels = append(ts.GroupLabels, protoparser.Label{Name: "baz", Value: fmt.Sprintf("%s%d", seed, bazI)})
						}
						var fpBuf []byte
						for _, l := range ts.GroupLabels {
							fpBuf = append(fpBuf, l.Name...)
							fpBuf = append(fpBuf, '=')
							fpBuf = append(fpBuf, l.Value...)
							fpBuf = append(fpBuf, ',')
						}
						fpBuf = append(fpBuf, seed...)
						ts.Fingerprint = hash(fpBuf)
						tss = append(tss, ts)
					}
				}
			}
			e.insertMany(tss)
		}
	}

	f := func(groupBy []string, gen func(e *estimator), expMetrics string) {
		t.Helper()

		cfg := EstimatorConfig{
			Interval: time.Minute * 10,
			GroupBy:  groupBy,
			Buckets:  5,
		}

		e, err := newEstimator(cfg)
		if err != nil {
			t.Fatalf("failed to create new estimator: %v", err)
		}
		defer e.stop()

		gen(e)

		if len(e.buckets) != cfg.Buckets {
			t.Fatalf("expected buckets length to be %d but got %d", cfg.Buckets, len(e.buckets))
		}
		for i, eb := range e.buckets {
			if eb.sketch != nil {
				t.Fatalf("expected bucket %d sketch to be nil", i)
			}
			if eb.prevSketch != nil {
				t.Fatalf("expected bucket %d prevSketch to be nil", i)
			}
		}

		buf := bytes.NewBuffer(nil)
		e.writeMetrics(buf)

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		sort.Strings(lines)
		actMetrics := "\n" + strings.Join(lines, "\n")

		if expMetrics != actMetrics {
			t.Fatalf("\nexpected:\n%s\n\ngot:\n%s", expMetrics, actMetrics)
		}
	}

	// group by metric name
	f([]string{"__name__"}, genCard(10, 10, 10, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="__name__"} 1
cardinality_estimate{interval="10m0s",group_by_keys="__name__",group_by_values="the_metric_name",by__name__="the_metric_name"} 1000`,
	)

	// time series does not contribute to a group
	f([]string{"foo"}, genCard(0, 10, 10, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 0`,
	)
	f([]string{"foo", "bar"}, genCard(0, 0, 10, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo,bar"} 0`,
	)

	// group by one label
	f([]string{"foo"}, genCard(1, 1, 0, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 1`,
	)
	f([]string{"foo"}, genCard(1, 2, 0, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 2`,
	)
	f([]string{"foo"}, genCard(1, 10, 0, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 10`,
	)
	f([]string{"foo"}, genCard(1, 100, 0, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 100`,
	)
	f([]string{"foo"}, genCard(1, 1000, 0, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 1000`,
	)
	f([]string{"foo"}, genCard(1, 10000, 0, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 9957`,
	)
	f([]string{"foo"}, genCard(1, 50000, 0, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 50387`,
	)
	f([]string{"foo"}, genCard(1, 1, 1, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 1`,
	)
	f([]string{"foo"}, genCard(1, 2, 2, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 4`,
	)
	f([]string{"foo"}, genCard(1, 10, 10, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 100`,
	)
	f([]string{"foo"}, genCard(1, 100, 100, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 9954`,
	)
	f([]string{"foo"}, genCard(1, 1000, 1000, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 1013124`,
	)

	// group by one label, rotate
	genCardRotate := func(fooCard, barCard, bazCard int, seed string) func(e *estimator) {
		return func(e *estimator) {
			genCard(fooCard, barCard, bazCard, seed)(e)
			e.rotate()
		}
	}
	f([]string{"foo"}, genCardRotate(1, 10, 10, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 100`,
	)
	f([]string{"foo"}, genCardRotate(1, 1000, 1000, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 1013124`,
	)

	// group by one label, rotate, insert same
	genCardRotateInsertSame := func(barCard, bazCard int) func(e *estimator) {
		return func(e *estimator) {
			genCard(1, barCard, bazCard, "")(e)
			e.rotate()
			genCard(1, barCard, bazCard, "")(e)
		}
	}
	f([]string{"foo"}, genCardRotateInsertSame(10, 10), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 100`,
	)
	f([]string{"foo"}, genCardRotateInsertSame(1000, 1000), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="0",by_foo="0"} 1013124`,
	)

	// group by one label, rotate, insert diff
	genCardRotateInsertDiff := func(barCard, bazCard int) func(e *estimator) {
		return func(e *estimator) {
			genCard(1, barCard, bazCard, "one")(e)
			e.rotate()
			genCard(1, barCard, bazCard, "two")(e)
		}
	}
	f([]string{"foo"}, genCardRotateInsertDiff(10, 10), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 2
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="one0",by_foo="one0"} 100
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="two0",by_foo="two0"} 100`,
	)
	f([]string{"foo"}, genCardRotateInsertDiff(1000, 1000), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 2
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="one0",by_foo="one0"} 995153
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="two0",by_foo="two0"} 992158`,
	)

	// group by one label, rotate, insert diff
	genCardRotateTwice := func(barCard, bazCard int) func(e *estimator) {
		return func(e *estimator) {
			genCard(1, barCard, bazCard, "one")(e)
			e.rotate()
			e.rotate()
		}
	}
	f([]string{"foo"}, genCardRotateTwice(10, 10), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 0`,
	)
	f([]string{"foo"}, genCardRotateTwice(1000, 1000), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 0`,
	)

	// group by two labels
	f([]string{"foo", "bar"}, genCard(1, 1, 1000, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo,bar"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="0,0",by_foo="0",by_bar="0"} 1000`,
	)
	f([]string{"foo", "bar"}, genCard(2, 1, 1000, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo,bar"} 2
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="0,0",by_foo="0",by_bar="0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="1,0",by_foo="1",by_bar="0"} 1000`,
	)
	f([]string{"foo", "bar"}, genCard(2, 2, 1000, ""), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo,bar"} 4
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="0,0",by_foo="0",by_bar="0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="0,1",by_foo="0",by_bar="1"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="1,0",by_foo="1",by_bar="0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="1,1",by_foo="1",by_bar="1"} 1000`,
	)

	// group by two labels, rotate
	genCardTwoLabelsRotate := func() func(e *estimator) {
		return func(e *estimator) {
			genCard(2, 2, 1000, "")(e)
			e.rotate()
		}
	}
	f([]string{"foo", "bar"}, genCardTwoLabelsRotate(), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo,bar"} 4
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="0,0",by_foo="0",by_bar="0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="0,1",by_foo="0",by_bar="1"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="1,0",by_foo="1",by_bar="0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="1,1",by_foo="1",by_bar="1"} 1000`,
	)

	// group by two labels, rotate, insert same
	genCardTwoLabelsRotateInsertSame := func() func(e *estimator) {
		return func(e *estimator) {
			genCard(2, 2, 1000, "")(e)
			e.rotate()
			genCard(2, 2, 1000, "")(e)
		}
	}
	f([]string{"foo", "bar"}, genCardTwoLabelsRotateInsertSame(), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo,bar"} 4
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="0,0",by_foo="0",by_bar="0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="0,1",by_foo="0",by_bar="1"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="1,0",by_foo="1",by_bar="0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="1,1",by_foo="1",by_bar="1"} 1000`,
	)

	// group by two labels, rotate, insert diff
	genCardTwoLabelsRotateInsertDiff := func() func(e *estimator) {
		return func(e *estimator) {
			genCard(2, 2, 1000, "one")(e)
			e.rotate()
			genCard(2, 2, 1000, "two")(e)
		}
	}
	f([]string{"foo", "bar"}, genCardTwoLabelsRotateInsertDiff(), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo,bar"} 8
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="one0,one0",by_foo="one0",by_bar="one0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="one0,one1",by_foo="one0",by_bar="one1"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="one1,one0",by_foo="one1",by_bar="one0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="one1,one1",by_foo="one1",by_bar="one1"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="two0,two0",by_foo="two0",by_bar="two0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="two0,two1",by_foo="two0",by_bar="two1"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="two1,two0",by_foo="two1",by_bar="two0"} 1000
cardinality_estimate{interval="10m0s",group_by_keys="foo,bar",group_by_values="two1,two1",by_foo="two1",by_bar="two1"} 1000`,
	)

	// group by two labels, rotate, insert diff
	genCardTwoLabelsRotateTwice := func() func(e *estimator) {
		return func(e *estimator) {
			genCard(2, 2, 1000, "one")(e)
			e.rotate()
			e.rotate()
		}
	}
	f([]string{"foo", "bar"}, genCardTwoLabelsRotateTwice(), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo,bar"} 0`,
	)

	// quote values: label values with special characters must be properly escaped
	genSpecialCard := func(fooVal string) func(e *estimator) {
		return func(e *estimator) {
			e.insertMany([]protoparser.TimeSerie{
				{
					GroupLabels: []protoparser.Label{{Name: "foo", Value: fooVal}},
					Fingerprint: hash([]byte("foo=" + fooVal + ",")),
				},
			})
		}
	}

	// double quote in value
	f([]string{"foo"}, genSpecialCard(`a"b`), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a\"b",by_foo="a\"b"} 1`,
	)

	f([]string{"foo"}, genSpecialCard(`a\b`), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a\\b",by_foo="a\\b"} 1`,
	)

	f([]string{"foo"}, genSpecialCard("a\nb"), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a\nb",by_foo="a\nb"} 1`,
	)

	f([]string{"foo"}, genSpecialCard("a\tb"), `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a\tb",by_foo="a\tb"} 1`,
	)
}

func TestGroupEstimateGroupLimit(t *testing.T) {
	makeTS := func(fooVal string) protoparser.TimeSerie {
		return protoparser.TimeSerie{
			GroupLabels: []protoparser.Label{{Name: "foo", Value: fooVal}},
			Fingerprint: hash([]byte("foo=" + fooVal + ",")),
		}
	}

	f := func(groupLimit int, gen func(e *estimator), expRejected int, expMetrics string) {
		t.Helper()

		cfg := EstimatorConfig{
			Interval:   time.Minute * 10,
			GroupBy:    []string{"foo"},
			GroupLimit: groupLimit,
			Buckets:    3,
		}

		e, err := newEstimator(cfg)
		if err != nil {
			t.Fatalf("failed to create new estimator: %v", err)
		}
		defer e.stop()

		gen(e)

		buf := bytes.NewBuffer(nil)
		e.writeMetrics(buf)

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		sort.Strings(lines)
		actMetrics := "\n" + strings.Join(lines, "\n")

		if expMetrics != actMetrics {
			t.Fatalf("\nexpected:\n%s\n\ngot:\n%s", expMetrics, actMetrics)
		}

		var actRejected int
		if e.buckets[0].groupRejectedSketch != nil {
			actRejected = int(e.buckets[0].groupRejectedSketch.Estimate())
		}
		if expRejected != actRejected {
			t.Fatalf("rejected expected: %d; got: %d", expRejected, actRejected)
		}
	}

	// all groups accepted
	f(3, func(e *estimator) {
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b"), makeTS("c")})
	}, 0, `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 3
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a",by_foo="a"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="b",by_foo="b"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="c",by_foo="c"} 1`,
	)

	// 2 groups only accepted
	f(2, func(e *estimator) {
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b"), makeTS("c")})
	}, 1, `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 3
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a",by_foo="a"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="b",by_foo="b"} 1`,
	)

	// one group only accepted
	f(1, func(e *estimator) {
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b"), makeTS("c")})
	}, 2, `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 3
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a",by_foo="a"} 1`,
	)

	// after rotate: groups in prevGroups bypass the limit; new groups are still checked
	f(2, func(e *estimator) {
		// fills limit
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b")})
		e.rotate()
		// "a" bypasses, "c" rejected
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("c")})
	}, 1, `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 3
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a",by_foo="a"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="b",by_foo="b"} 1`,
	)

	// after rotate: new group accepted when remaining capacity allows
	f(3, func(e *estimator) {
		// 2 groups, limit=3
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b")})
		e.rotate()
		// "a" bypasses, "c" accepted (2+1=3 <= 3)
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("c")})
	}, 0, `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 3
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a",by_foo="a"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="b",by_foo="b"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="c",by_foo="c"} 1`,
	)

	// reject 100
	f(3, func(e *estimator) {
		var tss []protoparser.TimeSerie
		for i := 0; i < 103; i++ {
			tss = append(tss, makeTS(fmt.Sprintf("a%d", i)))
		}
		e.insertMany(tss)
	}, 100, `
cardinality_estimate{interval="10m0s",group_by_keys="__group__",group_by_values="foo"} 103
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a0",by_foo="a0"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a1",by_foo="a1"} 1
cardinality_estimate{interval="10m0s",group_by_keys="foo",group_by_values="a2",by_foo="a2"} 1`,
	)
}
