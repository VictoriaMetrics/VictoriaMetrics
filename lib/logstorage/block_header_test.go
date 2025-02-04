package logstorage

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

func TestBlockHeaderMarshalUnmarshal(t *testing.T) {
	f := func(bh *blockHeader, marshaledLen int) {
		t.Helper()
		data := bh.marshal(nil)
		if len(data) != marshaledLen {
			t.Fatalf("unexpected lengths of the marshaled blockHeader; got %d; want %d", len(data), marshaledLen)
		}
		bh2 := &blockHeader{}
		tail, err := bh2.unmarshal(data, partFormatLatestVersion)
		if err != nil {
			t.Fatalf("unexpected error in unmarshal: %s", err)
		}
		if len(tail) > 0 {
			t.Fatalf("unexpected non-empty tail after unmarshal: %X", tail)
		}
		if !reflect.DeepEqual(bh, bh2) {
			t.Fatalf("unexpected blockHeader unmarshaled\ngot\n%v\nwant\n%v", bh2, bh)
		}
	}
	f(&blockHeader{}, 63)
	f(&blockHeader{
		streamID: streamID{
			tenantID: TenantID{
				AccountID: 123,
				ProjectID: 456,
			},
			id: u128{
				lo: 3443,
				hi: 23434,
			},
		},
		uncompressedSizeBytes: 4344,
		rowsCount:             1234,
		timestampsHeader: timestampsHeader{
			blockOffset:  13234,
			blockSize:    8843,
			minTimestamp: -4334,
			maxTimestamp: 23434,
			marshalType:  encoding.MarshalTypeNearestDelta2,
		},
		columnsHeaderIndexOffset: 8923481,
		columnsHeaderIndexSize:   8989832,
		columnsHeaderOffset:      4384,
		columnsHeaderSize:        894,
	}, 73)
}

func TestColumnsHeaderIndexMarshalUnmarshal(t *testing.T) {
	f := func(cshIndex *columnsHeaderIndex, marshaledLen int) {
		t.Helper()

		data := cshIndex.marshal(nil)
		if len(data) != marshaledLen {
			t.Fatalf("unexpected lengths of the marshaled columnsHeader; got %d; want %d", len(data), marshaledLen)
		}
		cshIndex2 := &columnsHeaderIndex{}
		if err := cshIndex2.unmarshalNoArena(data); err != nil {
			t.Fatalf("unexpected error in unmarshal: %s", err)
		}

		if !reflect.DeepEqual(cshIndex, cshIndex2) {
			t.Fatalf("unexpected blockHeaderIndex unmarshaled\ngot\n%v\nwant\n%v", cshIndex2, cshIndex)
		}
	}

	f(&columnsHeaderIndex{}, 2)
	f(&columnsHeaderIndex{
		columnHeadersRefs: []columnHeaderRef{
			{
				columnNameID: 234,
				offset:       123432,
			},
			{
				columnNameID: 23898,
				offset:       0,
			},
		},
		constColumnsRefs: []columnHeaderRef{
			{
				columnNameID: 0,
				offset:       8989,
			},
		},
	}, 14)
}

