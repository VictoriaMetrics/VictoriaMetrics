package flagutil

import (
	"path/filepath"
	"testing"
)

func TestURL(t *testing.T) {
	s := URL{
		flagname: "url-foo",
	}

	// Verify that String returns "secret"
	expectedSecret := "secret"
	if str := s.String(); str != expectedSecret {
		t.Fatalf("unexpected value returned from URL.String; got %q; want %q", str, expectedSecret)
	}

	// set regular url
	expectedURL := "http://usr:pass@localhost:8428/snapshot/create?authKey=foobar"
	if err := s.Set(expectedURL); err != nil {
		t.Fatalf("cannot set url: %s", err)
	}
	for i := 0; i < 5; i++ {
		if str := s.Get(); str != expectedURL {
			t.Fatalf("unexpected url; got %q; want %q", str, expectedURL)
		}
		if str := s.String(); str != expectedSecret {
			t.Fatalf("unexpected value returned from URL.String; got %q; want %q", str, expectedSecret)
		}
	}

	// read the url from file by relative path
	localURLFile := "testdata/url.txt"
	expectedURL = "http://usr:pass@localhost:8428/snapshot/create?authKey=foobar"
	path := "file://" + localURLFile
	if err := s.Set(path); err != nil {
		t.Fatalf("cannot set url to file: %s", err)
	}
	for i := 0; i < 5; i++ {
		if str := s.Get(); str != expectedURL {
			t.Fatalf("unexpected url; got %q; want %q", str, expectedURL)
		}
		if str := s.String(); str != expectedSecret {
			t.Fatalf("unexpected value returned from URL.String; got %q; want %q", str, expectedSecret)
		}
	}

	// read the url from file by absolute path
	var err error
	localURLFile, err = filepath.Abs("testdata/url.txt")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedURL = "http://usr:pass@localhost:8428/snapshot/create?authKey=foobar"
	path = "file://" + localURLFile
	if err := s.Set(path); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	for i := 0; i < 5; i++ {
		if str := s.Get(); str != expectedURL {
			t.Fatalf("unexpected url; got %q; want %q", str, expectedURL)
		}
		if str := s.String(); str != expectedSecret {
			t.Fatalf("unexpected value returned from URL.String; got %q; want %q", str, expectedSecret)
		}
	}

}
