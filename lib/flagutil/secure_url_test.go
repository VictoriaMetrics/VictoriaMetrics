package flagutil

import (
	"path/filepath"
	"testing"
)

func TestSecureUrl(t *testing.T) {
	s := SecureUrl{
		flagname: "url-foo",
	}

	// Verify that String returns "secret"
	expectedSecret := "secret"
	if str := s.String(); str != expectedSecret {
		t.Fatalf("unexpected value returned from SecureUrl.String; got %q; want %q", str, expectedSecret)
	}

	// set regular url
	expectedUrl := "http://usr:pass@localhost:8428/snapshot/create"
	if err := s.Set(expectedUrl); err != nil {
		t.Fatalf("cannot set url: %s", err)
	}
	for i := 0; i < 5; i++ {
		if str := s.Get(); str != expectedUrl {
			t.Fatalf("unexpected url; got %q; want %q", str, expectedUrl)
		}
		if str := s.String(); str != expectedSecret {
			t.Fatalf("unexpected value returned from SecureUrl.String; got %q; want %q", str, expectedSecret)
		}
	}

	// read the url from file by relative path
	localUrlFile := "testdata/url.txt"
	expectedUrl = "http://usr:pass@localhost:8428/snapshot/create"
	path := "file://" + localUrlFile
	if err := s.Set(path); err != nil {
		t.Fatalf("cannot set url to file: %s", err)
	}
	for i := 0; i < 5; i++ {
		if str := s.Get(); str != expectedUrl {
			t.Fatalf("unexpected url; got %q; want %q", str, expectedUrl)
		}
		if str := s.String(); str != expectedSecret {
			t.Fatalf("unexpected value returned from SecureUrl.String; got %q; want %q", str, expectedSecret)
		}
	}

	// read the url from file by absolute path
	var err error
	localUrlFile, err = filepath.Abs("testdata/url.txt")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedUrl = "http://usr:pass@localhost:8428/snapshot/create"
	path = "file://" + localUrlFile
	if err := s.Set(path); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	for i := 0; i < 5; i++ {
		if str := s.Get(); str != expectedUrl {
			t.Fatalf("unexpected url; got %q; want %q", str, expectedUrl)
		}
		if str := s.String(); str != expectedSecret {
			t.Fatalf("unexpected value returned from SecureUrl.String; got %q; want %q", str, expectedSecret)
		}
	}

}