func TestColumnsHeaderMarshalUnmarshal(t *testing.T) {
	f := func(csh *columnsHeader, marshaledLen int) {
		t.Helper()

		cshIndex := getColumnsHeaderIndex()
		g := &columnNameIDGenerator{}

		data := csh.marshal(nil, cshIndex, g)
		if len(data) != marshaledLen {
			t.Fatalf("unexpected length of the marshaled columnsHeader; got %d; want %d", len(data), marshaledLen)
		}
		csh2 := &columnsHeader{}
		if err := csh2.unmarshalNoArena(data, partFormatLatestVersion); err != nil {
			t.Fatalf("unexpected error in unmarshal: %s", err)
		}
		if err := csh2.setColumnNames(cshIndex, g.columnNames); err != nil {
			t.Fatalf("cannot set column names: %s", err)
		}

		if !reflect.DeepEqual(csh, csh2) {
			t.Fatalf("unexpected blockHeader unmarshaled\ngot\n%v\nwant\n%v", csh2, csh)
		}
	}

	f(&columnsHeader{}, 2)
	f(&columnsHeader{
		columnHeaders: []columnHeader{
			{
				name:              "foobar",
				valueType:         valueTypeString,
				valuesOffset:      12345,
				valuesSize:        23434,
				bloomFilterOffset: 89843,
				bloomFilterSize:   8934,
			},
			{
				name:              "message",
				valueType:         valueTypeUint16,
				minValue:          123,
				maxValue:          456,
				valuesOffset:      3412345,
				valuesSize:        234434,
				bloomFilterOffset: 83,
				bloomFilterSize:   34,
			},
		},
		constColumns: []Field{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}, 31)
}

func TestBlockHeaderUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()
		dataOrig := append([]byte{}, data...)
		bh := getBlockHeader()
		defer putBlockHeader(bh)
		tail, err := bh.unmarshal(data, partFormatLatestVersion)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if string(tail) != string(dataOrig) {
			t.Fatalf("unexpected tail;\ngot\n%q\nwant\n%q", tail, dataOrig)
		}
	}
	f(nil)
	f([]byte("foo"))

	bh := blockHeader{
		streamID: streamID{
			tenantID: TenantID{
				AccountID: 123,
				ProjectID: 456,
			},
			id: u128{
				lo: 3443,
				hi: 23434,
			},
		},
		uncompressedSizeBytes: 4344,
		rowsCount:             1234,
		timestampsHeader: timestampsHeader{
			blockOffset:  13234,
			blockSize:    8843,
			minTimestamp: -4334,
			maxTimestamp: 23434,
			marshalType:  encoding.MarshalTypeNearestDelta2,
		},
		columnsHeaderIndexOffset: 89434,
		columnsHeaderIndexSize:   89123,
		columnsHeaderOffset:      4384,
		columnsHeaderSize:        894,
	}
	data := bh.marshal(nil)
	for len(data) > 0 {
		data = data[:len(data)-1]
		f(data)
	}
}

func TestColumnsHeaderIndexUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()

		cshIndex := getColumnsHeaderIndex()
		defer putColumnsHeaderIndex(cshIndex)
		if err := cshIndex.unmarshalNoArena(data); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	f(nil)
	f([]byte("foo"))

	cshIndex := &columnsHeaderIndex{
		columnHeadersRefs: []columnHeaderRef{
			{
				columnNameID: 0,
				offset:       123,
			},
		},
		constColumnsRefs: []columnHeaderRef{
			{
				columnNameID: 2,
				offset:       89834,
			},
			{
				columnNameID: 234,
				offset:       8934,
			},
		},
	}
	data := cshIndex.marshal(nil)
	for len(data) > 0 {
		data = data[:len(data)-1]
		f(data)
	}
}

func TestColumnsHeaderUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()

		csh := getColumnsHeader()
		defer putColumnsHeader(csh)
		if err := csh.unmarshalNoArena(data, partFormatLatestVersion); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	f(nil)
	f([]byte("foo"))

	csh := &columnsHeader{
		columnHeaders: []columnHeader{
			{
				name:              "foobar",
				valueType:         valueTypeString,
				valuesOffset:      12345,
				valuesSize:        23434,
				bloomFilterOffset: 89843,
				bloomFilterSize:   8934,
			},
			{
				name:              "message",
				valueType:         valueTypeUint16,
				minValue:          123,
				maxValue:          456,
				valuesOffset:      3412345,
				valuesSize:        234434,
				bloomFilterOffset: 83,
				bloomFilterSize:   34,
			},
		},
		constColumns: []Field{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}
	cshIndex := getColumnsHeaderIndex()
	g := &columnNameIDGenerator{}
	data := csh.marshal(nil, cshIndex, g)
	for len(data) > 0 {
		data = data[:len(data)-1]
		f(data)
	}
	putColumnsHeaderIndex(cshIndex)
}

func TestBlockHeaderReset(t *testing.T) {
	bh := &blockHeader{
		streamID: streamID{
			tenantID: TenantID{
				AccountID: 123,
				ProjectID: 456,
			},
			id: u128{
				lo: 3443,
				hi: 23434,
			},
		},
		uncompressedSizeBytes: 8984,
		rowsCount:             1234,
		timestampsHeader: timestampsHeader{
			blockOffset:  13234,
			blockSize:    8843,
			minTimestamp: -4334,
			maxTimestamp: 23434,
			marshalType:  encoding.MarshalTypeNearestDelta2,
		},
		columnsHeaderIndexOffset: 18934,
		columnsHeaderIndexSize:   8912,
		columnsHeaderOffset:      12332,
		columnsHeaderSize:        234,
	}
	bh.reset()
	bhZero := &blockHeader{}
	if !reflect.DeepEqual(bh, bhZero) {
		t.Fatalf("unexpected non-zero blockHeader after reset: %v", bh)
	}
}

