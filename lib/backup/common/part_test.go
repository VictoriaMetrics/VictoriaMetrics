package common

import (
	"testing"
)

func TestIsLocalPathInsideDir(t *testing.T) {
	f := func(dir, path string, expected bool) {
		t.Helper()
		p := Part{Path: path}
		if got := p.IsLocalPathInsideDir(dir); got != expected {
			t.Fatalf("IsLocalPathInsideDir(%q, %q): got %v, want %v", dir, path, got, expected)
		}
	}

	// normal path inside dir
	f("/data/storage", "parts/segment1/data.bin", true)

	// dir with trailing slash is normalized
	f("/data/storage/", "parts/segment1/data.bin", true)

	// deeply nested path
	f("/data/storage", "a/b/c/d/e/file.dat", true)

	// traversal that stays inside dir
	f("/data/storage", "foo/../bar/file.dat", true)

	// root dir allows any path
	f("/", "any/path/here", true)

	// root dir allows traversal attempts since nothing is outside /
	f("/", "../outside/marker.txt", true)

	// path with leading slash is treated as relative by filepath.Join and stays inside dir
	f("/data/storage", "/outside/marker.txt", true)

	// dir with .. components is normalized; path inside resolved dir
	f("/data/storage/../foo", "parts/file.dat", true)

	// dir with .. components is normalized; traversal outside resolved dir
	f("/data/storage/../foo", "../storage/evil.txt", false)

	// simple traversal
	f("/data/storage", "../outside/marker.txt", false)

	// traversal with trailing slash in dir
	f("/data/storage/", "../outside/marker.txt", false)

	// deep traversal
	f("/data/storage", "a/../../outside/marker.txt", false)

	// sibling directory whose name shares a prefix with dir
	f("/data/storage", "../storagefoo/evil.txt", false)
}
