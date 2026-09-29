package newrelic

import (
	"fmt"

	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// Rows contains rows parsed from NewRelic Event request
//
// See https://docs.newrelic.com/docs/infrastructure/manage-your-data/data-instrumentation/default-infrastructure-monitoring-data/#infrastructure-events
type Rows struct {
	Rows []Row
}

// maxRowsBatchSize limits the estimated memory size of rows accumulated before
// they are passed to the callback.
const maxRowsBatchSize = 4 * 1024 * 1024

// Reset resets r, so it can be reused
func (r *Rows) Reset() {
	rows := r.Rows
	for i := range rows {
		rows[i].reset()
	}
	r.Rows = rows[:0]
}

var jsonParserPool fastjson.ParserPool

// UnmarshalWithCallback parses NewRelic Event request from b and calls callback
// for every batch of parsed rows.
//
// callback shouldn't hold rows after returning.
//
// b can be reused after returning from r.
func (r *Rows) UnmarshalWithCallback(b []byte, callback func(rows []Row) error) error {
	return r.unmarshal(b, maxRowsBatchSize, callback)
}

func (r *Rows) unmarshal(b []byte, maxBatchSize int, callback func(rows []Row) error) error {
	p := jsonParserPool.Get()
	defer jsonParserPool.Put(p)

	r.Reset()
	v, err := p.ParseBytes(b)
	if err != nil {
		return err
	}
	metricPosts, err := v.Array()
	if err != nil {
		return fmt.Errorf("cannot find the top-level array of MetricPost objects: %w", err)
	}
	batchSize := 0
	var callbackErr error
	for _, mp := range metricPosts {
		o, err := mp.Object()
		if err != nil {
			return fmt.Errorf("cannot find MetricPost object: %w", err)
		}
		rows := r.Rows
		o.Visit(func(k []byte, v *fastjson.Value) {
			if err != nil {
				return
			}
			switch string(k) {
			case "Events":
				events, errLocal := v.Array()
				if errLocal != nil {
					err = fmt.Errorf("cannot find Events array in MetricPost object: %w", errLocal)
					return
				}
				for _, e := range events {
					eventObject, errLocal := e.Object()
					if errLocal != nil {
						err = fmt.Errorf("cannot find EventObject: %w", errLocal)
						return
					}
					if cap(rows) > len(rows) {
						rows = rows[:len(rows)+1]
					} else {
						rows = append(rows, Row{})
					}
					row := &rows[len(rows)-1]
					rowSize, errLocal := row.unmarshal(eventObject)
					if errLocal != nil {
						err = fmt.Errorf("cannot unmarshal EventObject: %w", errLocal)
						return
					}
					batchSize += rowSize
					if batchSize > maxBatchSize {
						if errLocal := callback(rows); errLocal != nil {
							callbackErr = errLocal
							err = callbackErr
							return
						}
						// There is no need to reset rows, since row.unmarshal resets every row before reuse.
						rows = rows[:0]
						batchSize = 0
					}
				}
			}
		})
		r.Rows = rows
		if callbackErr != nil {
			return callbackErr
		}
		if err != nil {
			return fmt.Errorf("cannot parse MetricPost object: %w", err)
		}
	}
	if len(r.Rows) > 0 {
		if err := callback(r.Rows); err != nil {
			return err
		}
	}
	return nil
}

// Row represents parsed row
type Row struct {
	Tags      []Tag
	Samples   []Sample
	Timestamp int64
}

// Tag represents a key=value tag
type Tag struct {
	Key   []byte
	Value []byte
}

// Sample represents parsed sample
type Sample struct {
	Name  []byte
	Value float64
}

func (r *Row) reset() {
	tags := r.Tags
	for i := range tags {
		tags[i].reset()
	}
	r.Tags = tags[:0]

	samples := r.Samples
	for i := range samples {
		samples[i].reset()
	}
	r.Samples = samples[:0]

	r.Timestamp = 0
}

func (t *Tag) reset() {
	t.Key = t.Key[:0]
	t.Value = t.Value[:0]
}

func (s *Sample) reset() {
	s.Name = s.Name[:0]
	s.Value = 0
}

// unmarshal parses r from o and returns the approximate size of the parsed data in bytes.
func (r *Row) unmarshal(o *fastjson.Object) (sizeBytes int, err error) {
	r.reset()
	tags := r.Tags[:0]
	samples := r.Samples[:0]
	o.Visit(func(k []byte, v *fastjson.Value) {
		if err != nil {
			return
		}
		if len(k) == 0 {
			return
		}
		switch v.Type() {
		case fastjson.TypeString:
			// Register new tag
			valueBytes := v.GetStringBytes()
			if len(valueBytes) == 0 {
				return
			}
			if cap(tags) > len(tags) {
				tags = tags[:len(tags)+1]
			} else {
				tags = append(tags, Tag{})
			}
			t := &tags[len(tags)-1]
			t.Key = append(t.Key[:0], k...)
			t.Value = append(t.Value[:0], valueBytes...)
			sizeBytes += len(k) + len(valueBytes)
		case fastjson.TypeNumber:
			if string(k) == "timestamp" {
				// Parse timestamp
				ts, errLocal := getFloat64(v)
				if errLocal != nil {
					err = fmt.Errorf("cannot parse `timestamp` field: %w", errLocal)
					return
				}
				if ts < (1 << 32) {
					// The timestamp is in seconds. Convert it to milliseconds.
					ts *= 1e3
				}
				r.Timestamp = int64(ts)
				return
			}
			// Register new sample
			if cap(samples) > len(samples) {
				samples = samples[:len(samples)+1]
			} else {
				samples = append(samples, Sample{})
			}
			s := &samples[len(samples)-1]
			s.Name = append(s.Name[:0], k...)
			s.Value = v.GetFloat64()
			sizeBytes += len(k) + 8
		}
	})
	r.Tags = tags
	r.Samples = samples
	return sizeBytes, err
}

func getFloat64(v *fastjson.Value) (float64, error) {
	switch v.Type() {
	case fastjson.TypeNumber:
		return v.Float64()
	case fastjson.TypeString:
		vStr, _ := v.StringBytes()
		vFloat, err := fastfloat.Parse(bytesutil.ToUnsafeString(vStr))
		if err != nil {
			return 0, fmt.Errorf("cannot parse value %q: %w", vStr, err)
		}
		return vFloat, nil
	default:
		return 0, fmt.Errorf("value doesn't contain float64; it contains %s", v.Type())
	}
}