func TestColumnsHeaderIndexReset(t *testing.T) {
	cshIndex := &columnsHeaderIndex{
		columnHeadersRefs: []columnHeaderRef{
			{
				columnNameID: 234,
				offset:       1234,
			},
		},
		constColumnsRefs: []columnHeaderRef{
			{
				columnNameID: 328,
				offset:       21344,
			},
			{
				columnNameID: 1,
				offset:       234,
			},
		},
	}
	cshIndex.reset()
	cshIndexZero := &columnsHeaderIndex{
		columnHeadersRefs: []columnHeaderRef{},
		constColumnsRefs:  []columnHeaderRef{},
	}
	if !reflect.DeepEqual(cshIndex, cshIndexZero) {
		t.Fatalf("unexpected non-zero columnsHeaderIndex after reset: %v", cshIndex)
	}
}

func TestColumnsHeaderReset(t *testing.T) {
	csh := &columnsHeader{
		columnHeaders: []columnHeader{
			{
				name:              "foobar",
				valueType:         valueTypeString,
				valuesOffset:      12345,
				valuesSize:        23434,
				bloomFilterOffset: 89843,
				bloomFilterSize:   8934,
			},
			{
				name:              "message",
				valueType:         valueTypeUint16,
				minValue:          123,
				maxValue:          456,
				valuesOffset:      3412345,
				valuesSize:        234434,
				bloomFilterOffset: 83,
				bloomFilterSize:   34,
			},
		},
		constColumns: []Field{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}
	csh.reset()
	cshZero := &columnsHeader{
		columnHeaders: []columnHeader{},
		constColumns:  []Field{},
	}
	if !reflect.DeepEqual(csh, cshZero) {
		t.Fatalf("unexpected non-zero columnsHeader after reset: %v", csh)
	}
}

func TestMarshalUnmarshalBlockHeaders(t *testing.T) {
	f := func(bhs []blockHeader, marshaledLen int) {
		t.Helper()
		var data []byte
		for i := range bhs {
			data = bhs[i].marshal(data)
		}
		if len(data) != marshaledLen {
			t.Fatalf("unexpected length for marshaled blockHeader entries; got %d; want %d", len(data), marshaledLen)
		}
		bhs2, err := unmarshalBlockHeaders(nil, data, partFormatLatestVersion)
		if err != nil {
			t.Fatalf("unexpected error when unmarshaling blockHeader entries: %s", err)
		}
		if !reflect.DeepEqual(bhs, bhs2) {
			t.Fatalf("unexpected blockHeader entries unmarshaled\ngot\n%v\nwant\n%v", bhs2, bhs)
		}
	}
	f(nil, 0)
	f([]blockHeader{{}}, 63)
	f([]blockHeader{
		{},
		{
			streamID: streamID{
				tenantID: TenantID{
					AccountID: 123,
					ProjectID: 456,
				},
				id: u128{
					lo: 3443,
					hi: 23434,
				},
			},
			uncompressedSizeBytes: 89894,
			rowsCount:             1234,
			timestampsHeader: timestampsHeader{
				blockOffset:  13234,
				blockSize:    8843,
				minTimestamp: -4334,
				maxTimestamp: 23434,
				marshalType:  encoding.MarshalTypeNearestDelta2,
			},
			columnsHeaderIndexOffset: 1234,
			columnsHeaderIndexSize:   89324,
			columnsHeaderOffset:      12332,
			columnsHeaderSize:        234,
		},
	}, 134)
}

func TestColumnHeaderMarshalUnmarshal(t *testing.T) {
	f := func(ch *columnHeader, marshaledLen int) {
		t.Helper()

		data := ch.marshal(nil)
		if len(data) != marshaledLen {
			t.Fatalf("unexpected marshaled length of columnHeader; got %d; want %d", len(data), marshaledLen)
		}
		var ch2 columnHeader
		tail, err := ch2.unmarshalNoArena(data, partFormatLatestVersion)
		if err != nil {
			t.Fatalf("unexpected error in umarshal(%v): %s", ch, err)
		}
		if len(tail) > 0 {
			t.Fatalf("unexpected non-empty tail after unmarshal(%v): %X", ch, tail)
		}

		// columnHeader.name isn't marshaled, since it is marshaled via columnsHeaderIndex starting from part format v1.
		ch2.name = ch.name

		if !reflect.DeepEqual(ch, &ch2) {
			t.Fatalf("unexpected columnHeader after unmarshal;\ngot\n%v\nwant\n%v", &ch2, ch)
		}
	}

	f(&columnHeader{
		name:      "foo",
		valueType: valueTypeUint8,
	}, 7)
	ch := &columnHeader{
		name:      "foobar",
		valueType: valueTypeDict,

		valuesOffset: 12345,
		valuesSize:   254452,
	}
	ch.valuesDict.getOrAdd("abc")
	f(ch, 11)
}

func TestColumnHeaderUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()

		dataOrig := append([]byte{}, data...)
		var ch columnHeader
		tail, err := ch.unmarshalNoArena(data, partFormatLatestVersion)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if string(tail) != string(dataOrig) {
			t.Fatalf("unexpected tail left; got %q; want %q", tail, dataOrig)
		}
	}

	f(nil)
	f([]byte("foo"))

	ch := &columnHeader{
		name:            "abc",
		valueType:       valueTypeUint16,
		bloomFilterSize: 3244,
	}
	data := ch.marshal(nil)
	f(data[:len(data)-1])
}

