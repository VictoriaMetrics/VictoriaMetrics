// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package pb

import (
	"errors"
	"fmt"

	"github.com/tinylib/msgp/msgp"
)

// dictionaryString reads an int from decoder dc and returns the string
// at that index from dict.
func dictionaryString(bts []byte, dict []string) (string, []byte, error) {
	var (
		ui  uint32
		err error
	)
	ui, bts, err = msgp.ReadUint32Bytes(bts)
	if err != nil {
		return "", bts, err
	}
	idx := int(ui)
	if idx >= len(dict) {
		return "", bts, fmt.Errorf("dictionary index %d out of range", idx)
	}
	return dict[idx], bts, nil
}

// UnmarshalMsgDictionary decodes a trace using the specification from the v0.5 endpoint.
// For details, see the documentation for endpoint v0.5 in pkg/trace/api/version.go
func (t *Traces) UnmarshalMsgDictionary(bts []byte) error {
	var err error
	if _, bts, err = msgp.ReadArrayHeaderBytes(bts); err != nil {
		return err
	}
	// read dictionary
	var sz uint32
	if sz, bts, err = msgp.ReadArrayHeaderBytes(bts); err != nil {
		return err
	}
	dict := make([]string, sz)
	for i := range dict {
		var str string
		str, bts, err = parseStringBytes(bts)
		if err != nil {
			return err
		}
		dict[i] = str
	}
	// read traces
	sz, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return err
	}
	if cap(*t) >= int(sz) {
		*t = (*t)[:sz]
	} else {
		*t = make(Traces, sz)
	}
	for i := range *t {
		sz, bts, err = msgp.ReadArrayHeaderBytes(bts)
		if err != nil {
			return err
		}
		if cap((*t)[i]) >= int(sz) {
			(*t)[i] = (*t)[i][:sz]
		} else {
			(*t)[i] = make(Trace, sz)
		}
		for j := range (*t)[i] {
			if (*t)[i][j] == nil {
				(*t)[i][j] = new(Span)
			}
			if bts, err = (*t)[i][j].UnmarshalMsgDictionary(bts, dict); err != nil {
				return err
			}
		}
	}
	return nil
}

// spanPropertyCount specifies the number of top-level properties that a span
// has.
const spanPropertyCount = 12

// UnmarshalMsgDictionary decodes a span from the given decoder dc, looking up strings
// in the given dictionary dict. For details, see the documentation for endpoint v0.5
// in pkg/trace/api/version.go
func (z *Span) UnmarshalMsgDictionary(bts []byte, dict []string) ([]byte, error) {
	var (
		sz  uint32
		err error
	)
	sz, bts, err = msgp.ReadArrayHeaderBytes(bts)
	if err != nil {
		return bts, err
	}
	if sz != spanPropertyCount {
		return bts, errors.New("encoded span needs exactly 12 elements in array")
	}
	// Service (0)
	z.Service, bts, err = dictionaryString(bts, dict)
	if err != nil {
		return bts, err
	}
	// Name (1)
	z.Name, bts, err = dictionaryString(bts, dict)
	if err != nil {
		return bts, err
	}
	// Resource (2)
	z.Resource, bts, err = dictionaryString(bts, dict)
	if err != nil {
		return bts, err
	}
	// TraceID (3)
	z.TraceID, bts, err = parseUint64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// SpanID (4)
	z.SpanID, bts, err = parseUint64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// ParentID (5)
	z.ParentID, bts, err = parseUint64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// Start (6)
	z.Start, bts, err = parseInt64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// Duration (7)
	z.Duration, bts, err = parseInt64Bytes(bts)
	if err != nil {
		return bts, err
	}
	// Error (8)
	z.Error, bts, err = parseInt32Bytes(bts)
	if err != nil {
		return bts, err
	}
	// Meta (9)
	sz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return bts, err
	}
	if z.Meta == nil && sz > 0 {
		z.Meta = make(map[string]string, sz)
	} else if len(z.Meta) > 0 {
		for key := range z.Meta {
			delete(z.Meta, key)
		}
	}
	hook, hookok := MetaHook()
	for sz > 0 {
		sz--
		var key, val string
		key, bts, err = dictionaryString(bts, dict)
		if err != nil {
			return bts, err
		}
		val, bts, err = dictionaryString(bts, dict)
		if err != nil {
			return bts, err
		}
		if hookok {
			z.Meta[key] = hook(key, val)
		} else {
			z.Meta[key] = val
		}
	}
	// Metrics (10)
	sz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return bts, err
	}
	if z.Metrics == nil && sz > 0 {
		z.Metrics = make(map[string]float64, sz)
	} else if len(z.Metrics) > 0 {
		for key := range z.Metrics {
			delete(z.Metrics, key)
		}
	}
	for sz > 0 {
		sz--
		var (
			key string
			val float64
		)
		key, bts, err = dictionaryString(bts, dict)
		if err != nil {
			return bts, err
		}
		val, bts, err = parseFloat64Bytes(bts)
		if err != nil {
			return bts, err
		}
		z.Metrics[key] = val
	}
	// Type (11)
	z.Type, bts, err = dictionaryString(bts, dict)
	if err != nil {
		return bts, err
	}
	return bts, nil
}
