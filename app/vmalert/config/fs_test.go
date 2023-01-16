package config

import (
	"strings"
	"testing"
)

func TestNewFS(t *testing.T) {
	f := func(path, expStr string) {
		t.Helper()
		fs, err := newFS(path, false)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		if fs.String() != expStr {
			t.Fatalf("expected FS %q; got %q", expStr, fs.String())
		}
		fs.MustStop()
	}

	f("/foo/bar", "Local FS{MatchPattern: \"/foo/bar\"}")

	f("gs://foo", "GCS{bucket: \"foo\", Prefix: \"\"}")
	f("gcs://foo", "GCS{bucket: \"foo\", Prefix: \"\"}")
	f("gcs://foo/", "GCS{bucket: \"foo\", Prefix: \"\"}")
	f("gcs://foo/bar", "GCS{bucket: \"foo\", Prefix: \"bar\"}")
	f("gcs://foo/bar/", "GCS{bucket: \"foo\", Prefix: \"bar\"}")
	f("gcs://foo/////bar/", "GCS{bucket: \"foo\", Prefix: \"bar\"}")
	f("gcs://foo/////bar///", "GCS{bucket: \"foo\", Prefix: \"bar\"}")

	f("s3://foo", "S3{bucket: \"foo\", Prefix: \"\"}")
	f("s3://foo/", "S3{bucket: \"foo\", Prefix: \"\"}")
	f("s3://foo/bar", "S3{bucket: \"foo\", Prefix: \"bar\"}")
	f("s3://foo/bar/", "S3{bucket: \"foo\", Prefix: \"bar\"}")
	f("s3://foo/////bar/", "S3{bucket: \"foo\", Prefix: \"bar\"}")
	f("s3://foo/////bar///", "S3{bucket: \"foo\", Prefix: \"bar\"}")
}

func TestNewFSNegative(t *testing.T) {
	f := func(path, expErr string) {
		t.Helper()
		_, err := newFS(path, false)
		if err == nil {
			t.Fatalf("expected to have err: %s", expErr)
		}
		if !strings.Contains(err.Error(), expErr) {
			t.Fatalf("expected to have err %q; got %q instead", expErr, err)
		}
	}

	f("", "path cannot be empty")
	f("gcs://", "can't parse bucket name for gcs")
	f("s3://", "can't parse bucket name for s3")
	f("foo://bar", "unsupported scheme")

}
