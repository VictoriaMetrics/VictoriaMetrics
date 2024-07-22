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

func TestFSInit_Failure(t *testing.T) {
	f := func(envArgs map[string]string, errStrExpected string) {
		t.Helper()

		fs := &FS{
			Dir: "foo",
		}
		env := testEnv(envArgs)
		fs.envLookupFunc = env.LookupEnv

		err := fs.Init()
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		errStr := err.Error()
		if !strings.Contains(errStr, errStrExpected) {
			t.Fatalf("expecting %q in the error %q", errStrExpected, errStr)
		}
	}

	var envArgs map[string]string

	f(envArgs, "failed to detect credentials for AZBlob")

	envArgs = map[string]string{
		"AZURE_STORAGE_ACCOUNT_NAME": "test",
	}
	f(envArgs, "failed to detect credentials for AZBlob")

	envArgs = map[string]string{
		"AZURE_STORAGE_ACCOUNT_NAME": "",
		"AZURE_STORAGE_ACCOUNT_KEY":  "!",
	}
	f(envArgs, "missing AZURE_STORAGE_ACCOUNT_NAME")

	envArgs = map[string]string{
		"AZURE_STORAGE_ACCOUNT_NAME": "foo",
		"AZURE_STORAGE_ACCOUNT_KEY":  "!",
	}
	f(envArgs, "failed to create Shared Key credentials")

	envArgs = map[string]string{
		"AZURE_STORAGE_ACCOUNT_CONNECTION_STRING": "foobar",
	}
	f(envArgs, "connection string is either blank or malformed")

	envArgs = map[string]string{
		"AZURE_STORAGE_ACCOUNT_CONNECTION_STRING": "teapot",
		"AZURE_STORAGE_ACCOUNT_NAME":              "test",
		"AZURE_STORAGE_ACCOUNT_KEY":               "dGVhcG90Cg==",
	}
	f(envArgs, "connection string is either blank or malformed")

	envArgs = map[string]string{
		"AZURE_USE_DEFAULT_CREDENTIAL": "true",
	}
	f(envArgs, "missing AZURE_STORAGE_ACCOUNT_NAME")
}

func TestFSInit_Success(t *testing.T) {
	f := func(envArgs map[string]string) {
		t.Helper()

		fs := &FS{
			Dir: "foo",
		}
		env := testEnv(envArgs)
		fs.envLookupFunc = env.LookupEnv

		err := fs.Init()
		if err != nil {
			t.Fatalf("unexpected error at fs.Init(): %s", err)
		}
	}

	envArgs := map[string]string{
		"AZURE_STORAGE_ACCOUNT_CONNECTION_STRING": "BlobEndpoint=https://test.blob.core.windows.net/;SharedAccessSignature=",
	}
	f(envArgs)

	envArgs = map[string]string{
		"AZURE_STORAGE_ACCOUNT_NAME": "test",
		"AZURE_STORAGE_ACCOUNT_KEY":  "dGVhcG90Cg==",
	}
	f(envArgs)

	envArgs = map[string]string{
		"AZURE_USE_DEFAULT_CREDENTIAL": "true",
		"AZURE_STORAGE_ACCOUNT_NAME":   "test",
	}
	f(envArgs)

	envArgs = map[string]string{
		"AZURE_STORAGE_ACCOUNT_NAME": "test",
		"AZURE_STORAGE_ACCOUNT_KEY":  "dGVhcG90Cg==",
		"AZURE_STORAGE_DOMAIN":       "foo.bar",
	}
	f(envArgs)
}

type testEnv map[string]string

func (e testEnv) LookupEnv(key string) (string, bool) {
	val, ok := e[key]
	return val, ok
}
