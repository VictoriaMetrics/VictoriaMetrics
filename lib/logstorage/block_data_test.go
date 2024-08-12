package logstorage

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

func TestBlockDataReset(t *testing.T) {
	bd := &blockData{
		streamID: streamID{
			tenantID: TenantID{
				AccountID: 123,
				ProjectID: 432,
			},
		},
		uncompressedSizeBytes: 2344,
		rowsCount:             134,
		timestampsData: timestampsData{
			data:         []byte("foo"),
			marshalType:  encoding.MarshalTypeDeltaConst,
			minTimestamp: 1234,
			maxTimestamp: 23443,
		},
		columnsData: []columnData{
			{
				name:            "foo",
				valueType:       valueTypeUint16,
				valuesData:      []byte("aaa"),
				bloomFilterData: []byte("bsdf"),
			},
		},
		constColumns: []Field{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}
	bd.reset()
	bdZero := &blockData{
		columnsData:  []columnData{},
		constColumns: []Field{},
	}
	if !reflect.DeepEqual(bd, bdZero) {
		t.Fatalf("unexpected non-zero blockData after reset: %v", bd)
	}
}

func TestBlockDataCopyFrom(t *testing.T) {
	f := func(bd *blockData) {
		t.Helper()

		a := getArena()
		defer putArena(a)

		var bd2 blockData
		bd2.copyFrom(a, bd)
		if !reflect.DeepEqual(bd, &bd2) {
			t.Fatalf("unexpected blockData copy\ngot\n%v\nwant\n%v", &bd2, bd)
		}

		// Try copying it again to the same destination
		bd2.copyFrom(a, bd)
		if !reflect.DeepEqual(bd, &bd2) {
			t.Fatalf("unexpected blockData copy to the same destination\ngot\n%v\nwant\n%v", &bd2, bd)
		}
	}

	f(&blockData{})

	bd := &blockData{
		streamID: streamID{
			tenantID: TenantID{
				AccountID: 123,
				ProjectID: 432,
			},
		},
		uncompressedSizeBytes: 8943,
		rowsCount:             134,
		timestampsData: timestampsData{
			data:         []byte("foo"),
			marshalType:  encoding.MarshalTypeDeltaConst,
			minTimestamp: 1234,
			maxTimestamp: 23443,
		},
		columnsData: []columnData{
			{
				name:            "foo",
				valueType:       valueTypeUint16,
				valuesData:      []byte("aaa"),
				bloomFilterData: []byte("bsdf"),
			},
			{
				name:            "bar",
				valuesData:      []byte("aaa"),
				bloomFilterData: []byte("bsdf"),
			},
		},
		constColumns: []Field{
			{
				Name:  "foobar",
				Value: "baz",
			},
		},
	}
	f(bd)
}
