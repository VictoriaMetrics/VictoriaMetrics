package promutils

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	if _, err := ParseDuration("foobar"); err == nil {
		t.Fatalf("expecting error for invalid duration")
	}
	dNative, err := ParseDuration("1w")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if dNative != 7*24*time.Hour {
		t.Fatalf("unexpected duration; got %s; want %s", dNative, 7*24*time.Hour)
	}
	d := NewDuration(dNative)
	if d.Duration() != dNative {
		t.Fatalf("unexpected duration; got %s; want %s", d.Duration(), dNative)
	}
	v, err := d.MarshalYAML()
	if err != nil {
		t.Fatalf("unexpected error in MarshalYAML(): %s", err)
	}
	sExpected := "168h0m0s"
	if s := v.(string); s != sExpected {
		t.Fatalf("unexpected value from MarshalYAML(); got %q; want %q", s, sExpected)
	}
	if err := d.UnmarshalYAML(func(v interface{}) error {
		sp := v.(*string)
		s := "1w3d5h"
		*sp = s
		return nil
	}); err != nil {
		t.Fatalf("unexpected error in UnmarshalYAML(): %s", err)
	}
	if dNative := d.Duration(); dNative != (10*24+5)*time.Hour {
		t.Fatalf("unexpected value; got %s; want %s", dNative, (10*24+5)*time.Hour)
	}
}
