package persistentqueue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestQueueCleanOnStart(t *testing.T) {
	requireQueueCleanupSupport(t)
	path := t.TempDir()
	q := mustOpen(path, "destination", 0)
	q.MustWriteBlock([]byte("old data"))
	q.MustClose()
	writeCleanupMarker(t, path)
	q = mustOpen(path, "destination", 0)
	if n := q.GetPendingBytes(); n != 0 {
		q.MustClose()
		t.Fatalf("cleanup left %d pending bytes", n)
	}
	q.MustWriteBlock([]byte("new data"))
	q.MustClose()
	for range 3 {
		q = mustOpen(path, "destination", 0)
		if n := q.GetPendingBytes(); n != uint64(len("new data")+8) {
			q.MustClose()
			t.Fatalf("restart lost newly written data; pending bytes=%d", n)
		}
		q.MustClose()
	}
	if _, err := os.Lstat(filepath.Join(path, cleanOnStartFilename)); !os.IsNotExist(err) {
		t.Fatalf("cleanup marker still exists: %v", err)
	}
}

func TestQueueCleanOnStartAbsentAndEmpty(t *testing.T) {
	path := t.TempDir()
	q := mustOpen(path, "destination", 0)
	q.MustWriteBlock([]byte("keep"))
	q.MustClose()
	q = mustOpen(path, "destination", 0)
	b, ok := q.MustReadBlockNonblocking(nil)
	if !ok || string(b) != "keep" {
		t.Fatalf("unmarked queue changed: %q, %v", b, ok)
	}
	q.MustClose()
	writeCleanupMarker(t, path)
	if runtime.GOOS == "windows" {
		mustPanicCleanup(t, func() { mustOpen(path, "destination", 0) })
		return
	}
	q = mustOpen(path, "destination", 0)
	defer q.MustClose()
	if q.GetPendingBytes() != 0 {
		t.Fatal("marked empty queue is not empty")
	}
}

