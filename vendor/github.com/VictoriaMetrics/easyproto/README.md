[![GoDoc](https://godoc.org/github.com/VictoriaMetrics/easyproto?status.svg)](http://godoc.org/github.com/VictoriaMetrics/easyproto)

# easyproto

Package [github.com/VictoriaMetrics/easyproto](http://godoc.org/github.com/VictoriaMetrics/easyproto) provides simple building blocks
for marshaling and unmarshaling of [protobuf](https://protobuf.dev/) messages with [proto3 encoding](https://protobuf.dev/programming-guides/encoding/).

## Features

- There is no need in [protoc](https://grpc.io/docs/protoc-installation/) or [go generate](https://go.dev/blog/generate) -
  just write simple maintainable code for marshaling and unmarshaling protobuf messages.
- `easyproto` doesn't increase your binary size by tens of megabytes unlike traditional `protoc`-combiled code may do.
- `easyproto` allows writing zero-alloc code for marshaling and unmarshaling of arbitrary complex protobuf messages. See [examples](#examples).

## Restrictions

- It supports only [proto3 encoding](https://protobuf.dev/programming-guides/encoding/), e.g. it doesn't support `proto2` encoding
  features such as [proto2 groups](https://protobuf.dev/programming-guides/proto2/#groups).
- It doesn't provide helpers for marshaling and unmarshaling of [well-known types](https://protobuf.dev/reference/protobuf/google.protobuf/),
  since they aren't used too much in practice.

## Examples

Suppose you need marshaling and unmarshaling of the following `timeseries` message:

```proto
message timeseries {
  string name = 1;
  repeated sample samples = 2;
}

message sample {
  double value = 1;
  int64 timestamp = 2;
}
```

At first let's create the corresponding data structures in Go:

```go
type Timeseries struct {
	Name    string
	Samples []Sample
}

type Sample struct {
	Value     float64
	Timestamp int64
}
```

Since you write the code on yourself without any `go generate` and `protoc` invocations,
you are free to use arbitrary fields and methods in these structs. You can also specify the most suitable types for these fields.
For example, the `Sample` struct may be written as the following if you need an ability to detect empty values and timestamps:

```go
type Sample struct {
	Value     *float64
	Timestamp *int64
}
```

* [How to marshal `Timeseries` struct to protobuf message](#marshaling)
* [How to unmarshal protobuf message to `Timeseries` struct](#unmarshaling)

### Marshaling

The following code can be used for marshaling `Timeseries` struct to protobuf message:

```go
import (
	"github.com/VictoriaMetrics/easyproto"
)

// MarshalProtobuf marshals ts into protobuf message, appends this message to dst and returns the result.
//
// This function doesn't allocate memory on repeated calls.
func (ts *Timeseries) MarshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	ts.marshalProtobuf(m.MessageMarshaler())
	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

func (ts *Timeseries) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, ts.Name)
	for _, s := range ts.Samples {
		s.marshalProtobuf(mm.AppendMessage(2))
	}
}

func (s *Sample) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendDouble(1, s.Value)
	mm.AppendInt64(2, s.Timestamp)
}

var mp easyproto.MarshalerPool
```

Note that you are free to modify this code according to your needs, since you write and maintain it.
For example, you can construct arbitrary protobuf messages on the fly without the need to prepare the source struct for marshaling:

```go
func CreateProtobufMessageOnTheFly() []byte {
	// Dynamically construct timeseries message with 10 samples
	var m easyproto.Marshaler
	mm := m.MessageMarshaler()
	mm.AppendString(1, "foo")
	for i := 0; i < 10; i++ {
		mmSample := mm.AppendMessage(2)
		mmSample.AppendDouble(1, float64(i)/10)
		mmSample.AppendInt64(2, int64(i)*1000)
	}
	return m.Marshal(nil)
}
```

This may be useful in tests.

### Unmarshaling

The following code can be used for unmarshaling [`timeseries` message](#examples) into `Timeseries` struct:

```go
// UnmarshalProtobuf unmarshals ts from protobuf message at src.
func (ts *Timeseries) UnmarshalProtobuf(src []byte) (err error) {
	// Set default Timeseries values
	ts.Name = ""
	ts.Samples = ts.Samples[:0]

	// Parse Timeseries message at src
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Timeseries message")
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Timeseries name")
			}
			// name refers to src. This means that the name changes when src changes.
			// Make a copy with strings.Clone(name) if needed.
			ts.Name = name
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Timeseries sample data")
			}
			ts.Samples = append(ts.Samples, Sample{})
			s := &ts.Samples[len(ts.Samples)-1]
			if err := s.UnmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal sample: %w", err)
			}
		}
	}
	return nil
}

// UnmarshalProtobuf unmarshals s from protobuf message at src.
func (s *Sample) UnmarshalProtobuf(src []byte) (err error) {
	// Set default Sample values
	s.Value = 0
	s.Timestamp = 0

	// Parse Sample message at src
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in sample")
		}
		switch fc.FieldNum {
		case 1:
			value, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read sample value")
			}
			s.Value = value
		case 2:
			timestamp, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read sample timestamp")
			}
			s.Timestamp = timestamp
		}
	}
	return nil
}
```

You are free to modify this code according to your needs, since you wrote it and you maintain it.

It is possible to extract the needed data from arbitrary protobuf messages without the need to create a destination struct.
For example, the following code extracts `timeseries` name from protobuf message, while ignoring all the other fields:

```go
func GetTimeseriesName(src []byte) (name string, err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if src != nil {
			return "", fmt.Errorf("cannot read the next field")
		}
		if fc.FieldNum == 1 {
			name, ok := fc.String()
			if !ok {
				return "", fmt.Errorf("cannot read timeseries name")
			}
			// Return a copy of name, since name refers to src.
			return strings.Clone(name), nil
		}
	}
	return "", fmt.Errorf("timeseries name isn't found in the message")
}
```

## Users

`easyproto` is used in the following projects:

- [VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics)
