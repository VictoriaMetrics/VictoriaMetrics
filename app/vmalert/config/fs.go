package config

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config/fslocal"
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

// readFromFSOrHTTP reads path either from filesystem or from http if path starts with http or https.
// when reading from filesystem, parses the given path list and inits FS for each item.
// Once initialed, readFromFSOrHTTP will try to read and return files from each FS.
// readFromFSOrHTTP returns an error if at least one FS failed to init.
// The function can be called multiple times but each unique path
// will be initialed only once.
//
// It is allowed to mix different FS types and url in path list.
func readFromFSOrHTTP(paths []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, path := range paths {
		if isHTTPURL(path) {
			// reads remote file via http or https, if url is given
			resp, err := http.Get(path)
			if err != nil {
				return nil, fmt.Errorf("cannot fetch %q: %w", path, err)
			}
			data, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				if len(data) > 4*1024 {
					data = data[:4*1024]
				}
				return nil, fmt.Errorf("unexpected status code when fetching %q: %d, expecting %d; response: %q", path, resp.StatusCode, http.StatusOK, data)
			}
			if err != nil {
				return nil, fmt.Errorf("cannot read %q: %s", path, err)
			}
			result[path] = data
			continue
		}

		var err error
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
func newFS(path string) (FS, error) {
	scheme := "fs"
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
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}
}

// isHTTPURL checks if a given targetURL is valid and contains a valid http scheme
func isHTTPURL(targetURL string) bool {
	parsed, err := url.Parse(targetURL)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
