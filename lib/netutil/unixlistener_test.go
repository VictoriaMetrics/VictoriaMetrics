package netutil

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUnixListenerSuccess(t *testing.T) {
	f := func(addr string) {
		t.Helper()

		uln, err := NewUnixListener(t.Name(), addr)
		if err != nil {
			t.Fatalf("unexpected error creating unix listener: %s", err)
		}
		defer uln.Close()

		if _, err := os.Stat(addr); err != nil {
			t.Fatalf("unexpected error stating unix address: %s", err)
		}
	}

	tmpDir := socketTempDir(t)

	// Socket file does not exist.
	socketPath := filepath.Join(tmpDir, "1.sock")
	f(socketPath)

	// Socket file exists, no live listeners.
	socketPath = filepath.Join(tmpDir, "2.sock")
	createStaleSocket(t, socketPath)
	f(socketPath)
}

func TestUnixListenerFailure(t *testing.T) {
	f := func(addr string) {
		t.Helper()

		uln, err := NewUnixListener(t.Name(), addr)
		if err == nil {
			_ = uln.Close()
			t.Fatalf("expecting non-nil error, got nil")
		}

		// Ensure file wasn't deleted.
		if _, err := os.Stat(addr); err != nil {
			t.Fatalf("cannot stat unix socket: %s", err)
		}
	}

	tmpDir := socketTempDir(t)

	// Must fail when existing socket file returns permissions denied error.
	if os.Geteuid() == 0 {
		// Current user is root.
		// Skip this test case, since permission checks are bypassed for root.
		t.Logf("skipping permissions denied test, since current user is root")
	} else {
		socketPath := filepath.Join(tmpDir, "1.sock")
		createStaleSocket(t, socketPath)
		if err := os.Chmod(socketPath, 0111); err != nil {
			t.Fatalf("cannot chmod socket %q: %s", socketPath, err)
		}
		f(socketPath)
	}

	// Socket file already has a listener.
	socketPath := filepath.Join(tmpDir, "2.sock")
	uln, err := NewUnixListener(t.Name(), socketPath)
	if err != nil {
		t.Fatalf("cannot create unix listener: %s", err)
	}
	f(socketPath)
	_ = uln.Close()

	// Non-socket file exists at the socket path.
	socketPath = filepath.Join(tmpDir, "3.txt")
	file, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("cannot create file: %s", err)
	}
	_ = file.Close()
	f(socketPath)
}

func createStaleSocket(t *testing.T, addr string) {
	t.Helper()
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: addr, Net: "unix"})
	if err != nil {
		t.Fatalf("cannot create stale socket %q: %s", addr, err)
	}
	// Keep the socket file after close in order to emulate unclean shutdown.
	l.SetUnlinkOnClose(false)
	if err := l.Close(); err != nil {
		t.Fatalf("cannot close unix listener: %s", err)
	}
}

// socketTempDir returns a temporary directory for unix socket files.
//
// t.TempDir() is placed under TMPDIR, which can make socket paths longer than
// the sun_path limit (104 bytes on macOS, 108 on Linux). Fall back to /tmp in this case
// on Unix systems, where /tmp is always available.
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11705
func socketTempDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	if runtime.GOOS == "windows" || len(filepath.Join(tmpDir, "1.sock")) < 100 {
		return tmpDir
	}
	tmpDir, err := os.MkdirTemp("/tmp", "vm-unixlistener-")
	if err != nil {
		t.Fatalf("cannot create temporary directory for unix sockets: %s", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})
	return tmpDir
}
