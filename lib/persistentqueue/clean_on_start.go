package persistentqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

const cleanOnStartFilename = "clean_on_start"

// cleanupQueueOnStart must run with the queue's flock held, before opening any
// reader or writer. The lock file and the queue directory must never be removed.
func cleanupQueueOnStart(path, name string) error {
	return cleanupQueueOnStartWithFS(path, name, queueCleanupFS{
		remove:      os.Remove,
		writeAtomic: fs.MustWriteAtomic,
		syncPath:    fs.MustSyncPath,
	})
}

// Keep the filesystem operations explicit so tests can interrupt cleanup at
// each durability boundary without changing process-wide filesystem functions.
type queueCleanupFS struct {
	remove      func(string) error
	writeAtomic func(string, []byte, bool)
	syncPath    func(string)
}

func cleanupQueueOnStartWithFS(path, name string, ops queueCleanupFS) error {
	markerPath := filepath.Join(path, cleanOnStartFilename)
	fi, err := os.Lstat(markerPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot inspect cleanup marker %q: %w", markerPath, err)
	}
	if runtime.GOOS == "windows" {
		// Windows has no directory fsync in lib/fs, and its current lock-file
		// implementation doesn't exclude a second queue opener. Do not risk
		// deleting data from an active queue or replaying a consumed request.
		return fmt.Errorf("queue cleanup marker %q is not supported on Windows", markerPath)
	}
	if !fi.Mode().IsRegular() || fi.Size() != 0 {
		return fmt.Errorf("cleanup marker %q must be an empty regular file", markerPath)
	}
	// Check readability as well as the file type. Never follow a marker symlink.
	// The operator must not modify the marker or queue during startup.
	f, err := os.Open(markerPath)
	if err != nil {
		return fmt.Errorf("cannot open cleanup marker %q: %w", markerPath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("cannot close cleanup marker %q: %w", markerPath, err)
	}

	des, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("cannot list queue %q for cleanup: %w", path, err)
	}
	// Reject unexpected chunk types before deleting anything. Do not recurse
	// into directories or follow symlinks, and leave unrelated files alone.
	for _, de := range des {
		if chunkFileNameRegex.MatchString(de.Name()) && !de.Type().IsRegular() {
			return fmt.Errorf("cannot clean non-regular queue chunk %q", filepath.Join(path, de.Name()))
		}
	}

	// Persist the request before destructive work. A crash before its durable
	// removal below must retry cleanup, never open a partially reset queue.
	ops.syncPath(markerPath)
	ops.syncPath(path)
	for _, de := range des {
		if !chunkFileNameRegex.MatchString(de.Name()) {
			continue
		}
		chunkPath := filepath.Join(path, de.Name())
		if err := ops.remove(chunkPath); err != nil {
			return fmt.Errorf("cannot remove queue chunk %q: %w", chunkPath, err)
		}
	}

	// These atomic writes sync both the files and their containing directory.
	// Keep the marker until the empty queue is durable, including old chunk
	// deletions. Retrying either write after an interruption is harmless.
	mi := metainfo{Name: name}
	// Use the same metainfo format as normal queue creation.
	data, err := json.Marshal(&mi)
	if err != nil {
		return fmt.Errorf("cannot marshal empty queue metainfo: %w", err)
	}
	ops.writeAtomic(filepath.Join(path, metainfoFilename), data, true)
	ops.writeAtomic(filepath.Join(path, fmt.Sprintf("%016X", 0)), nil, true)
	ops.syncPath(path)
	if err := ops.remove(markerPath); err != nil {
		return fmt.Errorf("cannot remove cleanup marker %q: %w", markerPath, err)
	}
	// No new data may be admitted until this unlink is durable; otherwise a
	// hardware reset could resurrect the marker and delete newly written data.
	ops.syncPath(path)
	return nil
}
