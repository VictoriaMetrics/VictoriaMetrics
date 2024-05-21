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
	fs.add("")
	s := fs.String()
	if s != "[_msg,bar,foo]" {
		t.Fatalf("unexpected String() result; got %s; want %s", s, "[_msg,bar,foo]")
	}
	if !fs.contains("foo") {
		t.Fatalf("fs must contain foo")
	}
	if !fs.contains("bar") {
		t.Fatalf("fs must contain bar")
	}
	if !fs.contains("") {
		t.Fatalf("fs must contain _msg")
	}
	if !fs.contains("_msg") {
		t.Fatalf("fs must contain _msg")
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
	fs.remove("")
	if fs.contains("") {
		t.Fatalf("fs mustn't contain _msg")
	}
	if fs.contains("_msg") {
		t.Fatalf("fs mustn't contain _msg")
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

	// verify addFields, removeFields, getAll
	fs.addFields([]string{"foo", "bar", "_msg"})
	if !fs.contains("foo") || !fs.contains("bar") || !fs.contains("_msg") {
		t.Fatalf("fs must contain foo, bar and _msg")
	}
	a := fs.getAll()
	if !reflect.DeepEqual(a, []string{"_msg", "bar", "foo"}) {
		t.Fatalf("unexpected result from getAll(); got %q; want %q", a, []string{"_msg", "bar", "foo"})
	}
	fs.removeFields([]string{"bar", "baz", "_msg"})
	if fs.contains("bar") || fs.contains("baz") || fs.contains("_msg") {
		t.Fatalf("fs mustn't contain bar, baz and _msg")
	}
	if !fs.contains("foo") {
		t.Fatalf("fs must contain foo")
	}

	// verify clone
	fs.addFields([]string{"foo", "bar", "baz"})
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
