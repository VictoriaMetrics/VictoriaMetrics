package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config/fsurl"
)

// FS represent a file system abstract for reading files.
type FS interface {
	// Init initializes FS.
	Init() error

	// String must return human-readable representation of FS.
	String() string

	// List returns the list of file names which will be read via Read fn
	List() ([]string, error)

	// Read returns a list of read files in form of a map
	// where key is a file name and value is a content of read file.
	// Read must be called only after the successful Init call.
	Read(files []string) (map[string][]byte, error)
}

var (
	fsRegistryMu sync.Mutex
	fsRegistry   = make(map[string]FS)
)

// ReadFromFS parses the given path list and inits FS for each item.
// Once initialed, ReadFromFS will try to read and return files from each FS.
// ReadFromFS returns an error if at least one FS failed to init.
// The function can be called multiple times but each unique path
// will be initialed only once.
//
// It is allowed to mix different FS types in path list.
func ReadFromFS(paths []string) (map[string][]byte, error) {
	var err error
	result := make(map[string][]byte)
	for _, path := range paths {
		fsRegistryMu.Lock()
		fs, ok := fsRegistry[path]
		if !ok {
			fs, err = newFS(path)
			if err != nil {
				fsRegistryMu.Unlock()
				return nil, fmt.Errorf("error while parsing path %q: %w", path, err)
			}
			if err := fs.Init(); err != nil {
				fsRegistryMu.Unlock()
				return nil, fmt.Errorf("error while initializing path %q: %w", path, err)
			}
			fsRegistry[path] = fs
		}
		fsRegistryMu.Unlock()

		list, err := fs.List()
		if err != nil {
			return nil, fmt.Errorf("failed to list files from %q", fs)
		}

		cLogger.Infof("found %d files to read from %q", len(list), fs)

		if len(list) < 1 {
			continue
		}

		ts := time.Now()
		files, err := fs.Read(list)
		if err != nil {
			return nil, fmt.Errorf("error while reading files from %q: %w", fs, err)
		}
		cLogger.Infof("finished reading %d files in %v from %q", len(list), time.Since(ts), fs)

		for k, v := range files {
			if _, ok := result[k]; ok {
				return nil, fmt.Errorf("duplicate found for file name %q: file names must be unique", k)
			}
			result[k] = v
		}
	}
	return result, nil
}

// newFS creates FS based on the give path.
// Supported file systems are: fs
func newFS(originPath string) (FS, error) {
	scheme := "fs"
	path := originPath
	n := strings.Index(path, "://")
	if n >= 0 {
		scheme = path[:n]
		path = path[n+len("://"):]
	}
	if len(path) == 0 {
		return nil, fmt.Errorf("path cannot be empty")
	}
	switch scheme {
	case "fs":
		return &fslocal.FS{Pattern: path}, nil
	case "http", "https":
		return &fsurl.FS{Path: originPath}, nil
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}
}
