package workingsetcache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileOrNew(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "test.cache")
	maxBytes := 1024 * 1024 // 1MB

	t.Run("file does not exist", func(t *testing.T) {
		// No file created, should fallback silently
		cache := loadFromFileOrNew(cacheFile, maxBytes)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		// Create empty file and remove read permission
		if err := os.WriteFile(cacheFile, []byte("data"), 0000); err != nil {
			t.Fatalf("failed to write test cache file: %v", err)
		}
		defer os.Chmod(cacheFile, 0644) // restore permission after test

		cache := loadFromFileOrNew(cacheFile, maxBytes)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}
	})

	t.Run("memory mismatch (corrupt cache file)", func(t *testing.T) {
		// Write invalid content that simulates mismatch
		if err := os.WriteFile(cacheFile, []byte("not a real cache file"), 0644); err != nil {
			t.Fatalf("failed to write corrupt cache file: %v", err)
		}

		cache := loadFromFileOrNew(cacheFile, maxBytes)
		if cache == nil {
			t.Fatal("expected a new cache instance, got nil")
		}
	})
}