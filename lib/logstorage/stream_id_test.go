package logstorage

import (
	"reflect"
	"testing"
)

func TestStreamIDMarshalUnmarshalString(t *testing.T) {
	f := func(sid *streamID, resultExpected string) {
		t.Helper()

		result := string(sid.marshalString(nil))

		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%q\nwant\n%q", result, resultExpected)
		}

		var sid2 streamID
		if !sid2.tryUnmarshalFromString(result) {
			t.Fatalf("cannot unmarshal streamID from %q", result)
		}

		result2 := string(sid2.marshalString(nil))
		if result != result2 {
			t.Fatalf("unexpected marshaled streamID; got %s; want %s", result2, result)
		}
	}

	f(&streamID{}, "000000000000000000000000000000000000000000000000")
	f(&streamID{
		tenantID: TenantID{
			AccountID: 123,
			ProjectID: 456,
		},
		id: u128{
			lo: 89,
			hi: 344334,
		},
	}, "0000007b000001c8000000000005410e0000000000000059")
}

func TestStreamIDMarshalUnmarshal(t *testing.T) {
	f := func(sid *streamID, marshaledLen int) {
		t.Helper()
		data := sid.marshal(nil)
		if len(data) != marshaledLen {
			t.Fatalf("unexpected length of marshaled streamID; got %d; want %d", len(data), marshaledLen)
		}
		var sid2 streamID
		tail, err := sid2.unmarshal(data)
		if err != nil {
			t.Fatalf("unexpected error on unmarshal(%s): %s", sid, err)
		}
		if len(tail) != 0 {
			t.Fatalf("unexpected non-empty tail on unmarshal(%s): %X", sid, tail)
		}
		if !reflect.DeepEqual(sid, &sid2) {
			t.Fatalf("unexpected result on unmarshal; got %s; want %s", &sid2, sid)
		}
		s1 := sid.String()
		s2 := sid2.String()
		if s1 != s2 {
			t.Fatalf("unexpected string result on unmarshal; got %s; want %s", s2, s1)
		}
	}
	f(&streamID{}, 24)
	f(&streamID{
		tenantID: TenantID{
			AccountID: 123,
			ProjectID: 456,
		},
		id: u128{
			lo: 89,
			hi: 344334,
		},
	}, 24)
}

func TestStreamIDUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()
		dataOrig := append([]byte{}, data...)
		var sid streamID
		tail, err := sid.unmarshal(data)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if string(tail) != string(dataOrig) {
			t.Fatalf("unexpected tail; got %q; want %q", tail, dataOrig)
		}
	}
	f(nil)
	f([]byte("foo"))
	f([]byte("1234567890"))
}

func TestStreamIDLessEqual(t *testing.T) {
	// compare equal values
	sid1 := &streamID{}
	sid2 := &streamID{}
	if sid1.less(sid2) {
		t.Fatalf("less for equal values must return false")
	}
	if sid2.less(sid1) {
		t.Fatalf("less for equal values must return false")
	}
	if !sid1.equal(sid2) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", sid1, sid2)
	}
	if !sid2.equal(sid1) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", sid2, sid1)
	}

	sid1 = &streamID{
		tenantID: TenantID{
			AccountID: 1,
			ProjectID: 2,
		},
		id: u128{
			hi: 123,
			lo: 456,
		},
	}
	sid2 = &streamID{
		tenantID: TenantID{
			AccountID: 1,
			ProjectID: 2,
		},
		id: u128{
			hi: 123,
			lo: 456,
		},
	}
	if sid1.less(sid2) {
		t.Fatalf("less for equal values must return false")
	}
	if sid2.less(sid1) {
		t.Fatalf("less for equal values must return false")
	}
	if !sid1.equal(sid2) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", sid1, sid2)
	}
	if !sid2.equal(sid1) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", sid2, sid1)
	}

	// compare unequal values
	sid1 = &streamID{
		id: u128{
			lo: 456,
		},
	}
	sid2 = &streamID{
		id: u128{
			hi: 123,
		},
	}
	if !sid1.less(sid2) {
		t.Fatalf("unexpected result for less(%s, %s); got false; want true", sid1, sid2)
	}
	if sid2.less(sid1) {
		t.Fatalf("unexpected result for less(%s, %s); got true; want false", sid2, sid1)
	}
	if sid1.equal(sid2) {
		t.Fatalf("unexpected result for equal(%s, %s); got true; want false", sid1, sid2)
	}

	sid1 = &streamID{
		id: u128{
			hi: 123,
			lo: 456,
		},
	}
	sid2 = &streamID{
		tenantID: TenantID{
			AccountID: 123,
		},
	}
	if !sid1.less(sid2) {
		t.Fatalf("unexpected result for less(%s, %s); got false; want true", sid1, sid2)
	}
	if sid2.less(sid1) {
		t.Fatalf("unexpected result for less(%s, %s); got true; want false", sid2, sid1)
	}
	if sid1.equal(sid2) {
		t.Fatalf("unexpected result for equal(%s, %s); got true; want false", sid1, sid2)
	}
}

func TestStreamIDReset(t *testing.T) {
	sid := &streamID{
		tenantID: TenantID{
			AccountID: 123,
			ProjectID: 456,
		},
		id: u128{
			hi: 234,
			lo: 9843,
		},
	}
	sid.reset()
	sidZero := &streamID{}
	if !reflect.DeepEqual(sid, sidZero) {
		t.Fatalf("non-zero streamID after reset(): %s", sid)
	}
}
