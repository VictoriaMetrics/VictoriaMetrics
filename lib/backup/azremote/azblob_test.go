package azremote

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
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

		err := fs.Init(context.Background())
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

		err := fs.Init(context.Background())
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

// capturingCredential records the context it was called with instead of acquiring a real token.
type capturingCredential struct {
	ctx context.Context
}

func (c *capturingCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.ctx = ctx
	return azcore.AccessToken{}, nil
}

// Only checks that GetToken is given a deadline
func TestCredentialWithTimeout_GetToken(t *testing.T) {
	cred := &capturingCredential{}
	if _, err := (credentialWithTimeout{cred}).GetToken(context.Background(), policy.TokenRequestOptions{}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if deadline, ok := cred.ctx.Deadline(); !ok || time.Until(deadline) > credentialAcquisitionTimeout {
		t.Fatalf("expected a deadline at most %s from now, got %v (ok=%v)", credentialAcquisitionTimeout, deadline, ok)
	}
}

type testEnv map[string]string

func (e testEnv) LookupEnv(key string) (string, bool) {
	val, ok := e[key]
	return val, ok
}
