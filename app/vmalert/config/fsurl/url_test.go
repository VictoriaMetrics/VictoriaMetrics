package fsurl

import "testing"

func TestFSInitRequiresHost(t *testing.T) {
	fs := &FS{Path: "http:///rules.yml"}
	if err := fs.Init(); err == nil {
		t.Fatal("expected a url with an empty host to be rejected")
	}

	fs = &FS{Path: "https://example.com/rules.yml"}
	if err := fs.Init(); err != nil {
		t.Fatal(err)
	}
}