func TestQueueCleanOnStartInvalidMarker(t *testing.T) {
	f := func(kind string, create func(string) error) {
		t.Helper()
		t.Run(kind, func(t *testing.T) {
			path := t.TempDir()
			q := mustOpen(path, "destination", 0)
			q.MustWriteBlock([]byte("preserve"))
			q.MustClose()
			marker := filepath.Join(path, cleanOnStartFilename)
			if err := create(marker); err != nil {
				t.Fatal(err)
			}
			// Exercise the public must-open boundary, not only the helper: its
			// corruption fallback must not remove an invalid cleanup request.
			mustPanicCleanup(t, func() { mustOpen(path, "destination", 0) })
			if _, err := os.Lstat(marker); err != nil {
				t.Fatalf("invalid marker was removed: %s", err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			q = mustOpen(path, "destination", 0)
			defer q.MustClose()
			b, ok := q.MustReadBlockNonblocking(nil)
			if !ok || string(b) != "preserve" {
				t.Fatalf("invalid marker lost data: %q, %v", b, ok)
			}
		})
	}
	f("nonempty", func(p string) error { return os.WriteFile(p, []byte("true"), 0600) })
	f("directory", func(p string) error { return os.Mkdir(p, 0700) })
	if runtime.GOOS != "windows" {
		f("symlink", func(p string) error { return os.Symlink("missing", p) })
		if os.Geteuid() != 0 {
			f("unreadable", func(p string) error { return os.WriteFile(p, nil, 0000) })
		}
	}
}

func TestQueueCleanOnStartPartialDeletion(t *testing.T) {
	path := newCleanupQueue(t)
	lock := fs.MustCreateFlockFile(path)
	flockInfo, err := lock.Stat()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.MustClose(lock)
	injected := errors.New("injected removal failure")
	removed := 0
	ops := queueCleanupFS{remove: func(p string) error {
		if chunkFileNameRegex.MatchString(filepath.Base(p)) {
			removed++
			if removed == 2 {
				return injected
			}
		}
		return os.Remove(p)
	}, writeAtomic: fs.MustWriteAtomic, syncPath: fs.MustSyncPath}
	if err := cleanupQueueOnStartWithFS(path, "destination", ops); !errors.Is(err, injected) {
		t.Fatalf("cleanup didn't propagate deletion error: %v", err)
	}
	if removed != 2 {
		t.Fatalf("didn't reach partial deletion: removed=%d", removed)
	}
	if _, err := os.Stat(filepath.Join(path, cleanOnStartFilename)); err != nil {
		t.Fatalf("pending request lost: %s", err)
	}
	if err := cleanupQueueOnStart(path, "destination"); err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(filepath.Join(path, fs.FlockFilename))
	if err != nil || !os.SameFile(flockInfo, current) {
		t.Fatalf("lock inode changed: %v", err)
	}
}

func TestQueueCleanOnStartMarkerRemovalError(t *testing.T) {
	path := newCleanupQueue(t)
	lock := fs.MustCreateFlockFile(path)
	defer fs.MustClose(lock)
	injected := errors.New("injected marker removal failure")
	ops := queueCleanupFS{remove: func(p string) error {
		if filepath.Base(p) == cleanOnStartFilename {
			return injected
		}
		return os.Remove(p)
	}, writeAtomic: fs.MustWriteAtomic, syncPath: fs.MustSyncPath}
	if err := cleanupQueueOnStartWithFS(path, "destination", ops); !errors.Is(err, injected) {
		t.Fatalf("cleanup didn't propagate marker removal error: %v", err)
	}
	if err := cleanupQueueOnStart(path, "destination"); err != nil {
		t.Fatalf("cannot retry cleanup: %s", err)
	}
}

func TestQueueCleanOnStartFilesystemFailures(t *testing.T) {
	f := func(operation string, failAt int, markerConsumed bool) {
		t.Helper()
		t.Run(fmt.Sprintf("%s-%d", operation, failAt), func(t *testing.T) {
			path := newCleanupQueue(t)
			lock := fs.MustCreateFlockFile(path)
			injected := errors.New("injected filesystem failure")
			calls := 0
			fail := func() {
				calls++
				if calls == failAt {
					panic(injected)
				}
			}
			ops := queueCleanupFS{remove: os.Remove, writeAtomic: fs.MustWriteAtomic, syncPath: fs.MustSyncPath}
			if operation == "write" {
				ops.writeAtomic = func(p string, b []byte, overwrite bool) { fail(); fs.MustWriteAtomic(p, b, overwrite) }
			} else {
				ops.syncPath = func(p string) { fail(); fs.MustSyncPath(p) }
			}
			func() {
				defer func() {
					if p := recover(); p != injected {
						t.Fatalf("filesystem failure wasn't propagated: %v", p)
					}
				}()
				if err := cleanupQueueOnStartWithFS(path, "destination", ops); err != nil {
					t.Fatal(err)
				}
			}()
			fs.MustClose(lock)
			_, err := os.Stat(filepath.Join(path, cleanOnStartFilename))
			if os.IsNotExist(err) != markerConsumed {
				t.Fatalf("unexpected marker state after failure: %v", err)
			}
			q := mustOpenInternal(path, "destination", 32, 16, 0)
			if q.GetPendingBytes() != 0 {
				q.MustClose()
				t.Fatal("recovery retained old data")
			}
			q.MustWriteBlock([]byte("fresh"))
			q.MustClose()
			q = mustOpenInternal(path, "destination", 32, 16, 0)
			defer q.MustClose()
			b, ok := q.MustReadBlockNonblocking(nil)
			if !ok || string(b) != "fresh" {
				t.Fatalf("recovery lost fresh data: %q, %v", b, ok)
			}
		})
	}
	f("write", 1, false)
	f("write", 2, false)
	f("sync", 1, false)
	f("sync", 2, false)
	f("sync", 3, false)
	f("sync", 4, true)
}

func TestQueueCleanOnStartUnexpectedFiles(t *testing.T) {
	path := newCleanupQueue(t)
	unexpected := filepath.Join(path, "0000000000000060")
	if err := os.Mkdir(unexpected, 0700); err != nil {
		t.Fatal(err)
	}
	lock := fs.MustCreateFlockFile(path)
	if err := cleanupQueueOnStart(path, "destination"); err == nil {
		t.Fatal("cleanup accepted a directory with a chunk filename")
	}
	if err := os.Remove(unexpected); err != nil {
		t.Fatal(err)
	}
	// Unrecognized files are not queue chunks, and must not be deleted.
	unknown := filepath.Join(path, "operator-note")
	if err := os.WriteFile(unknown, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cleanupQueueOnStart(path, "destination"); err != nil {
		t.Fatal(err)
	}
	fs.MustClose(lock)
	if b, err := os.ReadFile(unknown); err != nil || string(b) != "keep" {
		t.Fatalf("cleanup changed unrelated file: %q, %v", b, err)
	}
}

func TestQueueCleanOnStartIsolation(t *testing.T) {
	requireQueueCleanupSupport(t)
	root := t.TempDir()
	p1, p2 := filepath.Join(root, "1_A"), filepath.Join(root, "2_B")
	for _, path := range []string{p1, p2} {
		q := MustOpenFastQueue(path, path, 0, 0, false)
		if !q.TryWriteBlock([]byte(path)) {
			t.Fatal("cannot write queue")
		}
		q.MustClose()
	}
	writeCleanupMarker(t, p1)
	q1 := MustOpenFastQueue(p1, p1, 0, 0, false)
	defer q1.MustClose()
	if q1.GetPendingBytes() != 0 {
		t.Fatal("marked destination retained data")
	}
	q2 := MustOpenFastQueue(p2, p2, 0, 0, false)
	defer q2.MustClose()
	b, ok := q2.MustReadBlock(nil)
	if !ok || string(b) != p2 {
		t.Fatalf("other destination changed: %q, %v", b, ok)
	}
}

func TestQueueCleanOnStartCrashBoundaries(t *testing.T) {
	// First enumerate actual filesystem operations, rather than hardcoding
	// their count. Then crash a separate process before AND after each one.
	path := newCleanupQueue(t)
	var steps []string
	ops := queueCleanupFS{
		remove: func(p string) error { steps = append(steps, "remove "+filepath.Base(p)); return os.Remove(p) },
		writeAtomic: func(p string, b []byte, overwrite bool) {
			steps = append(steps, "write "+filepath.Base(p))
			fs.MustWriteAtomic(p, b, overwrite)
		},
		syncPath: func(p string) { steps = append(steps, "sync "+filepath.Base(p)); fs.MustSyncPath(p) },
	}
	lock := fs.MustCreateFlockFile(path)
	if err := cleanupQueueOnStartWithFS(path, "destination", ops); err != nil {
		t.Fatal(err)
	}
	fs.MustClose(lock)
	for i, step := range steps {
		for _, when := range []string{"before", "after"} {
			t.Run(fmt.Sprintf("%d-%s-%s", i, when, step), func(t *testing.T) {
				path := newCleanupQueue(t)
				cmd := cleanupHelperCommand(t, path, "crash", strconv.Itoa(i), when)
				output, err := cmd.CombinedOutput()
				var ee *exec.ExitError
				if !errors.As(err, &ee) || ee.ExitCode() != 42 {
					t.Fatalf("helper didn't crash at boundary: %v\n%s", err, output)
				}
				q := mustOpenInternal(path, "destination", 32, 16, 0)
				if q.GetPendingBytes() != 0 {
					q.MustClose()
					t.Fatal("interrupted cleanup reopened old data")
				}
				q.MustWriteBlock([]byte("fresh"))
				q.MustClose()
				for range 2 {
					q = mustOpenInternal(path, "destination", 32, 16, 0)
					if q.GetPendingBytes() != uint64(len("fresh")+8) {
						q.MustClose()
						t.Fatal("repeated restart deleted fresh data")
					}
					q.MustClose()
				}
			})
		}
	}
}

func TestQueueCleanOnStartActiveWriter(t *testing.T) {
	path := t.TempDir()
	q := mustOpen(path, "destination", 0)
	q.MustWriteBlock([]byte("old"))
	writeCleanupMarker(t, path)
	output, err := cleanupHelperCommand(t, path, "open", "", "").CombinedOutput()
	expectedError := "exclusive access"
	if runtime.GOOS == "windows" {
		expectedError = "not supported on Windows"
	}
	if err == nil || !strings.Contains(string(output), expectedError) {
		q.MustClose()
		t.Fatalf("competing opener wasn't rejected by flock: %v\n%s", err, output)
	}
	q.MustWriteBlock([]byte("still active"))
	for _, want := range []string{"old", "still active"} {
		b, ok := q.MustReadBlockNonblocking(nil)
		if !ok || string(b) != want {
			q.MustClose()
			t.Fatalf("active queue modified: got %q, want %q", b, want)
		}
	}
	q.MustClose()
	if _, err := os.Stat(filepath.Join(path, cleanOnStartFilename)); err != nil {
		t.Fatalf("competing opener consumed marker: %s", err)
	}
}

func TestQueueCleanupProcessHelper(t *testing.T) {
	path := os.Getenv("VM_QUEUE_CLEANUP_TEST_PATH")
	if path == "" {
		return
	}
	if os.Getenv("VM_QUEUE_CLEANUP_TEST_MODE") == "open" {
		q := mustOpen(path, "destination", 0)
		q.MustClose()
		return
	}
	stop, err := strconv.Atoi(os.Getenv("VM_QUEUE_CLEANUP_TEST_STEP"))
	if err != nil {
		t.Fatal(err)
	}
	when := os.Getenv("VM_QUEUE_CLEANUP_TEST_WHEN")
	i := 0
	boundary := func(w string) {
		if i == stop && w == when {
			os.Exit(42)
		}
	}
	ops := queueCleanupFS{
		remove: func(p string) error { boundary("before"); err := os.Remove(p); boundary("after"); i++; return err },
		writeAtomic: func(p string, b []byte, overwrite bool) {
			boundary("before")
			fs.MustWriteAtomic(p, b, overwrite)
			boundary("after")
			i++
		},
		syncPath: func(p string) { boundary("before"); fs.MustSyncPath(p); boundary("after"); i++ },
	}
	lock := fs.MustCreateFlockFile(path)
	defer fs.MustClose(lock)
	if err := cleanupQueueOnStartWithFS(path, "destination", ops); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash boundary was not reached")
}

func cleanupHelperCommand(t *testing.T, path, mode, step, when string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQueueCleanupProcessHelper$")
	cmd.Env = append(os.Environ(), "VM_QUEUE_CLEANUP_TEST_PATH="+path, "VM_QUEUE_CLEANUP_TEST_MODE="+mode,
		"VM_QUEUE_CLEANUP_TEST_STEP="+step, "VM_QUEUE_CLEANUP_TEST_WHEN="+when)
	return cmd
}

func newCleanupQueue(t *testing.T) string {
	t.Helper()
	requireQueueCleanupSupport(t)
	path := t.TempDir()
	q := mustOpenInternal(path, "destination", 32, 16, 0)
	for range 3 {
		q.MustWriteBlock([]byte("old block"))
	}
	q.MustClose()
	writeCleanupMarker(t, path)
	return path
}

func writeCleanupMarker(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(path, cleanOnStartFilename), nil, 0600); err != nil {
		t.Fatal(err)
	}
}

func requireQueueCleanupSupport(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("one-shot cleanup is rejected on Windows")
	}
}

func mustPanicCleanup(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		p := recover()
		if p == nil || !strings.Contains(fmt.Sprint(p), "cannot clean persistent queue") {
			t.Fatalf("expected cleanup failure, got %v", p)
		}
	}()
	f()
}
