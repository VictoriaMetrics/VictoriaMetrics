package logstorage

import (
	"reflect"
	"testing"
)

func TestIndexBlockHeaderMarshalUnmarshal(t *testing.T) {
	f := func(ih *indexBlockHeader, marshaledLen int) {
		t.Helper()
		data := ih.marshal(nil)
		if len(data) != marshaledLen {
			t.Fatalf("unexpected marshaled length of indexBlockHeader; got %d; want %d", len(data), marshaledLen)
		}
		var ih2 indexBlockHeader
		tail, err := ih2.unmarshal(data)
		if err != nil {
			t.Fatalf("cannot unmarshal indexBlockHeader: %s", err)
		}
		if len(tail) > 0 {
			t.Fatalf("unexpected non-empty tail left after unmarshaling indexBlockHeader: %X", tail)
		}
		if !reflect.DeepEqual(ih, &ih2) {
			t.Fatalf("unexpected unmarshaled indexBlockHeader\ngot\n%v\nwant\n%v", &ih2, ih)
		}
	}
	f(&indexBlockHeader{}, 56)
	f(&indexBlockHeader{
		streamID: streamID{
			tenantID: TenantID{
				AccountID: 123,
				ProjectID: 456,
			},
			id: u128{
				hi: 214,
				lo: 2111,
			},
		},
		minTimestamp:     1234,
		maxTimestamp:     898943,
		indexBlockOffset: 234,
		indexBlockSize:   898,
	}, 56)
}

func TestIndexBlockHeaderUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()
		dataOrig := append([]byte{}, data...)
		var ih indexBlockHeader
		tail, err := ih.unmarshal(data)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if string(tail) != string(dataOrig) {
			t.Fatalf("unexpected tail; got %q; want %q", tail, dataOrig)
		}
	}
	f(nil)
	f([]byte("foo"))

	ih := &indexBlockHeader{
		streamID: streamID{
			tenantID: TenantID{
				AccountID: 123,
				ProjectID: 456,
			},
			id: u128{
				hi: 214,
				lo: 2111,
			},
		},
		minTimestamp:     1234,
		maxTimestamp:     898943,
		indexBlockOffset: 234,
		indexBlockSize:   898,
	}
	data := ih.marshal(nil)
	for len(data) > 0 {
		data = data[:len(data)-1]
		f(data)
	}
}

func TestIndexBlockHeaderReset(t *testing.T) {
	ih := &indexBlockHeader{
		streamID: streamID{
			tenantID: TenantID{
				AccountID: 123,
				ProjectID: 456,
			},
			id: u128{
				hi: 214,
				lo: 2111,
			},
		},
		minTimestamp:     1234,
		maxTimestamp:     898943,
		indexBlockOffset: 234,
		indexBlockSize:   898,
	}
	ih.reset()
	ihZero := &indexBlockHeader{}
	if !reflect.DeepEqual(ih, ihZero) {
		t.Fatalf("unexpected non-zero indexBlockHeader after reset: %v", ih)
	}
}

func TestMarshalUnmarshalIndexBlockHeaders(t *testing.T) {
	f := func(ihs []indexBlockHeader, marshaledLen int) {
		t.Helper()
		var data []byte
		for i := range ihs {
			data = ihs[i].marshal(data)
		}
		if len(data) != marshaledLen {
			t.Fatalf("unexpected marshaled length for indexBlockHeader entries; got %d; want %d", len(data), marshaledLen)
		}
		ihs2, err := unmarshalIndexBlockHeaders(nil, data)
		if err != nil {
			t.Fatalf("cannot unmarshal indexBlockHeader entries: %s", err)
		}
		if !reflect.DeepEqual(ihs, ihs2) {
			t.Fatalf("unexpected indexBlockHeader entries after unmarshaling\ngot\n%v\nwant\n%v", ihs2, ihs)
		}
	}
	f(nil, 0)
	f([]indexBlockHeader{{}}, 56)
	f([]indexBlockHeader{
		{
			indexBlockOffset: 234,
			indexBlockSize:   5432,
		},
		{
			minTimestamp: -123,
		},
	}, 112)
}
