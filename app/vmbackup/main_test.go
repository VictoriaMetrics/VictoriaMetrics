package main

import (
	"path/filepath"
	"testing"
)

func TestHasFilepathPrefix(t *testing.T) {
	f := func(dst, storageDataPath string, resultExpected bool) {
		t.Helper()
		result := hasFilepathPrefix(dst, storageDataPath)
		if result != resultExpected {
			t.Errorf("unexpected hasFilepathPrefix(%q, %q); got: %v; want: %v", dst, storageDataPath, result, resultExpected)
		}
	}
	pwd, err := filepath.Abs("")
	if err != nil {
		t.Fatalf("cannot determine working directory: %s", err)
	}
	f("s3://foo/bar", "foo", false)
	f("fs://"+pwd+"/foo", "foo", true)
	f("fs://"+pwd+"/foo", "foo/bar", false)
	f("fs://"+pwd+"/foo/bar", "foo", true)
	f("fs://"+pwd+"/foo", "bar", false)
	f("fs://"+pwd+"/foo", pwd+"/foo", true)
	f("fs://"+pwd+"/foo", pwd+"/foo/bar", false)
	f("fs://"+pwd+"/foo/bar", pwd+"/foo", true)
	f("fs://"+pwd+"/foo", pwd+"/bar", false)
}
