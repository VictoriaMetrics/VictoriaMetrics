package fslocal

import (
	"fmt"
	"os"

	"github.com/bmatcuk/doublestar/v4"
)

// FS represents a local file system
type FS struct {
	// Pattern is used for matching one or multiple files.
	// The pattern may describe hierarchical names such as
	// /usr/*/bin/ed (assuming the Separator is '/').
	Pattern string
}

// Init verifies that configured Pattern is correct
func (fs *FS) Init() error {
	_, err := doublestar.FilepathGlob(fs.Pattern)
	return err
}

// String implements Stringer interface
func (fs *FS) String() string {
	return fmt.Sprintf("Local FS{MatchPattern: %q}", fs.Pattern)
}

// List returns the list of file names which will be read via Read fn
func (fs *FS) List() ([]string, error) {
	matches, err := doublestar.FilepathGlob(fs.Pattern)
	if err != nil {
		return nil, fmt.Errorf("error while matching files via pattern %s: %w", fs.Pattern, err)
	}
	return matches, nil
}

// Read returns a map of read files where
// key is the file name and value is file's content.
func (fs *FS) Read(files []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("error while reading file %q: %w", path, err)
		}
		result[path] = data
	}
	return result, nil
}
