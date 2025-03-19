package storage

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	vmfs "github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

func TestMustOpenLegacyIndexDBTables_noTables(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBIsNil(t, prev)
	assertIndexDBIsNil(t, curr)
}

func TestMustOpenLegacyIndexDBTables_prevOnly(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)

	assertPathsExist(t, prevPath)

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBName(t, prev, prevName)
	assertIndexDBIsNil(t, curr)
}

func TestMustOpenLegacyIndexDBTables_currAndPrev(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)
	currName := "123456789ABCDEF1"
	currPath := filepath.Join(legacyIDBPath, currName)
	vmfs.MustMkdirIfNotExist(currPath)

	assertPathsExist(t, prevPath, currPath)

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBName(t, prev, prevName)
	assertIndexDBName(t, curr, currName)
}

func TestMustOpenLegacyIndexDBTables_nextIsRemoved(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)
	currName := "123456789ABCDEF1"
	currPath := filepath.Join(legacyIDBPath, currName)
	vmfs.MustMkdirIfNotExist(currPath)
	nextName := "123456789ABCDEF2"
	nextPath := filepath.Join(legacyIDBPath, nextName)
	vmfs.MustMkdirIfNotExist(nextPath)

	assertPathsExist(t, prevPath, currPath, nextPath)

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBName(t, prev, prevName)
	assertIndexDBName(t, curr, currName)
	assertPathsDoNotExist(t, nextPath)
}

func TestMustOpenLegacyIndexDBTables_nextAndAbsoleteDirsAreRemoved(t *testing.T) {
	defer testRemoveAll(t)

	legacyIDBPath := t.Name()
	absolete1Name := "123456789ABCDEEE"
	absolete1Path := filepath.Join(legacyIDBPath, absolete1Name)
	vmfs.MustMkdirIfNotExist(absolete1Path)
	absolete2Name := "123456789ABCDEEF"
	absolete2Path := filepath.Join(legacyIDBPath, absolete2Name)
	vmfs.MustMkdirIfNotExist(absolete2Path)
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)
	currName := "123456789ABCDEF1"
	currPath := filepath.Join(legacyIDBPath, currName)
	vmfs.MustMkdirIfNotExist(currPath)
	nextName := "123456789ABCDEF2"
	nextPath := filepath.Join(legacyIDBPath, nextName)
	vmfs.MustMkdirIfNotExist(nextPath)

	assertPathsExist(t, absolete1Path, absolete2Path, prevPath, currPath, nextPath)

	s := Storage{}
	prev, curr := s.mustOpenLegacyIndexDBTables(legacyIDBPath)
	assertIndexDBName(t, prev, prevName)
	assertIndexDBName(t, curr, currName)
	assertPathsDoNotExist(t, absolete1Path, absolete2Path, nextPath)
}

func TestLegacyMustRotateIndexDBs(t *testing.T) {
	defer testRemoveAll(t)

	storagePath := t.Name()
	legacyIDBPath := filepath.Join(storagePath, indexdbDirname)
	prevName := "123456789ABCDEF0"
	prevPath := filepath.Join(legacyIDBPath, prevName)
	vmfs.MustMkdirIfNotExist(prevPath)
	currName := "123456789ABCDEF1"
	currPath := filepath.Join(legacyIDBPath, currName)
	vmfs.MustMkdirIfNotExist(currPath)

	assertPathsExist(t, prevPath, currPath)

	s := MustOpenStorage(storagePath, 0, 0, 0)
	defer s.MustClose()

	var prev, curr *indexDB

	if !s.hasLegacyIDBs() {
		t.Fatalf("storage was expected to have legacy indexDBs but it doesn't")
	}
	prev, curr = s.legacyIDBs()
	assertIndexDBName(t, prev, prevName)
	assertIndexDBName(t, curr, currName)
	assertDirEntries(t, legacyIDBPath, 2, []string{prevName, currName})

	s.legacyMustRotateIndexDB(time.Now())

	if !s.hasLegacyIDBs() {
		t.Fatalf("storage was expected to have legacy indexDBs but it doesn't")
	}
	prev, curr = s.legacyIDBs()
	assertIndexDBName(t, prev, currName)
	assertIndexDBIsNil(t, curr)
	assertPathsDoNotExist(t, prevPath)
	assertPathsExist(t, currPath)
	assertDirEntries(t, legacyIDBPath, 2, []string{currName})

	s.legacyMustRotateIndexDB(time.Now())

	if s.hasLegacyIDBs() {
		t.Fatalf("storage was expected to have no legacy indexDBs but it has them")
	}
	prev, curr = s.legacyIDBs()
	assertIndexDBIsNil(t, prev)
	assertIndexDBIsNil(t, curr)
	assertPathsDoNotExist(t, prevPath, currPath)
	assertDirEntries(t, legacyIDBPath, 2, []string{})
}

func assertPathsExist(t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		if !vmfs.IsPathExist(path) {
			t.Fatalf("path does not exist: %s", path)
		}
	}
}

func assertPathsDoNotExist(t *testing.T, paths ...string) {
	t.Helper()

	for _, path := range paths {
		if vmfs.IsPathExist(path) {
			t.Fatalf("path exists: %s", path)
		}
	}
}

func assertDirEntries(t *testing.T, dir string, depth int, want []string) {
	t.Helper()

	got := []string{}

	f := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Only include entries at the given depth level.
		if strings.Count(path, "/") != depth {
			return nil
		}
		got = append(got, entry.Name())
		return nil
	}
	if err := filepath.WalkDir(dir, f); err != nil {
		t.Fatalf("could not walk dir %q: %v", dir, err)
	}

	slices.Sort(got)
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected dir entries (-want, +got):\n%s", diff)
	}
}

func assertIndexDBName(t *testing.T, idb *indexDB, want string) {
	t.Helper()

	if idb == nil {
		t.Fatalf("unexpected idb: got nil, want non-nil")
	}
	if got := idb.name; got != want {
		t.Errorf("unexpected idb name: got %s, want %s", got, want)
	}
}

func assertIndexDBIsNil(t *testing.T, idb *indexDB) {
	t.Helper()

	if idb != nil {
		t.Fatalf("unexpected idb: got %s, want nil", idb.name)
	}
}
