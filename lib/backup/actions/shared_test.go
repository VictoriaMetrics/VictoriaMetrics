package actions

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsremote"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot/snapshotutil"
)

var (
	sharedTestSnapshot1 = map[string]string{
		"data/small/2025_01/0000000000000001/values.bin": "aaa",
		"data/small/2025_01/0000000000000002/values.bin": "bbb",
		"data/small/2025_01/parts.json":                  `["0000000000000001","0000000000000002"]`,
		"metadata/minTimestampForCompositeIndex":         "123",
	}

	// sharedTestSnapshot2 keeps part 1 and replaces part 2 with part 3.
	sharedTestSnapshot2 = map[string]string{
		"data/small/2025_01/0000000000000001/values.bin": "aaa",
		"data/small/2025_01/0000000000000003/values.bin": "bbbccc",
		"data/small/2025_01/parts.json":                  `["0000000000000001","0000000000000003"]`,
		"metadata/minTimestampForCompositeIndex":         "123",
	}
)

func TestSharedBackupRestore(t *testing.T) {
	dir := t.TempDir()
	repo := newTestSharedRepo(dir)

	mustSharedBackup(t, dir, repo, "b1", sharedTestSnapshot1, 0)
	mustSharedBackup(t, dir, repo, "b2", sharedTestSnapshot2, 0)

	storeParts := mustListParts(t, repo.store)
	wantStorePaths := []string{
		"data/small/2025_01/0000000000000001/values.bin",
		"data/small/2025_01/0000000000000002/values.bin",
		"data/small/2025_01/0000000000000003/values.bin",
	}
	if !reflect.DeepEqual(storeParts, wantStorePaths) {
		t.Fatalf("unexpected store parts\ngot\n%q\nwant\n%q", storeParts, wantStorePaths)
	}

	// Restore the newest backup from the repo root on top of b1
	// in order to verify that the restore removes the files missing in b2.
	dstDir := filepath.Join(dir, "restore")
	mustRestore(t, filepath.Join(dir, "backups", "b1"), dstDir)
	if got := readTestDir(t, dstDir); !reflect.DeepEqual(got, sharedTestSnapshot1) {
		t.Fatalf("unexpected restored files for b1\ngot\n%v\nwant\n%v", got, sharedTestSnapshot1)
	}
	mustRestore(t, dir, dstDir)
	if got := readTestDir(t, dstDir); !reflect.DeepEqual(got, sharedTestSnapshot2) {
		t.Fatalf("unexpected restored files for b2\ngot\n%v\nwant\n%v", got, sharedTestSnapshot2)
	}
}

func TestSharedBackupRestoreUncommitted(t *testing.T) {
	dir := t.TempDir()
	repo := newTestSharedRepo(dir)

	mustSharedBackup(t, dir, repo, "b1", sharedTestSnapshot1, 0)
	mustSharedBackup(t, dir, repo, "b2", sharedTestSnapshot2, 0)
	// Simulate a backup, which failed before the index update.
	if err := repo.writeIndex(&sharedIndex{Backups: []sharedIndexEntry{{Name: "b1"}}}); err != nil {
		t.Fatalf("cannot write index: %s", err)
	}

	if _, err := NewRestoreSrcFS(context.Background(), "fs://"+filepath.Join(dir, "backups", "b2")); err == nil {
		t.Fatalf("expecting error when restoring uncommitted backup")
	}
}

func TestClassicBackupToSharedRepo(t *testing.T) {
	dir := t.TempDir()
	repo := newTestSharedRepo(dir)
	mustSharedBackup(t, dir, repo, "b1", sharedTestSnapshot1, 0)

	b := &Backup{
		Concurrency: 2,
		Src:         newTestSnapshot(t, dir, sharedTestSnapshot2),
		Dst:         &fsremote.FS{Dir: dir},
	}
	if err := b.Run(); err == nil {
		t.Fatalf("expecting error for classic backup to shared repo")
	}
	if n := len(mustListParts(t, repo.store)); n != 2 {
		t.Fatalf("unexpected number of store parts; got %d; want 2", n)
	}
}

