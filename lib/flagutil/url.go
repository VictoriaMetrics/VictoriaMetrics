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

// NewURL returns new `url` flag with the given name and description.
//
// The url value is hidden when calling URL.String() for security reasons,
// since the returned value can be put in logs.
// Call URL.Get() for obtaining the real url value.
func NewURL(name, description string) *URL {
	description += fmt.Sprintf("\nFlag value can be read from the given file when using -%s=file:///abs/path/to/file or -%s=file://./relative/path/to/file . ", name, name)
	u := &URL{
		flagname: name,
	}
	s := ""
	u.value.Store(&s)
	flag.Var(u, name, description)
	return u
}

// URL is a flag holding a url.
//
// If the flag value is file:///path/to/file,
// then its contents is automatically re-read from the given file.
type URL struct {
	nextRefreshTimestamp atomic.Uint64

	value atomic.Pointer[string]

	// flagname is the name of the flag
	flagname string

	// sourcePath contains either url or path to file with the url
	sourcePath string
}

// Get returns the current u value.
//
// It re-reads u value from the file:///path/to/file
// if they were passed to URL.Set.
func (u *URL) Get() string {
	u.maybeRereadURL()
	sPtr := u.value.Load()
	return *sPtr
}

func (u *URL) maybeRereadURL() {
	if u.sourcePath == "" {
		// Fast path - nothing to re-read
		return
	}
	tsCurr := fasttime.UnixTimestamp()
	tsNext := u.nextRefreshTimestamp.Load()
	if tsCurr < tsNext {
		// Fast path - nothing to re-read
		return
	}

	// Re-read url from s.sourcePath
	u.nextRefreshTimestamp.Store(tsCurr + 2)
	data, err := os.ReadFile(u.sourcePath)
	if err != nil {
		// cannot use lib/logger, since it can be uninitialized yet
		log.Printf("flagutil: fall back to the previous url for -%s, since failed to re-read it from %q: cannot read %q: %s\n", u.flagname, u.sourcePath, u.sourcePath, err.Error())

	} else {
		url := strings.TrimRightFunc(string(data), unicode.IsSpace)
		u.value.Store(&url)
	}
}

// String implements flag.Value interface.
func (u *URL) String() string {
	return "secret"
}

// Set implements flag.Value interface.
func (u *URL) Set(value string) error {
	u.nextRefreshTimestamp.Store(0)
	switch {
	case strings.HasPrefix(value, "file://"):
		u.sourcePath = strings.TrimPrefix(value, "file://")
		data, err := os.ReadFile(u.sourcePath)
		if err != nil {
			// cannot use lib/logger, since it can be uninitialized yet
			return fmt.Errorf("cannot read %q: %w", u.sourcePath, err)
		}
		url := strings.TrimRightFunc(string(data), unicode.IsSpace)
		u.value.Store(&url)
		return nil
	default:
		u.sourcePath = ""
		u.value.Store(&value)
		return nil
	}
}
