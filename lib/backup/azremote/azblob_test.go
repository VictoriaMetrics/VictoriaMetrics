package azremote

import (
	"bytes"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func Test_cleanDirectory(t *testing.T) {
	cases := map[string]struct {
		Dir         string
		ExpectedDir string
	}{
		"dir / prefix is removed": {
			Dir:         "/foo/",
			ExpectedDir: "foo/",
		},
		"multiple dir prefix / is removed": {
			Dir:         "//foo/",
			ExpectedDir: "foo/",
		},
		"suffix is added": {
			Dir:         "foo",
			ExpectedDir: "foo/",
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			dir := cleanDirectory(test.Dir)

			if dir != test.ExpectedDir {
				t.Errorf("expected dir %q, got %q", test.ExpectedDir, dir)
			}
		})
	}
}

func Test_FSInit(t *testing.T) {
	cases := map[string]struct {
		IgnoreFakeEnv bool
		Env           testEnv
		ExpectedDir   string
		ExpectedErr   string
		ExpectedLogs  []string
	}{
		"connection string err bubbles": {
			Env: map[string]string{
				envStorageAccCs: "teapot",
			},
			ExpectedLogs: []string{`Creating AZBlob service client from connection string`},
			ExpectedErr:  `connection string is either blank or malformed`,
		},
		"connection string env var is used": {
			Env: map[string]string{
				envStorageAccCs: "BlobEndpoint=https://test.blob.core.windows.net/;SharedAccessSignature=",
			},
			ExpectedLogs: []string{`Creating AZBlob service client from connection string`},
		},
		"base envtemplate package is used and connection string err bubbles": {
			IgnoreFakeEnv: true,
			Env: map[string]string{
				envStorageAccCs: "BlobEndpoint=https://test.blob.core.windows.net/;SharedAccessSignature=",
			},
			ExpectedErr: "failed to detect any credentials",
		},
		"only storage account name is an err": {
			Env: map[string]string{
				envStorageAcctName: "test",
			},
			ExpectedErr: "failed to detect any credentials",
		},
		"shared key credential err bubbles": {
			Env: map[string]string{
				envStorageAcctName: "test",
				envStorageAccKey:   "teapot",
			},
			ExpectedLogs: []string{`Creating AZBlob service client from account name and key`},
			ExpectedErr:  `illegal base64 data at input`,
		},
		"uses shared key credential": {
			Env: map[string]string{
				envStorageAcctName: "test",
				envStorageAccKey:   "dGVhcG90Cg==",
			},
			ExpectedLogs: []string{`Creating AZBlob service client from account name and key`},
		},
		"allows overriding domain name with account name and key": {
			Env: map[string]string{
				envStorageAcctName: "test",
				envStorageAccKey:   "dGVhcG90Cg==",
				envStorageDomain:   "foo.bar",
			},
			ExpectedLogs: []string{
				`Creating AZBlob service client from account name and key`,
				`Overriding default Azure blob domain with "foo.bar"`,
			},
		},
		"can't specify both connection string and shared key": {
			Env: map[string]string{
				envStorageAccCs:    "teapot",
				envStorageAcctName: "test",
				envStorageAccKey:   "dGVhcG90Cg==",
			},
			ExpectedErr: `only one of connection string, account name and key, or default credential can be specified`,
		},
		"just use default is an err": {
			Env: map[string]string{
				envStorageDefault: "true",
			},
			ExpectedErr: "failed to detect any credentials",
		},
		"uses default credential": {
			Env: map[string]string{
				envStorageDefault:  "true",
				envStorageAcctName: "test",
			},
			ExpectedLogs: []string{`Creating AZBlob service client from default credential`},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			tlog := &testLogger{}

			logger.SetOutputForTests(tlog)
			t.Cleanup(logger.ResetOutputForTest)

			fs := &FS{Dir: "foo"}
			if test.Env != nil && !test.IgnoreFakeEnv {
				fs.env = test.Env.LookupEnv
			}

			err := fs.Init()
			checkErr(t, err, test.ExpectedErr)

			tlog.MustContain(t, test.ExpectedLogs...)
		})
	}
}

func checkErr(t *testing.T, err error, shouldContain string) {
	t.Helper()

	switch {
	case err == nil && shouldContain != "":
		t.Errorf("expected error %q, got nil", shouldContain)
	case err == nil && shouldContain == "":
		return
	case err != nil && shouldContain == "":
		t.Errorf("expected no error, got %q", err)
	case !strings.Contains(err.Error(), shouldContain):
		t.Errorf("expected error %q, got %q", shouldContain, err)
	}
}

type testLogger struct {
	buf *bytes.Buffer
}

func (l *testLogger) Write(p []byte) (n int, err error) {
	if l.buf == nil {
		l.buf = &bytes.Buffer{}
	}

	return l.buf.Write(p)
}

func (l *testLogger) MustContain(t *testing.T, vals ...string) {
	t.Helper()

	contents := l.buf.String()

	for _, val := range vals {
		if !strings.Contains(contents, val) {
			t.Errorf("expected log to contain %q, got %q", val, l.buf.String())
		}
	}
}

type testEnv map[string]string

func (e testEnv) LookupEnv(key string) (string, bool) {
	val, ok := e[key]
	return val, ok
}
