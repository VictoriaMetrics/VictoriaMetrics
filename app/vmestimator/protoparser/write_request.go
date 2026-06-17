package protoparser

import (
	"fmt"
	"slices"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/easyproto"
	"github.com/cespare/xxhash/v2"
)

type TimeSerie struct {
	GroupLabels []Label
	Fingerprint uint64
}

type Label struct {
	Name  string
	Value string
}

func getWriteRequestUnmarshaler() *writeRequestUnmarshaler {
	v := wruPool.Get()
	if v == nil {
		return &writeRequestUnmarshaler{
			tss:        make([]TimeSerie, 0, 1024),
			labelsPool: make([]Label, 0, 4096),
			d:          xxhash.New(),
		}
	}
	return v.(*writeRequestUnmarshaler)
}

func putWriteRequestUnmarshaler(wru *writeRequestUnmarshaler) {
	wru.Reset()
	wruPool.Put(wru)
}

var wruPool sync.Pool

// WriteRequestUnmarshaler is reusable unmarshaler for WriteRequest protobuf messages.
//
// It maintains internal pools for labels and samples to reduce memory allocations.
// See UnmarshalProtobuf for details on how to use it.
type writeRequestUnmarshaler struct {
	tss        []TimeSerie
	labelsPool []Label
	d          *xxhash.Digest
}

// Reset resets wru, so it could be re-used.
func (wru *writeRequestUnmarshaler) Reset() {
	wru.tss = wru.tss[:0]
	wru.labelsPool = wru.labelsPool[:0]
	wru.d.Reset()
}

func (wru *writeRequestUnmarshaler) UnmarshalProtobuf(src []byte, groupLabels []string, callback func(tss []TimeSerie)) error {
	wru.Reset()

	var err error

	tss := wru.tss

	// message WriteRequest {
	//    repeated TimeSeries timeseries = 1;
	//    reserved 2;
	//    repeated Metadata metadata = 3;
	// }
	labelsPool := wru.labelsPool
	var fc easyproto.FieldContext
	for len(src) > 0 {
		if len(tss) >= cap(tss) {
			callback(tss)
			tss = tss[:0]
			labelsPool = labelsPool[:0]
		}

		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read timeseries data")
			}
			tss = tss[:len(tss)+1]
			ts := &tss[len(tss)-1]
			d := wru.d
			d.Reset()
			labelsPool, err = ts.unmarshalProtobuf(data, groupLabels, labelsPool, d)
			if err != nil {
				return fmt.Errorf("cannot unmarshal timeseries: %w", err)
			}
		}
	}

	if len(tss) > 0 {
		callback(tss)
		tss = tss[:0]
		labelsPool = labelsPool[:0]
	}

	wru.tss = tss[:0]
	wru.labelsPool = labelsPool
	wru.d.Reset()
	return nil
}

func (ts *TimeSerie) unmarshalProtobuf(src []byte, groupLabels []string, labelsPool []Label, d *xxhash.Digest) ([]Label, error) {
	// message TimeSeries {
	//   repeated Label labels   = 1;
	//   repeated Sample samples = 2;
	// }

	labelsPoolLen := len(labelsPool)
	var fc easyproto.FieldContext
	var nameBytes, valueBytes []byte
	var lfc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return labelsPool, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return labelsPool, fmt.Errorf("cannot read label data")
			}

			ldata := data
			for len(ldata) > 0 {
				ldata, err = lfc.NextField(ldata)
				if err != nil {
					return labelsPool, fmt.Errorf("cannot read label field: %w", err)
				}
				switch lfc.FieldNum {
				case 1:
					nameBytes, ok = lfc.Bytes()
					if !ok {
						return labelsPool, fmt.Errorf("cannot read label name")
					}
				case 2:
					valueBytes, ok = lfc.Bytes()
					if !ok {
						return labelsPool, fmt.Errorf("cannot read label value")
					}
				}
			}

			_, _ = d.Write(data)

			name := bytesutil.ToUnsafeString(nameBytes)
			if slices.Contains(groupLabels, name) {
				labelsPool = append(labelsPool, Label{
					Name:  name,
					Value: bytesutil.ToUnsafeString(valueBytes),
				})
			}
		}
	}
	ts.GroupLabels = labelsPool[labelsPoolLen:]
	ts.Fingerprint = d.Sum64()
	return labelsPool, nil
}
