package logstorage

import (
	"reflect"
	"testing"
)

func TestFieldsSet(t *testing.T) {
	fs := newFieldsSet()

	// verify add, remove and contains
	if fs.contains("*") {
		t.Fatalf("fs mustn't contain *")
	}
	if fs.contains("foo") {
		t.Fatalf("fs musn't contain foo")
	}
	fs.add("foo")
	fs.add("bar")
	s := fs.String()
	if s != "[bar,foo]" {
		t.Fatalf("unexpected String() result; got %s; want %s", s, "[bar,foo]")
	}
	if !fs.contains("foo") {
		t.Fatalf("fs must contain foo")
	}
	if !fs.contains("bar") {
		t.Fatalf("fs must contain bar")
	}
	if fs.contains("baz") {
		t.Fatalf("fs musn't contain baz")
	}
	if fs.contains("*") {
		t.Fatalf("fs mustn't contain *")
	}
	fs.remove("foo")
	if fs.contains("foo") {
		t.Fatalf("fs mustn't contain foo")
	}
	fs.remove("bar")
	if fs.contains("bar") {
		t.Fatalf("fs mustn't contain bar")
	}

	// verify *
	fs.add("*")
	if !fs.contains("*") {
		t.Fatalf("fs must contain *")
	}
	if !fs.contains("foo") || !fs.contains("bar") || !fs.contains("baz") {
		t.Fatalf("fs must contain anything")
	}
	fs.remove("foo")
	if !fs.contains("foo") {
		t.Fatalf("fs must contain anything")
	}
	fs.remove("*")
	if fs.contains("foo") || fs.contains("bar") || fs.contains("baz") {
		t.Fatalf("fs must be empty")
	}

	// verify addAll, getAll, removeAll
	fs.addAll([]string{"foo", "bar"})
	if !fs.contains("foo") || !fs.contains("bar") {
		t.Fatalf("fs must contain foo and bar")
	}
	a := fs.getAll()
	if !reflect.DeepEqual(a, []string{"bar", "foo"}) {
		t.Fatalf("unexpected result from getAll(); got %q; want %q", a, []string{"bar", "foo"})
	}
	fs.removeAll([]string{"bar", "baz"})
	if fs.contains("bar") || fs.contains("baz") {
		t.Fatalf("fs mustn't contain bar and baz")
	}
	if !fs.contains("foo") {
		t.Fatalf("fs must contain foo")
	}

	// verify clone
	fs.addAll([]string{"foo", "bar", "baz"})
	fsStr := fs.String()
	fsCopy := fs.clone()
	fsCopyStr := fsCopy.String()
	if fsStr != fsCopyStr {
		t.Fatalf("unexpected clone result; got %s; want %s", fsCopyStr, fsStr)
	}
	fsCopy.remove("foo")
	if fsCopy.contains("foo") {
		t.Fatalf("fsCopy mustn't contain foo")
	}
	if !fs.contains("foo") {
		t.Fatalf("fs must contain foo")
	}
}
