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

func TestLoadFromFileOrNew(t *testing.T) {
	defer os.RemoveAll(t.Name())

	newCache := func(t *testing.T, maxBytes int) string {
		c := New(maxBytes)
		defer c.Stop()

		for i := 0; i < 100; i++ {
			c.Set([]byte("foo"+strconv.Itoa(i)), []byte("fooVal"))
			c.Set([]byte("bar"+strconv.Itoa(i)), []byte("barVal"))
		}

		path := filepath.Join(t.Name(), "workingsetcache")
		if err := c.Save(path); err != nil {
			t.Fatalf("Save error: %s", err)
		}

		return path
	}

	t.Run("CacheDirNotExist", func(t *testing.T) {
		logBuffer := &bytes.Buffer{}
		logger.SetOutputForTests(logBuffer)
		defer logger.ResetOutputForTest()

		path := filepath.Join(t.Name(), "workingsetcache")

		// No file created, should fallback silently
		cache := loadFromFileOrNew(path, 10000)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		testCacheEntriesEqual(t, cache, 0)
		if !strings.Contains(logBuffer.String(), "missing files; init new cache") {
			t.Fatalf("expected log message not found; got: %s", logBuffer.String())
		}
	})

	t.Run("CacheDirEmpty", func(t *testing.T) {
		logBuffer := &bytes.Buffer{}
		logger.SetOutputForTests(logBuffer)
		defer logger.ResetOutputForTest()

		path := filepath.Join(t.Name(), "workingsetcache")
		if err := os.MkdirAll(path, 0777); err != nil {
			t.Fatalf("failed to create cache directory: %v", err)
		}

		cache := loadFromFileOrNew(path, 10000)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}
		testCacheEntriesEqual(t, cache, 0)
		if !strings.Contains(logBuffer.String(), "missing files; init new cache") {
			t.Fatalf("expected log message not found; got: %s", logBuffer.String())
		}
	})

	t.Run("MetadataFileNotExist", func(t *testing.T) {
		cachePath := newCache(t, 10000)

		if err := os.Remove(filepath.Join(cachePath, `metadata.bin`)); err != nil {
			t.Fatalf("failed to remove metadata.bin file: %v", err)
		}

		logBuffer := &bytes.Buffer{}
		logger.SetOutputForTests(logBuffer)
		defer logger.ResetOutputForTest()

		cache := loadFromFileOrNew(cachePath, 10000)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		testCacheEntriesEqual(t, cache, 0)
		if !strings.Contains(logBuffer.String(), "missing files; init new cache") {
			t.Fatalf("expected log message not found; got: %s", logBuffer.String())
		}
	})

	t.Run("MetadataFileInvalid", func(t *testing.T) {
		cachePath := newCache(t, 10000)

		if err := os.WriteFile(filepath.Join(cachePath, `metadata.bin`), []byte(""), 0644); err != nil {
			t.Fatalf("failed to write test metadata file: %v", err)
		}

		logBuffer := &bytes.Buffer{}
		logger.SetOutputForTests(logBuffer)
		defer logger.ResetOutputForTest()

		cache := loadFromFileOrNew(cachePath, 10000)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		testCacheEntriesEqual(t, cache, 0)
		if !strings.Contains(logBuffer.String(), "unusable: cannot read maxBucketChunks") {
			t.Fatalf("expected log message not found; got: %s", logBuffer.String())
		}
	})

	t.Run("CacheSizeMismatch", func(t *testing.T) {
		cachePath := newCache(t, 87654321)

		logBuffer := &bytes.Buffer{}
		logger.SetOutputForTests(logBuffer)
		defer logger.ResetOutputForTest()

		cache := loadFromFileOrNew(cachePath, 1234567)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		testCacheEntriesEqual(t, cache, 0)
		if !strings.Contains(logBuffer.String(), "contains maxBytes=1234567; want 33554432; init new cache") {
			t.Fatalf("expected log message not found; got: %s", logBuffer.String())
		}
	})

	t.Run("LoadedOK", func(t *testing.T) {
		cachePath := newCache(t, 10000)

		cache := loadFromFileOrNew(cachePath, 10000)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		testCacheEntriesEqual(t, cache, 200)

		actualVal := cache.Get(nil, []byte("foo1"))
		if !bytes.Equal(actualVal, []byte("fooVal")) {
			t.Fatalf("expected cached value 'fooVal', got %q", actualVal)
		}
	})
}

func testCacheEntriesEqual(t *testing.T, c *fastcache.Cache, expEntries uint64) {
	var s fastcache.Stats
	c.UpdateStats(&s)

	if s.EntriesCount != expEntries {
		t.Fatalf("expected %d entries in cache, got %d", expEntries, s.EntriesCount)
	}
}