func TestSplitSharedBackupPath(t *testing.T) {
	f := func(path, repoExpected, nameExpected string, okExpected bool) {
		t.Helper()
		repo, name, ok := splitSharedBackupPath(path)
		if repo != repoExpected || name != nameExpected || ok != okExpected {
			t.Fatalf("unexpected splitSharedBackupPath(%q); got (%q, %q, %v); want (%q, %q, %v)",
				path, repo, name, ok, repoExpected, nameExpected, okExpected)
		}
	}

	f("s3://bucket/repo/backups/b1", "s3://bucket/repo", "b1", true)
	f("s3://bucket/repo/backups/b1/", "s3://bucket/repo", "b1", true)
	f("fs:///data/backups/b1", "fs:///data", "b1", true)

	f("s3://bucket/repo/b1", "", "", false)
	f("s3://bucket/repo/mybackups/b1", "", "", false)
	f("b1", "", "", false)
}

func TestSharedBackupKeepLastBackups(t *testing.T) {
	dir := t.TempDir()
	repo := newTestSharedRepo(dir)

	mustSharedBackup(t, dir, repo, "b1", sharedTestSnapshot1, 1)
	mustSharedBackup(t, dir, repo, "b2", sharedTestSnapshot2, 1)

	if _, err := repo.openBackup("b1"); err == nil {
		t.Fatalf("expecting error when opening deleted backup b1")
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "b1")); !os.IsNotExist(err) {
		t.Fatalf("expecting removed directory for backup b1; got stat error: %v", err)
	}
	storeParts := mustListParts(t, repo.store)
	wantStorePaths := []string{
		"data/small/2025_01/0000000000000001/values.bin",
		"data/small/2025_01/0000000000000003/values.bin",
	}
	if !reflect.DeepEqual(storeParts, wantStorePaths) {
		t.Fatalf("unexpected store parts\ngot\n%q\nwant\n%q", storeParts, wantStorePaths)
	}

	dstDir := filepath.Join(dir, "restore")
	mustRestore(t, dir, dstDir)
	if got := readTestDir(t, dstDir); !reflect.DeepEqual(got, sharedTestSnapshot2) {
		t.Fatalf("unexpected restored files for b2\ngot\n%v\nwant\n%v", got, sharedTestSnapshot2)
	}
}

