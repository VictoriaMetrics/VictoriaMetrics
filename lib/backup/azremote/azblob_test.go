package azremote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

func TestDownloadPartRange(t *testing.T) {
	testCases := []struct {
		name                 string
		offset, size         uint64
		empty, ignore, short bool
		wantErr              bool
	}{
		{"whole", 0, 16, false, false, false, false},
		{"middle", 4, 5, false, false, false, false},
		{"tail", 15, 1, false, false, false, false},
		{"empty", 0, 0, true, false, false, false},
		{"ignored-range", 4, 5, false, true, false, true},
		{"short-response", 4, 5, false, false, true, true},
		{"outside-object", 15, 2, false, false, false, true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := "0123456789abcdef"
			if tc.empty {
				data = ""
			}
			p := common.Part{Path: "data/file", FileSize: 1024, Offset: 128, Size: uint64(len(data))}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/container/"+p.RemotePath("backup") {
					t.Errorf("request changed the original object key: %s", r.URL.Path)
				}
				rangeHeader := r.Header.Get("x-ms-range")
				if rangeHeader == "" {
					rangeHeader = r.Header.Get("Range")
				}
				wantRange := ""
				if tc.offset != 0 || tc.size != p.Size {
					wantRange = fmt.Sprintf("bytes=%d-%d", tc.offset, tc.offset+tc.size-1)
				}
				if rangeHeader != wantRange {
					t.Errorf("unexpected range: %q; want %q", rangeHeader, wantRange)
				}
				r.Header.Set("Range", rangeHeader)
				if tc.ignore {
					_, _ = io.WriteString(w, data)
				} else if tc.short {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", tc.offset, tc.offset+tc.size-1, p.Size))
					w.WriteHeader(http.StatusPartialContent)
					_, _ = io.WriteString(w, data[tc.offset:tc.offset+tc.size-1])
				} else {
					http.ServeContent(w, r, "object", time.Time{}, bytes.NewReader([]byte(data)))
				}
			}))
			defer server.Close()
			client, err := container.NewClientWithNoCredential(server.URL+"/container", nil)
			if err != nil {
				t.Fatal(err)
			}
			fs := &FS{Container: "container", Dir: "backup/", client: client, ctx: context.Background()}
			var dst bytes.Buffer
			err = fs.DownloadPartRange(p, tc.offset, tc.size, &dst)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected download error: %v", err)
			}
			if !tc.wantErr && dst.String() != data[tc.offset:tc.offset+tc.size] {
				t.Fatalf("unexpected downloaded bytes: %q", dst.String())
			}
			if tc.name == "outside-object" && calls.Load() != 0 {
				t.Fatal("invalid range issued an HTTP request")
			}
		})
	}
}

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

type testEnv map[string]string

func (e testEnv) LookupEnv(key string) (string, bool) {
	val, ok := e[key]
	return val, ok
}
