package datadogsketches

import (
	"fmt"
	"math"
	"strconv"

	"github.com/VictoriaMetrics/easyproto"
)

var (
	// These constants were obtained from https://github.com/DataDog/opentelemetry-mapping-go/blob/48d52eeea60d28da2e14c154a24557c4d290c6e2/pkg/quantile/config.go
	eps        = 1.0 / 128
	gamma      = 1 + 2*eps
	gammaLn    = math.Log1p(2 * eps)
	defaultMin = 1e-9
	bias       = 1 - int(math.Floor(math.Log(defaultMin)/gammaLn))
	quantiles  = []float64{0.5, 0.75, 0.9, 0.95, 0.99}
)

var quantilesStr = func() []string {
	a := make([]string, len(quantiles))
	for i, q := range quantiles {
		a[i] = strconv.FormatFloat(q, 'g', 3, 64)
	}
	return a
}()

// Label is a single label for Metric
type Label struct {
	Name  string
	Value string
}

// Metric stores metrics extracted from sketches
type Metric struct {
	Name   string
	Labels []Label
	Points []Point
}

// Point stores a single point extracted from sketches
type Point struct {
	Value     float64
	Timestamp int64
}

// SketchPayload stores sketches extracted from /api/beta/sketches endpoint
//
//	message SketchPayload {
//		 repeated Sketch sketches = 1
//	}
//
// See https://github.com/DataDog/agent-payload/blob/38db68d9641c8a0bd2e1eac53b9d54793448850f/proto/metrics/agent_payload.proto#L90
type SketchPayload struct {
	Sketches []*Sketch
}

// UnmarshalProtobuf decodes src to SketchPayload struct
func (sp *SketchPayload) UnmarshalProtobuf(src []byte) (err error) {
	sp.Sketches = nil
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in SketchPayload message: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Sketch data")
			}
			var s Sketch
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Sketch: %w", err)
			}
			sp.Sketches = append(sp.Sketches, &s)
		}
	}
	return nil
}

// Sketch proto struct
//
//	message Sketch {
//	  string metric = 1;
//	  string host = 2;
//	  repeated string tags = 4;
//	  repeated Dogsketch dogsketches = 7
//	}
//
// See https://github.com/DataDog/agent-payload/blob/38db68d9641c8a0bd2e1eac53b9d54793448850f/proto/metrics/agent_payload.proto#L91
type Sketch struct {
	Metric      string
	Host        string
	Tags        []string
	Dogsketches []*Dogsketch
}

// unmarshalProtobuf decodes src to Sketch struct
func (s *Sketch) unmarshalProtobuf(src []byte) (err error) {
	s.Metric = ""
	s.Host = ""
	s.Tags = nil
	s.Dogsketches = nil

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Sketch message: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			metric, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read metric")
			}
			s.Metric = metric
		case 2:
			host, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read host")
			}
			s.Host = host
		case 4:
			tag, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read tag")
			}
			s.Tags = append(s.Tags, tag)
		case 7:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Dogsketch data")
			}
			var d Dogsketch
			if err := d.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Dogsketch: %w", err)
			}
			s.Dogsketches = append(s.Dogsketches, &d)
		}
	}
	return nil
}

// RowsCount returns the number of samples s generates.
func (s *Sketch) RowsCount() int {
	// The sketch contains len(quantiles) plus *_sum and *_count metrics
	// per each Dogsketch in s.Dogsketches.
	return (len(quantiles) + 2) * len(s.Dogsketches)
}