func TestSharedBackupGCDryRun(t *testing.T) {
	dir := t.TempDir()
	repo := newTestSharedRepo(dir)

	mustSharedBackup(t, dir, repo, "b1", sharedTestSnapshot1, 0)
	b := &SharedBackup{
		Concurrency:     2,
		Src:             newTestSnapshot(t, dir, sharedTestSnapshot2),
		Repo:            repo,
		Name:            "b2",
		KeepLastBackups: 1,
		GCDryRun:        true,
	}
	if err := b.Run(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if _, err := repo.openBackup("b1"); err != nil {
		t.Fatalf("backup b1 must be kept in dry run mode: %s", err)
	}
	if n := len(mustListParts(t, repo.store)); n != 3 {
		t.Fatalf("unexpected number of store parts; got %d; want 3", n)
	}
}

func TestSharedBackupGCMissingManifest(t *testing.T) {
	dir := t.TempDir()
	repo := newTestSharedRepo(dir)

	mustSharedBackup(t, dir, repo, "b1", sharedTestSnapshot1, 0)
	if err := os.Remove(filepath.Join(dir, "backups", "b1", backupnames.BackupManifestFilename)); err != nil {
		t.Fatalf("cannot remove manifest: %s", err)
	}

	b := &SharedBackup{
		Concurrency: 2,
		Src:         newTestSnapshot(t, dir, sharedTestSnapshot2),
		Repo:        repo,
		Name:        "b2",
	}
	if err := b.Run(); err == nil {
		t.Fatalf("expecting GC error for backup with missing manifest")
	}
	// Part 2 is referenced only by b1, so GC must not delete it.
	if n := len(mustListParts(t, repo.store)); n != 3 {
		t.Fatalf("unexpected number of store parts; got %d; want 3", n)
	}
}

func TestSharedBackupInvalidRepo(t *testing.T) {
	f := func(prepare func(dir string), name string) {
		t.Helper()
		dir := t.TempDir()
		repo := newTestSharedRepo(dir)
		mustSharedBackup(t, dir, repo, "b1", sharedTestSnapshot1, 0)
		prepare(dir)

		b := &SharedBackup{
			Concurrency: 2,
			Src:         newTestSnapshot(t, dir, sharedTestSnapshot2),
			Repo:        repo,
			Name:        name,
		}
		if err := b.Run(); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// missing index for non-empty store
	f(func(dir string) {
		mustRemoveFile(t, filepath.Join(dir, backupnames.SharedIndexFilename))
	}, "b2")

	// classic backup at the root
	f(func(dir string) {
		mustWriteFile(t, filepath.Join(dir, backupnames.BackupCompleteFilename), "")
	}, "b2")

	// duplicate backup name
	f(func(_ string) {}, "b1")

	// invalid backup name
	f(func(_ string) {}, "..")
	f(func(_ string) {}, "foo/bar")
}

func TestIsStorePath(t *testing.T) {
	f := func(path string, resultExpected bool) {
		t.Helper()
		if result := isStorePath(path); result != resultExpected {
			t.Fatalf("unexpected isStorePath(%q); got %v; want %v", path, result, resultExpected)
		}
	}

	f("data/small/2025_01/0000000000000001/values.bin", true)
	f("data/big/2025_01/18791BCEDF4968E5/metadata.json", true)
	f("data/indexdb/2025_01/18791BCEDF4968E5/items.bin", true)
	f("indexdb/18791BCEDF4968E5/18791BCEDF4968E6/items.bin", true)

	f("data/small/2025_01/parts.json", false)
	f("data/small/2025_01/appliedRetention.txt", false)
	f("indexdb/18791BCEDF4968E5/parts.json", false)
	f("metadata/minTimestampForCompositeIndex", false)
	f("data/small/2025_01/18791bcedf4968e5/values.bin", false)
	f("values.bin", false)
}

func newTestSharedRepo(dir string) *SharedRepo {
	newFS := func(subdir string) (common.RemoteFS, error) {
		return &fsremote.FS{Dir: filepath.Join(dir, filepath.FromSlash(subdir))}, nil
	}
	return &SharedRepo{
		root:  &fsremote.FS{Dir: dir},
		store: &fsremote.FS{Dir: filepath.Join(dir, "store")},
		newFS: newFS,
	}
}

func newTestSnapshot(t *testing.T, dir string, files map[string]string) *fslocal.FS {
	t.Helper()
	snapshotDir := filepath.Join(dir, "snapshots", snapshotutil.NewName())
	for path, data := range files {
		mustWriteFile(t, filepath.Join(snapshotDir, filepath.FromSlash(path)), data)
	}
	fs := &fslocal.FS{Dir: snapshotDir}
	if err := fs.Init(); err != nil {
		t.Fatalf("cannot init fs: %s", err)
	}
	return fs
}

func mustSharedBackup(t *testing.T, dir string, repo *SharedRepo, name string, files map[string]string, keepLast int) {
	t.Helper()
	b := &SharedBackup{
		Concurrency:     2,
		Src:             newTestSnapshot(t, dir, files),
		Repo:            repo,
		Name:            name,
		KeepLastBackups: keepLast,
	}
	if err := b.Run(); err != nil {
		t.Fatalf("cannot make backup %q: %s", name, err)
	}
}

func mustRestore(t *testing.T, srcDir, dstDir string) {
	t.Helper()
	src, err := NewRestoreSrcFS(context.Background(), "fs://"+srcDir)
	if err != nil {
		t.Fatalf("cannot open backup at %q: %s", srcDir, err)
	}
	defer src.MustStop()
	dst := &fslocal.FS{Dir: dstDir}
	if err := dst.Init(); err != nil {
		t.Fatalf("cannot init fs: %s", err)
	}
	r := &Restore{
		Concurrency: 2,
		Src:         src,
		Dst:         dst,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("cannot restore backup from %q: %s", srcDir, err)
	}
}

func mustListParts(t *testing.T, fs common.RemoteFS) []string {
	t.Helper()
	parts, err := fs.ListParts()
	if err != nil {
		t.Fatalf("cannot list parts at %s: %s", fs, err)
	}
	common.SortParts(parts)
	paths := make([]string, len(parts))
	for i, p := range parts {
		paths[i] = p.Path
	}
	return paths
}

func readTestDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == "flock.lock" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("cannot read %q: %s", dir, err)
	}
	return files
}

func mustWriteFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("cannot create dir for %q: %s", path, err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("cannot write %q: %s", path, err)
	}
}

func mustRemoveFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
}
