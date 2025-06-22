package workingsetcache

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/fastcache"
)

func TestLoadFromFileOrNewError(t *testing.T) {
	defer os.RemoveAll(t.Name())

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
	if err := os.Remove(filepath.Join(path, `metadata.bin`)); err != nil {
		t.Fatalf("failed to remove metadata.bin file: %v", err)
	}
	f(path, "missing files; init new cache")

	path = initCacheForTest(t, `invalidMetadata`, 10000)
	if err := os.WriteFile(filepath.Join(path, `metadata.bin`), []byte(""), 0644); err != nil {
		t.Fatalf("failed to write test metadata file: %v", err)
	}
	f(path, "invalid: cannot read maxBucketChunks")

	path = initCacheForTest(t, `cacheMismatch`, 87654321)
	f(path, "contains maxBytes=10000; want 33554432; init new cache")
}

func TestLoadFromFileOrNewOK(t *testing.T) {
	defer os.RemoveAll(t.Name())

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
