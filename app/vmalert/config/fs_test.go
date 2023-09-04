package config

import (
	"strings"
	"testing"
)

func TestNewFS(t *testing.T) {
	f := func(path, expStr string) {
		t.Helper()
		fs, err := newFS(path)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		if fs.String() != expStr {
			t.Fatalf("expected FS %q; got %q", expStr, fs.String())
		}
	}

	f("/foo/bar", "Local FS{MatchPattern: \"/foo/bar\"}")
	f("fs:///foo/bar", "Local FS{MatchPattern: \"/foo/bar\"}")
}

func TestNewFSNegative(t *testing.T) {
	f := func(path, expErr string) {
		t.Helper()
		_, err := newFS(path)
		if err == nil {
			t.Fatalf("expected to have err: %s", expErr)
		}
		if !strings.Contains(err.Error(), expErr) {
			t.Fatalf("expected to have err %q; got %q instead", expErr, err)
		}
	}

	f("", "path cannot be empty")
	f("fs://", "path cannot be empty")
	f("foobar://baz", `unsupported scheme "foobar"`)
}