// ToSummary generates Prometheus summary from the given s.
func (s *Sketch) ToSummary() []*Metric {
	quantilesLen := len(quantiles)
	metrics := make([]*Metric, quantilesLen, quantilesLen+4)
	dogsketches := s.Dogsketches

	sumPoints := make([]Point, 0, len(dogsketches))
	countPoints := make([]Point, 0, len(dogsketches))
	minPoints := make([]Point, 0, len(dogsketches))
	maxPoints := make([]Point, 0, len(dogsketches))

	for _, d := range dogsketches {
		timestamp := d.Ts * 1000
		sumPoints = append(sumPoints, Point{
			Timestamp: timestamp,
			Value:     d.Sum,
		})
		countPoints = append(countPoints, Point{
			Timestamp: timestamp,
			Value:     float64(d.Cnt),
		})
		minPoints = append(minPoints, Point{
			Timestamp: timestamp,
			Value:     float64(d.Min),
		})
		maxPoints = append(maxPoints, Point{
			Timestamp: timestamp,
			Value:     float64(d.Max),
		})
	}

	for i, q := range quantiles {
		points := make([]Point, 0, len(dogsketches))
		for _, d := range dogsketches {
			timestamp := d.Ts * 1000
			points = append(points, Point{
				Timestamp: timestamp,
				Value:     d.quantile(q),
			})
		}
		metrics[i] = &Metric{
			Name: s.Metric,
			Labels: []Label{{
				Name:  "quantile",
				Value: quantilesStr[i],
			}},
			Points: points,
		}
	}

	metrics = append(metrics,
		&Metric{
			Name:   s.Metric + ".sum",
			Points: sumPoints,
		},
		&Metric{
			Name:   s.Metric + ".count",
			Points: countPoints,
		},
		&Metric{
			Name:   s.Metric + ".min",
			Points: minPoints,
		},
		&Metric{
			Name:   s.Metric + ".max",
			Points: maxPoints,
		},
	)

	return metrics
}

// Dogsketch proto struct
//
//	message Dogsketch {
//	  int64 ts = 1;
//	  int64 cnt = 2;
//	  double min = 3;
//	  double max = 4;
//	  double sum = 6;
//	  repeated sint32 k = 7;
//	  repeated uint32 n = 8;
//	}
//
// See https://github.com/DataDog/agent-payload/blob/38db68d9641c8a0bd2e1eac53b9d54793448850f/proto/metrics/agent_payload.proto#L104
type Dogsketch struct {
	Ts  int64
	Cnt int64
	Min float64
	Max float64
	Sum float64
	K   []int32
	N   []uint32
}

// unmarshalProtobuf decodes src to Dogsketch struct
func (d *Dogsketch) unmarshalProtobuf(src []byte) (err error) {
	d.Ts = 0
	d.Cnt = 0
	d.Min = 0.0
	d.Max = 0.0
	d.Sum = 0.0
	d.K = nil
	d.N = nil

	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Dogsketch message: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			ts, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read timestamp")
			}
			d.Ts = ts
		case 2:
			cnt, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read count")
			}
			d.Cnt = cnt
		case 3:
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read min")
			}
			d.Min = v
		case 4:
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read max")
			}
			d.Max = v
		case 6:
			sum, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read sum")
			}
			d.Sum = sum
		case 7:
			var ok bool
			d.K, ok = fc.UnpackSint32s(d.K)
			if !ok {
				return fmt.Errorf("cannot read k")
			}
		case 8:
			var ok bool
			d.N, ok = fc.UnpackUint32s(d.N)
			if !ok {
				return fmt.Errorf("cannot read n")
			}
		}
	}
	return nil
}

// This function has been copied from https://github.com/DataDog/opentelemetry-mapping-go/blob/48d52eeea60d28da2e14c154a24557c4d290c6e2/pkg/quantile/sparse.go#L92
func (d *Dogsketch) quantile(q float64) float64 {
	switch {
	case d.Cnt == 0:
		return 0
	case q <= 0:
		return d.Min
	case q >= 1:
		return d.Max
	}

	ns := d.N
	ks := d.K
	if len(ns) != len(ks) {
		// Avoid index out of range panic in the loop below.
		return 0
	}

	rank := math.RoundToEven(q * float64(d.Cnt-1))
	cnt := float64(0)
	for i, n := range ns {
		cnt += float64(n)
		if cnt <= rank {
			continue
		}
		weight := (cnt - rank) / float64(n)
		vLow := f64(ks[i])
		vHigh := vLow * gamma
		if i == 0 {
			vLow = d.Min
		}
		return vLow*weight + vHigh*(1-weight)
	}
	return d.Max
}

// This function has been copied from https://github.com/DataDog/opentelemetry-mapping-go/blob/48d52eeea60d28da2e14c154a24557c4d290c6e2/pkg/quantile/config.go#L54
func f64(k int32) float64 {
	// See https://github.com/DataDog/opentelemetry-mapping-go/blob/48d52eeea60d28da2e14c154a24557c4d290c6e2/pkg/quantile/key.go#L14
	if k <= -((1 << 15) - 1) {
		return math.Inf(-1)
	}
	if k >= ((1 << 15) - 1) {
		return math.Inf(1)
	}
	if k == 0 {
		return 0
	}
	if k < 0 {
		return -f64(-k)
	}

	exp := float64(int(k) - bias)
	return math.Pow(gamma, exp)
}
