package flagutil

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// NewURL returns new `url` flag with the given name and description.
//
// The url value is redacted when calling URL.String() in the following way:
// 1. Basic Auth username and password are replaced with "xxxxx"
// 2. Values of GET params matching `secretWordsRe` expression are replaced with "xxxxx".
//
// Call URL.Get() for obtaining original URL address.
func NewURL(name, description string) *URL {
	description += fmt.Sprintf("\nFlag value can be read from the given file when using -%s=file:///abs/path/to/file or -%s=file://./relative/path/to/file . ", name, name)
	u := &URL{
		flagname: name,
	}
	ru := &redactedURL{
		URL:      &url.URL{},
		redacted: "",
	}
	u.value.Store(&ru)
	flag.Var(u, name, description)
	return u
}

// URL is a flag holding URL address
//
// If the flag value is file:///path/to/file,
// then its contents is automatically re-read from the given file on disk.
type URL struct {
	nextRefreshTimestamp atomic.Uint64

	value atomic.Pointer[*redactedURL]

	// flagname is the name of the flag
	flagname string

	// sourcePath contains either url or path to file with the url
	sourcePath string
}

type redactedURL struct {
	*url.URL
	redacted string
}

// Get returns the current u address.
//
// It re-reads u value from the file:///path/to/file
// if they were passed to URL.Set.
func (u *URL) Get() string {
	u.maybeRereadURL()
	ru := *u.value.Load()
	return ru.URL.String()
}

// Get returns the current u redacted address.
//
// It re-reads u value from the file:///path/to/file
// if they were passed to URL.Set.
func (u *URL) String() string {
	u.maybeRereadURL()
	ru := *u.value.Load()
	return ru.redacted
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

	// Re-read value from s.sourcePath
	u.nextRefreshTimestamp.Store(tsCurr + 2)
	data, err := os.ReadFile(u.sourcePath)
	if err != nil {
		// cannot use lib/logger, since it can be uninitialized yet
		log.Printf("flagutil: fall back to the previous url for -%s, since failed to re-read it from %q: cannot read %q: %s\n", u.flagname, u.sourcePath, u.sourcePath, err.Error())
	} else {
		addr := strings.TrimRightFunc(string(data), unicode.IsSpace)
		res, err := newRedactedURL(addr)
		if err != nil {
			log.Printf("flagutil: cannot parse %q: %s\n", u.flagname, err.Error())
			return
		}
		u.value.Store(&res)
	}
}

// Set implements flag.Value interface.
func (u *URL) Set(value string) error {
	u.nextRefreshTimestamp.Store(0)
	var s string
	switch {
	case strings.HasPrefix(value, "file://"):
		u.sourcePath = strings.TrimPrefix(value, "file://")
		data, err := os.ReadFile(u.sourcePath)
		if err != nil {
			// cannot use lib/logger, since it can be uninitialized yet
			return fmt.Errorf("cannot read %q: %w", u.sourcePath, err)
		}
		s = strings.TrimRightFunc(string(data), unicode.IsSpace)
	default:
		u.sourcePath = ""
		s = value
	}

	res, err := newRedactedURL(s)
	if err != nil {
		return fmt.Errorf("cannot parse %q: %s", u.flagname, err)
	}
	u.value.Store(&res)
	return nil
}

var secretWordsRe = regexp.MustCompile("auth|pass|key|secret|token")

func newRedactedURL(s string) (*redactedURL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URL: %s", err)
	}
	ru := &redactedURL{URL: u}

	// copy URL before mutating query params
	u2 := *u
	values := u2.Query()
	for k, vs := range values {
		if secretWordsRe.MatchString(k) {
			for i := range vs {
				vs[i] = "xxxxx"
			}
		}
	}
	u2.RawQuery = values.Encode()
	if _, has := u2.User.Password(); has {
		u2.User = url.UserPassword("xxxxx", "xxxxx")
	}
	ru.redacted = u2.String()
	return ru, nil
}
