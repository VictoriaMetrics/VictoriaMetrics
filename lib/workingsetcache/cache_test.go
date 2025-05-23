package workingsetcache

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestLoadFromFileOrNew(t *testing.T) {
	newCache := func(t *testing.T, maxBytes int) string {
		tmpDir, err := os.MkdirTemp("", "test")
		if err != nil {
			t.Fatal(err)
		}
		filePath := filepath.Join(tmpDir, "TestLoadFromFileOrNew.workingsetcache")

		c := New(maxBytes)

		c.Set([]byte("foo"), []byte("fooVal"))
		c.Set([]byte("bar"), []byte("barVal"))
		if err := c.Save(filePath); err != nil {
			t.Fatalf("Save error: %s", err)
		}

		t.Cleanup(func() {
			c.Reset()
			os.RemoveAll(filePath)
		})

		return filePath
	}

	t.Run("CacheDirNotExist", func(t *testing.T) {
		logBuffer := &bytes.Buffer{}
		logger.SetOutputForTests(logBuffer)
		defer logger.ResetOutputForTest()

		// No file created, should fallback silently
		cache := loadFromFileOrNew(`cacheDir/Not/Exist`, 10000)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		if len(cache.Get(nil, []byte("foo"))) != 0 {
			t.Fatalf("expected empty cache, got non-empty")
		}
		if !strings.Contains(logBuffer.String(), "not found; init new cache") {
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

		if len(cache.Get(nil, []byte("foo"))) != 0 {
			t.Fatalf("expected empty cache, got non-empty")
		}
		if !strings.Contains(logBuffer.String(), "not found; init new cache") {
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

		if len(cache.Get(nil, []byte("foo"))) != 0 {
			t.Fatalf("expected empty cache, got non-empty")
		}
		if !strings.Contains(logBuffer.String(), "invalid: cannot read maxBucketChunks") {
			t.Fatalf("expected log message not found; got: %s", logBuffer.String())
		}
	})

	t.Run("CacheSizeMismatch", func(t *testing.T) {
		cachePath := newCache(t, 987654321)

		logBuffer := &bytes.Buffer{}
		logger.SetOutputForTests(logBuffer)
		defer logger.ResetOutputForTest()

		cache := loadFromFileOrNew(cachePath, 123456789)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		if len(cache.Get(nil, []byte("foo"))) != 0 {
			t.Fatalf("expected empty cache, got non-empty")
		}
		if !strings.Contains(logBuffer.String(), "contains maxBytes=123456789; want 134217728; init new cache") {
			t.Fatalf("expected log message not found; got: %s", logBuffer.String())
		}
	})

	t.Run("LoadedOK", func(t *testing.T) {
		cachePath := newCache(t, 10000)

		cache := loadFromFileOrNew(cachePath, 10000)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}

		actualVal := cache.Get(nil, []byte("foo"))
		if !bytes.Equal(actualVal, []byte("fooVal")) {
			t.Fatalf("expected cached value 'fooVal', got %q", actualVal)
		}
	})
}
