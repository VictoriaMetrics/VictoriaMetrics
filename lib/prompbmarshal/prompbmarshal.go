package prompbmarshal

import (
	"github.com/VictoriaMetrics/easyproto"
)

// WriteRequest represents Prometheus remote write API request.
type WriteRequest struct {
	// Timeseries contains a list of time series for the given WriteRequest
	Timeseries []TimeSeries
}

// Reset resets wr for subsequent re-use.
func (wr *WriteRequest) Reset() {
	wr.Timeseries = ResetTimeSeries(wr.Timeseries)
}

// ResetTimeSeries clears all the GC references from tss and returns an empty tss ready for further use.
func ResetTimeSeries(tss []TimeSeries) []TimeSeries {
	for i := range tss {
		tss[i] = TimeSeries{}
	}
	return tss[:0]
}

// TimeSeries represents a single time series.
type TimeSeries struct {
	// Labels contains a list of labels for the given TimeSeries
	Labels []Label

	// Samples contains a list of samples for the given TimeSeries
	Samples []Sample
}

// Label represents time series label.
type Label struct {
	// Name is label name.
	Name string

	// Value is label value.
	Value string
}

// Sample represents time series sample
type Sample struct {
	// Value is sample value.
	Value float64

	// Timestamp is sample timestamp.
	Timestamp int64
}

// MarshalProtobuf appends protobuf-marshaled wr to dst and returns the result.
func (wr *WriteRequest) MarshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	wr.appendToProtobuf(m.MessageMarshaler())
	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

func (wr *WriteRequest) appendToProtobuf(mm *easyproto.MessageMarshaler) {
	// message WriteRequest {
	//    repeated TimeSeries timeseries = 1;
	// }
	tss := wr.Timeseries
	for i := range tss {
		tss[i].appendToProtobuf(mm.AppendMessage(1))
	}
}

func (ts *TimeSeries) appendToProtobuf(mm *easyproto.MessageMarshaler) {
	// message TimeSeries {
	//   repeated Label labels   = 1;
	//   repeated Sample samples = 2;
	// }
	labels := ts.Labels
	for i := range labels {
		labels[i].appendToProtobuf(mm.AppendMessage(1))
	}

	samples := ts.Samples
	for i := range samples {
		samples[i].appendToProtobuf(mm.AppendMessage(2))
	}
}

func (lbl *Label) appendToProtobuf(mm *easyproto.MessageMarshaler) {
	// message Label {
	//   string name  = 1;
	//   string value = 2;
	// }
	mm.AppendString(1, lbl.Name)
	mm.AppendString(2, lbl.Value)
}

func (s *Sample) appendToProtobuf(mm *easyproto.MessageMarshaler) {
	// message Sample {
	//   double value    = 1;
	//   int64 timestamp = 2;
	// }
	mm.AppendDouble(1, s.Value)
	mm.AppendInt64(2, s.Timestamp)
}

var mp easyproto.MarshalerPool
