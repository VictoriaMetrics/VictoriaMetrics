package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmestimator/protoparser"
)

func TestGlobalSnapshot(t *testing.T) {
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

	f := func(gen func(e *estimator)) {
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
		expMetric := buf.String()

		buf.Reset()
		if err := convertNoGroupToSnapshot(e, nil).writeMetrics(buf); err != nil {
			t.Fatalf("convertNoGroupToSnapshot: %v", err)
		}
		assertMetricsSame(t, "convertNoGroupToSnapshot", expMetric, buf.String())

		// test encode/decode snapshot produce same result
		buf.Reset()
		if err := e.writeSnapshot(gob.NewEncoder(buf)); err != nil {
			t.Fatalf("writeSnapshot: %v", err)
		}
		ss := newSnapshots()
		if err := decodeSnapshots(buf, func(s *snapshot) {
			ss.add(s)
		}); err != nil {
			t.Fatalf("decodeSnapshot: %v", err)
		}
		buf.Reset()
		if err := ss.writeMetrics(buf); err != nil {
			t.Fatalf("writeMetrics: %v", err)
		}
		assertMetricsSame(t, "encode/decode", expMetric, buf.String())
	}

	f(func(e *estimator) {})

	// no previous
	f(genCard(0, ""))
	f(genCard(100, ""))
	f(genCard(10000, ""))
	f(genCard(100000, ""))

	// rotate once
	genRotateOnce := func(cardinality int) func(e *estimator) {
		return func(e *estimator) {
			genCard(cardinality, "")(e)
			e.rotate()
		}
	}
	f(genRotateOnce(0))
	f(genRotateOnce(100))
	f(genRotateOnce(10000))
	f(genRotateOnce(100000))

	// insert, rotate insert the same
	genInsertRotateInsertSameOnce := func(cardinality int) func(e *estimator) {
		return func(e *estimator) {
			genCard(cardinality/2, "")(e)
			e.rotate()
			genCard(cardinality/2, "")(e)
		}
	}
	f(genInsertRotateInsertSameOnce(0))
	f(genInsertRotateInsertSameOnce(100))
	f(genInsertRotateInsertSameOnce(10000))
	f(genInsertRotateInsertSameOnce(100000))

	// insert, rotate insert
	genInsertRotateInsertOnce := func(cardinality int) func(e *estimator) {
		return func(e *estimator) {
			genCard(cardinality/2, "one")(e)
			e.rotate()
			genCard(cardinality/2, "two")(e)
		}
	}
	f(genInsertRotateInsertOnce(0))
	f(genInsertRotateInsertOnce(100))
	f(genInsertRotateInsertOnce(10000))
	f(genInsertRotateInsertOnce(100000))

	// insert, rotate insert
	genRotateTwoTimes := func(cardinality int) func(e *estimator) {
		return func(e *estimator) {
			genCard(cardinality, "")(e)
			e.rotate()
			e.rotate()
		}
	}
	f(genRotateTwoTimes(0))
	f(genRotateTwoTimes(100))
	f(genRotateTwoTimes(10000))
	f(genRotateTwoTimes(100000))
}