func TestColumnHeaderReset(t *testing.T) {
	ch := &columnHeader{
		name:      "foobar",
		valueType: valueTypeUint16,

		valuesOffset: 12345,
		valuesSize:   254452,

		bloomFilterOffset: 34898234,
		bloomFilterSize:   873434,
	}
	ch.valuesDict.getOrAdd("abc")
	ch.reset()
	chZero := &columnHeader{}
	chZero.valuesDict.values = []string{}
	if !reflect.DeepEqual(ch, chZero) {
		t.Fatalf("unexpected non-zero columnHeader after reset: %v", ch)
	}
}

func TestTimestampsHeaderMarshalUnmarshal(t *testing.T) {
	f := func(th *timestampsHeader, marshaledLen int) {
		t.Helper()
		data := th.marshal(nil)
		if len(data) != marshaledLen {
			t.Fatalf("unexpected length of marshaled timestampsHeader; got %d; want %d", len(data), marshaledLen)
		}
		var th2 timestampsHeader
		tail, err := th2.unmarshal(data)
		if err != nil {
			t.Fatalf("unexpected error in unmarshal(%v): %s", th, err)
		}
		if len(tail) > 0 {
			t.Fatalf("unexpected non-nil tail after unmarshal(%v): %X", th, tail)
		}
		if !reflect.DeepEqual(th, &th2) {
			t.Fatalf("unexpected timestampsHeader after unmarshal; got\n%v\nwant\n%v", &th2, th)
		}
	}
	f(&timestampsHeader{}, 33)

	f(&timestampsHeader{
		blockOffset:  12345,
		blockSize:    3424834,
		minTimestamp: -123443,
		maxTimestamp: 234343,
		marshalType:  encoding.MarshalTypeZSTDNearestDelta,
	}, 33)
}

func TestTimestampsHeaderUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()
		dataOrig := append([]byte{}, data...)
		var th timestampsHeader
		tail, err := th.unmarshal(data)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if string(tail) != string(dataOrig) {
			t.Fatalf("unexpected tail left; got %q; want %q", tail, dataOrig)
		}
	}
	f(nil)
	f([]byte("foo"))
}

func TestTimestampsHeaderReset(t *testing.T) {
	th := &timestampsHeader{
		blockOffset:  12345,
		blockSize:    3424834,
		minTimestamp: -123443,
		maxTimestamp: 234343,
		marshalType:  encoding.MarshalTypeZSTDNearestDelta,
	}
	th.reset()
	thZero := &timestampsHeader{}
	if !reflect.DeepEqual(th, thZero) {
		t.Fatalf("unexpected non-zero timestampsHeader after reset: %v", th)
	}
}
