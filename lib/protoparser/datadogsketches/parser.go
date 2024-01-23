package datadogsketches

import (
	"fmt"
	"math"
	"strconv"

	"github.com/VictoriaMetrics/easyproto"
)

var (
	epsillon   = 1.0 / 128
	gamma      = 1 + 2*epsillon
	gammaLn    = math.Log(gamma)
	defaultMin = 0.981e-9
	bias       = 1 - int(math.Floor(math.Log(defaultMin)/gammaLn))
	quantiles  = []float64{0.5, 0.75, 0.9, 0.95, 0.99}
)

type label struct {
	Name  string
	Value string
}

// Metric stores metrics extracted from sketches
type Metric struct {
	Name   string
	Labels []label
	Points []float64
}

// SketchPayload stores sketches extracted from /api/beta/sketches endpoint
//
//	message SketchPayload {
//		 repeated Sketch sketches = 1
//	}
type SketchPayload struct {
	Sketches []*Sketch
}

// UnmarshalProtobuf decodes byte array to SketchPayload struct
func (sp *SketchPayload) UnmarshalProtobuf(src []byte) (err error) {
	sp.Sketches = sp.Sketches[:0]
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
				return fmt.Errorf("cannot read SketchPayload sketches data")
			}
			sp.Sketches = append(sp.Sketches, &Sketch{})
			s := sp.Sketches[len(sp.Sketches)-1]
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal sketch: %w", err)
			}
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
type Sketch struct {
	Metric      string
	Host        string
	Tags        []string
	Dogsketches []*Dogsketch
}

// unmarshalProtobuf decodes byte array to Sketch struct
func (s *Sketch) unmarshalProtobuf(src []byte) (err error) {
	s.Metric = ""
	s.Host = ""
	s.Tags = s.Tags[:0]
	s.Dogsketches = s.Dogsketches[:0]

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
				return fmt.Errorf("cannot read Sketch metric")
			}
			s.Metric = metric
		case 2:
			host, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Sketch host")
			}
			s.Host = host
		case 4:
			tag, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Sketch tag")
			}
			s.Tags = append(s.Tags, tag)
		case 7:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Sketch dogsketches data")
			}
			s.Dogsketches = append(s.Dogsketches, &Dogsketch{})
			d := s.Dogsketches[len(s.Dogsketches)-1]
			if err := d.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal dogsketch: %w", err)
			}
		}
	}
	return nil
}

// RowsCount calculates generated rows num from sketch
func (s *Sketch) RowsCount() int {
	return (len(quantiles) + len(s.extractAggr())) * len(s.Dogsketches)
}

func (s *Sketch) extractAggr() []*Metric {
	return []*Metric{
		{
			Name:   s.Metric + "_sum",
			Labels: []label{},
			Points: make([]float64, len(s.Dogsketches)),
		}, {
			Name:   s.Metric + "_count",
			Labels: []label{},
			Points: make([]float64, len(s.Dogsketches)),
		},
	}
}

// ToHistogram generates histogram metrics
func (s *Sketch) ToHistogram() []*Metric {
	dogsketches := s.Dogsketches
	aggr := s.extractAggr()
	metrics := make([]*Metric, len(quantiles))
	for q := range quantiles {
		quantile := quantiles[q]
		metrics[q] = &Metric{
			Name: s.Metric,
			Labels: []label{{
				Name:  "quantile",
				Value: strconv.FormatFloat(quantile, 'g', 3, 64),
			}},
			Points: make([]float64, len(dogsketches)),
		}
		for d := range dogsketches {
			dogsketch := dogsketches[d]
			aggr[0].Points[d] = dogsketch.Sum
			aggr[1].Points[d] = float64(dogsketch.Cnt)
			metrics[q].Points[d] = dogsketch.pointForQuantile(quantile)
		}
	}
	return append(metrics, aggr...)
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
type Dogsketch struct {
	Ts  int64
	Cnt int64
	Min float64
	Max float64
	Sum float64
	K   []int32
	N   []uint32
}

// unmarshalProtobuf decodes byte array to Dogsketch struct
func (d *Dogsketch) unmarshalProtobuf(src []byte) (err error) {
	d.Ts = 0
	d.Cnt = 0
	d.Min = 0.0
	d.Max = 0.0
	d.Sum = 0.0
	d.K = d.K[:0]
	d.N = d.N[:0]
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Dogsketch message, %w", err)
		}
		switch fc.FieldNum {
		case 1:
			ts, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read Dogsketch timestamp")
			}
			d.Ts = ts
		case 2:
			cnt, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read Dogsketch count")
			}
			d.Cnt = cnt
		case 3:
			min, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Dogsketch min")
			}
			d.Min = min
		case 4:
			max, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Dogsketch max")
			}
			d.Max = max
		case 6:
			sum, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Dogsketch sum")
			}
			d.Sum = sum
		case 7:
			var ok bool
			d.K, ok = fc.UnpackSint32s(d.K)
			if !ok {
				return fmt.Errorf("cannot read Dogsketch k")
			}
		case 8:
			var ok bool
			d.N, ok = fc.UnpackUint32s(d.N)
			if !ok {
				return fmt.Errorf("cannot read Dogsketch n")
			}
		}
	}
	return nil
}

func (d *Dogsketch) pointForQuantile(quantile float64) float64 {
	switch {
	case d.Cnt == 0:
		return 0
	case quantile <= 0:
		return d.Min
	case quantile >= 1:
		return d.Max
	}

	rank := quantile * float64(d.Cnt-1)
	nLen := len(d.N)
	for cnt, i := 0.0, 0; i < nLen; i++ {
		cnt += float64(d.N[i])
		if cnt <= rank {
			continue
		}
		weight := (cnt - rank) / float64(d.N[i])
		vLow := f64(d.K[i])
		vHigh := vLow * gamma
		switch i {
		case nLen:
			vHigh = d.Max
		case 0:
			vLow = d.Min
		}
		return vLow*weight + vHigh*(1-weight)
	}
	return d.Max
}

func f64(k int32) float64 {
	switch {
	case k < 0:
		return -f64(-k)
	case k == math.MaxInt16 || k == math.MinInt16:
		return math.Inf(int(k))
	case k == 0:
		return 0
	}
	exp := float64(int(k) - bias)
	return math.Pow(gamma, exp)
}