func TestGroupSnapshot(t *testing.T) {
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

	f := func(groupBy []string, gen func(e *estimator)) {
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
		expMetrics := buf.String()

		buf.Reset()
		if err := convertGroupToSnapshot(e, nil).writeMetrics(buf); err != nil {
			t.Fatalf("failed to write metrics: %v", err)
		}
		assertMetricsSame(t, "convertGroupToSnapshot", expMetrics, buf.String())

		// test encode/decode snapshot produce same result
		buf.Reset()
		if err := e.writeSnapshot(gob.NewEncoder(buf)); err != nil {
			t.Fatalf("writeSnapshot: %v", err)
		}
		ss := newSnapshots()
		if err := decodeSnapshots(buf, func(s *snapshot) {
			ss.add(s)
		}); err != nil {
			t.Fatalf("decodeSnapshot: %v", err)
		}
		buf.Reset()
		if err := ss.writeMetrics(buf); err != nil {
			t.Fatalf("writeMetrics: %v", err)
		}
		assertMetricsSame(t, "encode/decode", expMetrics, buf.String())
	}

	f([]string{"__name__"}, func(e *estimator) {})

	// group by metric name
	f([]string{"__name__"}, genCard(10, 10, 10, ""))

	// time series does not contribute to a group
	f([]string{"foo"}, genCard(0, 10, 10, ""))

	// group by one label
	f([]string{"foo"}, genCard(1, 100, 0, ""))
	f([]string{"foo"}, genCard(1, 10000, 0, ""))
	f([]string{"foo"}, genCard(1, 1000, 1000, ""))

	// group by one label, rotate
	genCardRotate := func(fooCard, barCard, bazCard int, seed string) func(e *estimator) {
		return func(e *estimator) {
			genCard(fooCard, barCard, bazCard, seed)(e)
			e.rotate()
		}
	}
	f([]string{"foo"}, genCardRotate(1, 10, 10, ""))
	f([]string{"foo"}, genCardRotate(1, 1000, 1000, ""))

	// group by one label, rotate, insert same
	genCardRotateInsertSame := func(barCard, bazCard int) func(e *estimator) {
		return func(e *estimator) {
			genCard(1, barCard, bazCard, "")(e)
			e.rotate()
			genCard(1, barCard, bazCard, "")(e)
		}
	}
	f([]string{"foo"}, genCardRotateInsertSame(10, 10))
	f([]string{"foo"}, genCardRotateInsertSame(1000, 1000))

	// group by one label, rotate, insert diff
	genCardRotateInsertDiff := func(barCard, bazCard int) func(e *estimator) {
		return func(e *estimator) {
			genCard(1, barCard, bazCard, "one")(e)
			e.rotate()
			genCard(1, barCard, bazCard, "two")(e)
		}
	}
	f([]string{"foo"}, genCardRotateInsertDiff(10, 10))
	f([]string{"foo"}, genCardRotateInsertDiff(1000, 1000))

	// group by one label, rotate, insert diff
	genCardRotateTwice := func(barCard, bazCard int) func(e *estimator) {
		return func(e *estimator) {
			genCard(1, barCard, bazCard, "one")(e)
			e.rotate()
			e.rotate()
		}
	}
	f([]string{"foo"}, genCardRotateTwice(10, 10))
	f([]string{"foo"}, genCardRotateTwice(1000, 1000))

	// group by two labels
	f([]string{"foo", "bar"}, genCard(1, 1, 1000, ""))
	f([]string{"foo", "bar"}, genCard(2, 1, 1000, ""))
	f([]string{"foo", "bar"}, genCard(2, 2, 1000, ""))

	// group by two labels, rotate
	genCardTwoLabelsRotate := func() func(e *estimator) {
		return func(e *estimator) {
			genCard(2, 2, 1000, "")(e)
			e.rotate()
		}
	}
	f([]string{"foo", "bar"}, genCardTwoLabelsRotate())

	// group by two labels, rotate, insert same
	genCardTwoLabelsRotateInsertSame := func() func(e *estimator) {
		return func(e *estimator) {
			genCard(2, 2, 1000, "")(e)
			e.rotate()
			genCard(2, 2, 1000, "")(e)
		}
	}
	f([]string{"foo", "bar"}, genCardTwoLabelsRotateInsertSame())

	// group by two labels, rotate, insert diff
	genCardTwoLabelsRotateInsertDiff := func() func(e *estimator) {
		return func(e *estimator) {
			genCard(2, 2, 1000, "one")(e)
			e.rotate()
			genCard(2, 2, 1000, "two")(e)
		}
	}
	f([]string{"foo", "bar"}, genCardTwoLabelsRotateInsertDiff())

	// group by two labels, rotate, insert diff
	genCardTwoLabelsRotateTwice := func() func(e *estimator) {
		return func(e *estimator) {
			genCard(2, 2, 1000, "one")(e)
			e.rotate()
			e.rotate()
		}
	}
	f([]string{"foo", "bar"}, genCardTwoLabelsRotateTwice())

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
	f([]string{"foo"}, genSpecialCard(`a"b`))
	f([]string{"foo"}, genSpecialCard(`a\b`))
	f([]string{"foo"}, genSpecialCard("a\nb"))
	f([]string{"foo"}, genSpecialCard("a\tb"))
}

func TestGroupSnapshotGroupLimit(t *testing.T) {
	makeTS := func(fooVal string) protoparser.TimeSerie {
		return protoparser.TimeSerie{
			GroupLabels: []protoparser.Label{{Name: "foo", Value: fooVal}},
			Fingerprint: hash([]byte("foo=" + fooVal + ",")),
		}
	}

	f := func(groupLimit int, gen func(e *estimator), expRejected int) {
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
		expMetrics := buf.String()

		buf.Reset()
		s := convertGroupToSnapshot(e, nil)
		if err := s.writeMetrics(buf); err != nil {
			t.Fatalf("failed to write metrics: %v", err)
		}
		assertMetricsSame(t, "convertGroupToSnapshot", expMetrics, buf.String())

		var actRejected int
		if s.GroupRejectedSketch != nil {
			actRejected = int(s.GroupRejectedSketch.Estimate())
		}
		if expRejected != actRejected {
			t.Fatalf("rejected expected: %d; got: %d", expRejected, actRejected)
		}

		// test encode/decode snapshot produce same result
		buf.Reset()
		if err := e.writeSnapshot(gob.NewEncoder(buf)); err != nil {
			t.Fatalf("writeSnapshot: %v", err)
		}
		ss := newSnapshots()
		if err := decodeSnapshots(buf, func(s *snapshot) {
			ss.add(s)
		}); err != nil {
			t.Fatalf("decodeSnapshot: %v", err)
		}
		buf.Reset()
		if err := ss.writeMetrics(buf); err != nil {
			t.Fatalf("writeMetrics: %v", err)
		}
		assertMetricsSame(t, "encode/decode", expMetrics, buf.String())
	}

	// all groups accepted
	f(3, func(e *estimator) {
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b"), makeTS("c")})
	}, 0)

	// 2 groups only accepted
	f(2, func(e *estimator) {
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b"), makeTS("c")})
	}, 1)

	// one group only accepted
	f(1, func(e *estimator) {
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b"), makeTS("c")})
	}, 2)

	// after rotate: groups in prevGroups bypass the limit; new groups are still checked
	f(2, func(e *estimator) {
		// fills limit
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b")})
		e.rotate()
		// "a" bypasses, "c" rejected
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("c")})
	}, 1)

	// after rotate: new group accepted when remaining capacity allows
	f(3, func(e *estimator) {
		// 2 groups, limit=3
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("b")})
		e.rotate()
		// "a" bypasses, "c" accepted (2+1=3 <= 3)
		e.insertMany([]protoparser.TimeSerie{makeTS("a"), makeTS("c")})
	}, 0)

	// reject 100
	f(3, func(e *estimator) {
		var tss []protoparser.TimeSerie
		for i := 0; i < 103; i++ {
			tss = append(tss, makeTS(fmt.Sprintf("a%d", i)))
		}
		e.insertMany(tss)
	}, 100)
}
