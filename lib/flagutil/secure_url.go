package flagutil

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// NewSecureURL returns new `url` flag with the given name and description.
//
// The url value is hidden when calling SecureURL.String() for security reasons,
// since the returned value can be put in logs.
// Call SecureURL.Get() for obtaining the real url value.
func NewSecureURL(name, description string) *SecureURL {
	description += fmt.Sprintf("\nFlag value can be read from the given file when using -%s=file:///abs/path/to/file or -%s=file://./relative/path/to/file . ", name, name)
	p := &SecureURL{
		flagname: name,
	}
	s := ""
	p.value.Store(&s)
	flag.Var(p, name, description)
	return p
}

// SecureURL  is a flag holding a url.
//
// If the flag value is file:///path/to/file,
// then its contents is automatically re-read from the given file.
type SecureURL struct {
	nextRefreshTimestamp atomic.Uint64

	value atomic.Pointer[string]

	// flagname is the name of the flag
	flagname string

	// sourcePath contains either url or path to file with the url
	sourcePath string
}

// Get returns the current p value.
//
// It re-reads s value from the file:///path/to/file
// if they were passed to SecureURL .Set.
func (s *SecureURL) Get() string {
	s.maybeRereadURL()
	sPtr := s.value.Load()
	return *sPtr
}

func (s *SecureURL) maybeRereadURL() {
	if s.sourcePath == "" {
		// Fast path - nothing to re-read
		return
	}
	tsCurr := fasttime.UnixTimestamp()
	tsNext := s.nextRefreshTimestamp.Load()
	if tsCurr < tsNext {
		// Fast path - nothing to re-read
		return
	}

	// Re-read url from s.sourcePath
	s.nextRefreshTimestamp.Store(tsCurr + 2)
	data, err := os.ReadFile(s.sourcePath)
	if err != nil {
		// cannot use lib/logger, since it can be uninitialized yet
		log.Printf("flagutil: fall back to the previous url for -%s, since failed to re-read it from %q: %s\n", s.flagname, s.sourcePath, fmt.Errorf("cannot read %q: %w", s.sourcePath, err))

	} else {
		url := strings.TrimRightFunc(string(data), unicode.IsSpace)
		s.value.Store(&url)
	}
}

// String implements flag.Value interface.
func (s *SecureURL) String() string {
	return "secret"
}

// Set implements flag.Value interface.
func (s *SecureURL) Set(value string) error {
	s.nextRefreshTimestamp.Store(0)
	switch {
	case strings.HasPrefix(value, "file://"):
		s.sourcePath = strings.TrimPrefix(value, "file://")
		data, err := os.ReadFile(s.sourcePath)
		if err != nil {
			// cannot use lib/logger, since it can be uninitialized yet
			return fmt.Errorf("cannot read %q: %w", s.sourcePath, err)
		}
		url := strings.TrimRightFunc(string(data), unicode.IsSpace)
		s.value.Store(&url)
		return nil
	default:
		s.sourcePath = ""
		s.value.Store(&value)
		return nil
	}
}
