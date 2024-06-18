package logstorage

import (
	"reflect"
	"testing"
)

func TestTenantIDMarshalUnmarshal(t *testing.T) {
	f := func(tid *TenantID) {
		t.Helper()
		data := tid.marshal(nil)
		var tid2 TenantID
		tail, err := tid2.unmarshal(data)
		if err != nil {
			t.Fatalf("unexpected error at unmarshal(%s): %s", tid, err)
		}
		if len(tail) != 0 {
			t.Fatalf("unexpected non-emtpy tail after unmarshal(%s): %X", tid, tail)
		}
		if !reflect.DeepEqual(tid, &tid2) {
			t.Fatalf("unexpected value after unmarshal; got %s; want %s", &tid2, tid)
		}
		s1 := tid.String()
		s2 := tid2.String()
		if s1 != s2 {
			t.Fatalf("unexpected string value after unmarshal; got %s; want %s", s2, s1)
		}
	}
	f(&TenantID{})
	f(&TenantID{
		AccountID: 123,
		ProjectID: 456,
	})
}

func TestTenantIDUnmarshalFailure(t *testing.T) {
	f := func(data []byte) {
		t.Helper()
		dataOrig := append([]byte{}, data...)
		var tid TenantID
		tail, err := tid.unmarshal(data)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if string(tail) != string(dataOrig) {
			t.Fatalf("unexpected tail; got %q; want %q", tail, dataOrig)
		}
	}
	f(nil)
	f([]byte("abc"))
}

func TestTenantIDLessEqual(t *testing.T) {
	// compare equal values
	tid1 := &TenantID{}
	tid2 := &TenantID{}
	if tid1.less(tid2) {
		t.Fatalf("less for equal values must return false")
	}
	if tid2.less(tid1) {
		t.Fatalf("less for equal values must return false")
	}
	if !tid1.equal(tid2) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", tid1, tid2)
	}
	if !tid2.equal(tid1) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", tid2, tid1)
	}

	tid1 = &TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	tid2 = &TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	if tid1.less(tid2) {
		t.Fatalf("less for equal values must return false")
	}
	if tid2.less(tid1) {
		t.Fatalf("less for equal values must return false")
	}
	if !tid1.equal(tid2) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", tid1, tid2)
	}
	if !tid2.equal(tid1) {
		t.Fatalf("unexpected equal(%s, %s) result; got false; want true", tid2, tid1)
	}

	// compare unequal values
	tid1 = &TenantID{
		ProjectID: 456,
	}
	tid2 = &TenantID{
		AccountID: 123,
	}
	if !tid1.less(tid2) {
		t.Fatalf("unexpected result for less(%s, %s); got false; want true", tid1, tid2)
	}
	if tid2.less(tid1) {
		t.Fatalf("unexpected result for less(%s, %s); got true; want false", tid2, tid1)
	}
	if tid1.equal(tid2) {
		t.Fatalf("unexpected result for equal(%s, %s); got true; want false", tid1, tid2)
	}

	tid1 = &TenantID{
		AccountID: 123,
	}
	tid2 = &TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	if !tid1.less(tid2) {
		t.Fatalf("unexpected result for less(%s, %s); got false; want true", tid1, tid2)
	}
	if tid2.less(tid1) {
		t.Fatalf("unexpected result for less(%s, %s); got true; want false", tid2, tid1)
	}
	if tid1.equal(tid2) {
		t.Fatalf("unexpected result for equal(%s, %s); got true; want false", tid1, tid2)
	}
}

func TestParseTenantID(t *testing.T) {
	f := func(tenant string, expected TenantID) {
		t.Helper()

		got, err := ParseTenantID(tenant)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
			return
		}

		if got.String() != expected.String() {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	}

	f("", TenantID{})
	f("123", TenantID{AccountID: 123})
	f("123:456", TenantID{AccountID: 123, ProjectID: 456})
	f("123:", TenantID{AccountID: 123})
	f(":456", TenantID{ProjectID: 456})
}
