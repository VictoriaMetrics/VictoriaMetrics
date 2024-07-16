package azremote

import (
	"strings"
	"testing"
)

func TestCleanDirectory(t *testing.T) {
	f := func(dir, exp string) {
		t.Helper()

		got := cleanDirectory(dir)
		if got != exp {
			t.Fatalf("expected dir %q, got %q", exp, got)
		}
	}

	f("/foo/", "foo/")
	f("//foo/", "foo/")
	f("foo", "foo/")
}

func TestFSInit(t *testing.T) {
	f := func(expErr string, params ...string) {
		t.Helper()

		env := make(testEnv)
		for i := 0; i < len(params); i += 2 {
			env[params[i]] = params[i+1]
		}

		fs := &FS{Dir: "foo"}
		fs.env = env.LookupEnv
		err := fs.Init()
		if err != nil {
			if expErr == "" {
				t.Fatalf("unexpected error %v", err)
			}
			if !strings.Contains(err.Error(), expErr) {
				t.Fatalf("expected error: \n%q, \ngot: \n%v", expErr, err)
			}
			return
		}
		if expErr != "" {
			t.Fatalf("expected to have an error %q, instead got nil", expErr)
		}
	}

	f("", envStorageAccCs, "BlobEndpoint=https://test.blob.core.windows.net/;SharedAccessSignature=")
	f("", envStorageAcctName, "test", envStorageAccKey, "dGVhcG90Cg==")
	f("", envStorageDefault, "true", envStorageAcctName, "test")
	f("", envStorageAcctName, "test", envStorageAccKey, "dGVhcG90Cg==", envStorageDomain, "foo.bar")

	f("failed to detect credentials for AZBlob")
	f("failed to detect credentials for AZBlob", envStorageAcctName, "test")
	f("failed to create Shared Key", envStorageAcctName, "", envStorageAccKey, "!")
	f("connection string is either blank or malformed", envStorageAccCs, "")
	f("failed to process credentials: only one of", envStorageAccCs, "teapot", envStorageAcctName, "test", envStorageAccKey, "dGVhcG90Cg==")
}

type testEnv map[string]string

func (e testEnv) LookupEnv(key string) (string, bool) {
	val, ok := e[key]
	return val, ok
}
