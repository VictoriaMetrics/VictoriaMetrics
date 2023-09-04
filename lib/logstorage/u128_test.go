package logstorage

import (
	"reflect"
	"testing"
)

func TestU128MarshalUnmarshal(t *testing.T) {
	f := func(u *u128, marshaledLen int) {
		t.Helper()
		data := u.marshal(nil)
		if len(data) != marshaledLen {
			t.Fatalf("unexpected length of marshaled u128; got %d; want %d", len(data), marshaledLen)
		}
		var u2 u128
		tail, err := u2.unmarshal(data)
		if err != nil {
			t.Fatalf("unexpected error at unmarshal(%s): %s", u, err)
		}
		if len(tail) != 0 {
			t.Fatalf("unexpected non-emtpy tail after unmarshal(%s): %X", u, tail)
		}
		if !reflect.DeepEqual(u, &u2) {
			t.Fatalf("unexpected value obtained from unmarshal(%s); got %s; want %s", u, &u2, u)
		}
		s1 := u.String()
		s2 := u2.String()
		if s1 != s2 {
			t.Fatalf("unexpected string representation after unmarshal; got %s; want %s", s2, s1)
		}
	}
	f(&u128{}, 16)
	f(&u128{
		hi: 123,
		lo: 456,
	}, 16)
}

func TestU128UnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()
		dataOrig := append([]byte{}, data...)
		var u u128
		tail, err := u.unmarshal(data)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if string(tail) != string(dataOrig) {
			t.Fatalf("unexpected tail; got %q; want %q", tail, dataOrig)
		}
	}
	f(nil)
	f([]byte("foo"))
}

func TestU128LessEqual(t *testing.T) {
	// compare equal values
	u1 := &u128{}
	u2 := &u128{}
	if u1.less(u2) {
		t.Fatalf("less for equal values must return false")
	}
	if u2.less(u1) {
		t.Fatalf("less for equal values must return false")
	}
	if !u1.equal(u2) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", u1, u2)
	}
	if !u2.equal(u1) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", u2, u1)
	}

	u1 = &u128{
		hi: 123,
		lo: 456,
	}
	u2 = &u128{
		hi: 123,
		lo: 456,
	}
	if u1.less(u2) {
		t.Fatalf("less for equal values must return false")
	}
	if u2.less(u1) {
		t.Fatalf("less for equal values must return false")
	}
	if !u1.equal(u2) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", u1, u2)
	}
	if !u2.equal(u1) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", u2, u1)
	}

	// compare unequal values
	u1 = &u128{
		lo: 456,
	}
	u2 = &u128{
		hi: 123,
	}
	if !u1.less(u2) {
		t.Fatalf("unexpected result for less(%s, %s); got false; want true", u1, u2)
	}
	if u2.less(u1) {
		t.Fatalf("unexpected result for less(%s, %s); got true; want false", u2, u1)
	}
	if u1.equal(u2) {
		t.Fatalf("unexpected result for equal(%s, %s); got true; want false", u1, u2)
	}

	u1 = &u128{
		hi: 123,
	}
	u2 = &u128{
		hi: 123,
		lo: 456,
	}
	if !u1.less(u2) {
		t.Fatalf("unexpected result for less(%s, %s); got false; want true", u1, u2)
	}
	if u2.less(u1) {
		t.Fatalf("unexpected result for less(%s, %s); got true; want false", u2, u1)
	}
	if u1.equal(u2) {
		t.Fatalf("unexpected result for equal(%s, %s); got true; want false", u1, u2)
	}
}
