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
	f("fs:///data1", "/data", false)
	f("fs:///data", "/data1", false)
	f("fs:///data", "/data/foo", false)
	f("fs:///data/foo", "/data", true)
	f("fs:///data/foo/", "/data/", true)
}
