package workingsetcache

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestLoadFromFileOrNewError(t *testing.T) {
	defer fs.MustRemoveDir(t.Name())

	f := func(path string, expErr string) {
		logBuffer := &bytes.Buffer{}
		logger.SetOutputForTests(logBuffer)
		defer logger.ResetOutputForTest()

		cache := loadFromFileOrNew(path, 10000)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		testCacheEntriesEqual(t, cache, 0)
		if !strings.Contains(logBuffer.String(), expErr) {
			t.Fatalf("expected log message not found; got: %s", logBuffer.String())
		}
	}

	f("cacheDirNotExist", "missing files; init new cache")

	path := filepath.Join(t.Name(), "workingsetcache", "emptyDir")
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatalf("failed to create cache directory: %v", err)
	}
	f(path, "missing files; init new cache")

	path = initCacheForTest(t, `missingMetadata`, 10000)
	fs.MustRemovePath(filepath.Join(path, `metadata.bin`))
	f(path, "missing files; init new cache")

	path = initCacheForTest(t, `invalidMetadata`, 10000)
	fs.MustWriteSync(filepath.Join(path, `metadata.bin`), nil)
	f(path, "invalid: cannot read maxBucketChunks")

	path = initCacheForTest(t, `cacheMismatch`, 87654321)
	f(path, "contains maxBytes=10000; want 33554432; init new cache")
}

func TestLoadFromFileOrNewOK(t *testing.T) {
	defer fs.MustRemoveDir(t.Name())

	cachePath := initCacheForTest(t, `ok`, 10000)

	cache := loadFromFileOrNew(cachePath, 10000)
	if cache == nil {
		t.Fatal("expected a new cache instance, got nil")
	}

	testCacheEntriesEqual(t, cache, 200)

	actualVal := cache.Get(nil, []byte("foo1"))
	if !bytes.Equal(actualVal, []byte("fooVal")) {
		t.Fatalf("expected cached value 'fooVal', got %q", actualVal)
	}
}

func initCacheForTest(t *testing.T, testCase string, maxBytes int) string {
	c := New(maxBytes)
	defer c.Stop()

	for i := 0; i < 100; i++ {
		c.Set([]byte("foo"+strconv.Itoa(i)), []byte("fooVal"))
		c.Set([]byte("bar"+strconv.Itoa(i)), []byte("barVal"))
	}

	path := filepath.Join(t.Name(), "workingsetcache", testCase)
	if err := c.Save(path); err != nil {
		t.Fatalf("Save error: %s", err)
	}

	return path
}

func testCacheEntriesEqual(t *testing.T, c *fastcache.Cache, expEntries uint64) {
	var s fastcache.Stats
	c.UpdateStats(&s)

	if s.EntriesCount != expEntries {
		t.Fatalf("expected %d entries in cache, got %d", expEntries, s.EntriesCount)
	}
}

// assertMode checks that the cache mode matches the expected one.
func assertMode(t *testing.T, c *Cache, want uint32) {
	t.Helper()
	if got := c.mode.Load(); got != want {
		t.Fatalf("unexpected cache mode: got %d, want %d", got, want)
	}
}

// assertMode checks that the cache stats matches the expected one.
func assertStats(t *testing.T, c *Cache, want fastcache.Stats) {
	t.Helper()
	var got fastcache.Stats
	c.UpdateStats(&got)
	ignoreFields := cmpopts.IgnoreFields(fastcache.Stats{}, "BytesSize", "MaxBytesSize", "EvictedBytes")
	if diff := cmp.Diff(want, got, ignoreFields); diff != "" {
		t.Fatalf("unexpected stats (-want, +got):\n%s", diff)
	}
}

// removeAll removes the contents of t.Name() directory if the test succeeded.
// For this to work, a test is expected to store its data in t.Name() dir.
// In case of test failure the directory is not removed to allow for manual
// inspection of the directory.
func removeAll(t *testing.T) {
	if !t.Failed() {
		_ = os.RemoveAll(t.Name())
	}
}
