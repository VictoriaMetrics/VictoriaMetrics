package flagutil

import (
	"path/filepath"
	"testing"
)

func TestPassword(t *testing.T) {
	p := Password{
		flagname: "foo",
	}

	// Verify that String returns "secret"
	expectedSecret := "secret"
	if s := p.String(); s != expectedSecret {
		t.Fatalf("unexpected value returned from Password.String; got %q; want %q", s, expectedSecret)
	}

	// set regular password
	expectedPassword := "top-secret-password"
	if err := p.Set(expectedPassword); err != nil {
		t.Fatalf("cannot set password: %s", err)
	}
	for i := 0; i < 5; i++ {
		if s := p.Get(); s != expectedPassword {
			t.Fatalf("unexpected password; got %q; want %q", s, expectedPassword)
		}
		if s := p.String(); s != expectedSecret {
			t.Fatalf("unexpected value returned from Password.String; got %q; want %q", s, expectedSecret)
		}
	}

	// read the password from file by relative path
	localPassFile := "testdata/password.txt"
	expectedPassword = "foo-bar-baz"
	path := "file://" + localPassFile
	if err := p.Set(path); err != nil {
		t.Fatalf("cannot set password to file: %s", err)
	}
	for i := 0; i < 5; i++ {
		if s := p.Get(); s != expectedPassword {
			t.Fatalf("unexpected password; got %q; want %q", s, expectedPassword)
		}
		if s := p.String(); s != expectedSecret {
			t.Fatalf("unexpected value returned from Password.String; got %q; want %q", s, expectedSecret)
		}
	}

	// read the password from file by absolute path
	var err error
	localPassFile, err = filepath.Abs("testdata/password.txt")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedPassword = "foo-bar-baz"
	path = "file://" + localPassFile
	if err := p.Set(path); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	for i := 0; i < 5; i++ {
		if s := p.Get(); s != expectedPassword {
			t.Fatalf("unexpected password; got %q; want %q", s, expectedPassword)
		}
		if s := p.String(); s != expectedSecret {
			t.Fatalf("unexpected value returned from Password.String; got %q; want %q", s, expectedSecret)
		}
	}

	// try reading the password from non-existing url
	if err := p.Set("http://127.0.0.1:56283/aaa/bb?cc=dd"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	for i := 0; i < 5; i++ {
		if s := p.Get(); len(s) != 64 {
			t.Fatalf("unexpected password obtained: %q; must be random 64-byte password", s)
		}
		if s := p.String(); s != expectedSecret {
			t.Fatalf("unexpected value returned from Password.String; got %q; want %q", s, expectedSecret)
		}
	}
}
