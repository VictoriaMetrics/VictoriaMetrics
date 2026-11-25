package fscommon

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestAppendFilesSymlinks(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "snapshot")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside, filepath.Join(root, "nested")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{filepath.Join(root, "inside.txt"), filepath.Join(root, "nested", "nested.txt"), filepath.Join(outside, "secret.txt")} {
		if err := os.WriteFile(name, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{
		"inside-link":    "inside.txt",
		"absolute-link":  filepath.Join(root, "inside.txt"),
		"outside-link":   filepath.Join(outside, "secret.txt"),
		"outside-dir":    outside,
		"directory-link": "nested",
		"cycle":          ".",
		"broken":         "missing.txt",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	rootLink := filepath.Join(base, "snapshot-link")
	if err := os.Symlink(root, rootLink); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativeRoot, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, dir string }{
		{"absolute", root}, {"relative", relativeRoot}, {"symlink-root", rootLink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := []string{"existing"}
			files, err := AppendFiles(prefix, tc.dir)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"existing"}
			for _, name := range []string{"inside.txt", "inside-link", "absolute-link", filepath.Join("nested", "nested.txt")} {
				want = append(want, filepath.Join(tc.dir, name))
			}
			sort.Strings(files)
			sort.Strings(want)
			if !reflect.DeepEqual(files, want) {
				t.Fatalf("got %q; want %q", files, want)
			}
		})
	}
}

func TestPathInside(t *testing.T) {
	root := t.TempDir()
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	for _, tc := range []struct {
		root, target string
		want         bool
	}{
		{root, root, true},
		{root, filepath.Join(root, "child"), true},
		{root, filepath.Dir(root), false},
		{root, root + "-sibling", false},
		{volumeRoot, root, true},
		{volumeRoot, volumeRoot, true},
	} {
		if got := pathInside(tc.root, tc.target); got != tc.want {
			t.Errorf("pathInside(%q, %q) = %t; want %t", tc.root, tc.target, got, tc.want)
		}
	}
}
