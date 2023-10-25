package fsurl

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// FS represents a struct which can read content from URL Path
type FS struct {
	// Path defines the URL to read the data from
	Path string
}

// Init verifies that configured Path is correct
func (fs *FS) Init() error {
	_, err := url.Parse(fs.Path)
	return err
}

// String implements Stringer interface
func (fs *FS) String() string {
	return fmt.Sprintf("URL {Path: %q}", fs.Path)
}

// List returns the list of file names which will be read via Read fn
// List isn't supported by FS and reads from Path only
func (fs *FS) List() ([]string, error) {
	return []string{fs.Path}, nil
}

// Read returns a map of read files where
// key is the file name and value is file's content.
func (fs *FS) Read(files []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, path := range files {
		resp, err := http.Get(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read from %q: %w", path, err)
		}
		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if len(data) > 4*1024 {
				data = data[:4*1024]
			}
			return nil, fmt.Errorf("unexpected status code when fetching %q: %d, expecting %d; response: %q",
				path, resp.StatusCode, http.StatusOK, data)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot read %q: %w", path, err)
		}
		result[path] = data
	}
	return result, nil
}
