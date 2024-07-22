package flagutil

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
)

// NewPassword returns new `password` flag with the given name and description.
//
// The password value is hidden when calling Password.String() for security reasons,
// since the returned value can be put in logs.
// Call Password.Get() for obtaining the real password value.
func NewPassword(name, description string) *Password {
	description += fmt.Sprintf("\nFlag value can be read from the given file when using -%s=file:///abs/path/to/file or -%s=file://./relative/path/to/file . "+
		"Flag value can be read from the given http/https url when using -%s=http://host/path or -%s=https://host/path", name, name, name, name)
	p := &Password{
		flagname: name,
	}
	s := ""
	p.value.Store(&s)
	flag.Var(p, name, description)
	return p
}

// Password is a flag holding a password.
//
// If the flag value is file:///path/to/file or http://host/path ,
// then its contents is automatically re-read from the given file or url
type Password struct {
	nextRefreshTimestamp atomic.Uint64

	value atomic.Pointer[string]

	// flagname is the name of the flag
	flagname string

	// sourcePath contains either url or path to file with the password
	sourcePath string
}

// Name returns the name of p flag.
func (p *Password) Name() string {
	return p.flagname
}

// Get returns the current p value.
//
// It re-reads p value from the file:///path/to/file or http://host/path
// if they were passed to Password.Set.
func (p *Password) Get() string {
	p.maybeRereadPassword()
	sPtr := p.value.Load()
	return *sPtr
}

func (p *Password) maybeRereadPassword() {
	if p.sourcePath == "" {
		// Fast path - nothing to re-read
		return
	}
	tsCurr := fasttime.UnixTimestamp()
	tsNext := p.nextRefreshTimestamp.Load()
	if tsCurr < tsNext {
		// Fast path - nothing to re-read
		return
	}

	// Re-read password from p.sourcePath
	p.nextRefreshTimestamp.Store(tsCurr + 2)
	s, err := fscore.ReadPasswordFromFileOrHTTP(p.sourcePath)
	if err != nil {
		// cannot use lib/logger, since it can be uninitialized yet
		log.Printf("flagutil: fall back to the previous password for -%s, since failed to re-read it from %q: %s\n", p.flagname, p.sourcePath, err)
	} else {
		p.value.Store(&s)
	}
}

// String implements flag.Value interface.
func (p *Password) String() string {
	return "secret"
}

// Set implements flag.Value interface.
func (p *Password) Set(value string) error {
	p.nextRefreshTimestamp.Store(0)
	switch {
	case strings.HasPrefix(value, "file://"):
		p.sourcePath = strings.TrimPrefix(value, "file://")
		// Do not attempt to read the password from sourcePath now, since the file may not exist yet.
		// The password will be read on the first access via Password.Get.
		// Generate a random password for now in order to prevent from unauthorized access to protected resources
		// while the sourcePath file doesn't exist.
		p.initRandomValue()
		return nil
	case strings.HasPrefix(value, "http://"), strings.HasPrefix(value, "https://"):
		p.sourcePath = value
		// Do not attempt to read the password from sourcePath now, since the url may now exist yet.
		// The password will be read on the first access via Password.Get.
		// Generate a random password for now in order to prevent from unauthorized access to protected resources
		// while the sourcePath file doesn't exist.
		p.initRandomValue()
		return nil
	default:
		p.sourcePath = ""
		p.value.Store(&value)
		return nil
	}
}

func (p *Password) initRandomValue() {
	var buf [64]byte
	_, err := io.ReadFull(rand.Reader, buf[:])
	if err != nil {
		// cannot use lib/logger here, since it can be uninitialized yet
		panic(fmt.Errorf("FATAL: cannot read random data: %s", err))
	}
	s := string(buf[:])
	p.value.Store(&s)
}
