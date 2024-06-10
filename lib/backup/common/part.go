package common

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Part is an atomic unit for transfer during backup / restore.
//
// Each source file can be split into parts with up to MaxPartSize sizes.
type Part struct {
	// Path is the path to file for backup.
	//
	// Path must consistently use `/` as directory separator.
	// Use ToCanonicalPath() function for converting local directory separators to `/`.
	Path string

	// FileSize is the size of the whole file for the given part.
	FileSize uint64

	// Offset is offset in the file to backup.
	Offset uint64

	// Size is the size of the part to backup starting from Offset.
	Size uint64

	// ActualSize is the actual size of the part.
	//
	// The part is considered broken if it isn't equal to Size.
	// Such a part must be removed from remote storage.
	ActualSize uint64
}

// key returns a string, which uniquely identifies p.
func (p *Part) key() string {
	if strings.HasSuffix(p.Path, "/parts.json") ||
		strings.HasSuffix(p.Path, "/appliedRetention.txt") {
		// parts.json and appliedRetention.txt files contents changes over time,
		// so it must have an unique key in order to always copy it during
		// backup, restore and server-side copy.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5005
		id := uniqueKeyID.Add(1)
		return fmt.Sprintf("unique-%016X", id)
	}
	// Do not use p.FileSize in the key, since it cannot be properly initialized when resuming the restore for partially restored file
	return fmt.Sprintf("%s%016X%016X%016X", p.Path, p.Offset, p.Size, p.ActualSize)
}

var uniqueKeyID atomic.Uint64

// String returns human-readable representation of the part.
func (p *Part) String() string {
	return fmt.Sprintf("part{path: %q, file_size: %d, offset: %d, size: %d}", p.Path, p.FileSize, p.Offset, p.Size)
}

// RemotePath returns remote path for the part p and the given prefix.
func (p *Part) RemotePath(prefix string) string {
	for strings.HasSuffix(prefix, "/") {
		prefix = prefix[:len(prefix)-1]
	}
	return fmt.Sprintf("%s/%s/%016X_%016X_%016X", prefix, p.Path, p.FileSize, p.Offset, p.Size)
}

// LocalPath returns local path for p at the given dir.
func (p *Part) LocalPath(dir string) string {
	path := p.Path
	if filepath.Separator != '/' {
		path = strings.ReplaceAll(path, "/", string(filepath.Separator))
	}
	return filepath.Join(dir, path)
}

// ToCanonicalPath returns canonical path by replacing local directory separators with `/`.
func ToCanonicalPath(path string) string {
	if filepath.Separator == '/' {
		return path
	}
	return strings.ReplaceAll(path, string(filepath.Separator), "/")
}

var partNameRegexp = regexp.MustCompile(`^(.+)[/\\]([0-9A-F]{16})_([0-9A-F]{16})_([0-9A-F]{16})$`)

// ParseFromRemotePath parses p from remotePath.
//
// Returns true on success.
//
// remotePath must be in canonical form received from ToCanonicalPath().
func (p *Part) ParseFromRemotePath(remotePath string) bool {
	tmp := partNameRegexp.FindStringSubmatch(remotePath)
	if len(tmp) != 5 {
		return false
	}
	path := tmp[1]
	for strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	fileSize, err := strconv.ParseUint(tmp[2], 16, 64)
	if err != nil {
		logger.Panicf("BUG: cannot parse fileSize from %q: %s", tmp[2], err)
	}
	offset, err := strconv.ParseUint(tmp[3], 16, 64)
	if err != nil {
		logger.Panicf("BUG: cannot parse offset from %q: %s", tmp[3], err)
	}
	size, err := strconv.ParseUint(tmp[4], 16, 64)
	if err != nil {
		logger.Panicf("BUG: cannot parse size from %q: %s", tmp[4], err)
	}
	p.Path = path
	p.FileSize = fileSize
	p.Offset = offset
	p.Size = size
	return true
}

// MaxPartSize is the maximum size for each part.
//
// The MaxPartSize reduces bandwidth usage during retires on network errors
// when transferring multi-TB files.
const MaxPartSize = 1024 * 1024 * 1024

// SortParts sorts parts by (Path, Offset)
func SortParts(parts []Part) {
	sort.Slice(parts, func(i, j int) bool {
		a := parts[i]
		b := parts[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Offset < b.Offset
	})
}

// PartsDifference returns a - b
func PartsDifference(a, b []Part) []Part {
	m := make(map[string]bool, len(b))
	for _, p := range b {
		k := p.key()
		m[k] = true
	}
	var d []Part
	for _, p := range a {
		k := p.key()
		if !m[k] {
			d = append(d, p)
		}
	}
	return d
}

// PartsIntersect returns the intersection of a and b
func PartsIntersect(a, b []Part) []Part {
	m := make(map[string]bool, len(a))
	for _, p := range a {
		k := p.key()
		m[k] = true
	}
	var d []Part
	for _, p := range b {
		k := p.key()
		if m[k] {
			d = append(d, p)
		}
	}
	return d
}
