package bytesutil

import (
	"flag"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

var (
	internStringMaxLen = flag.Int("internStringMaxLen", 500, "The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. "+
		"See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration")
	disableCache = flag.Bool("internStringDisableCache", false, "Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. "+
		"See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen")
	cacheExpireDuration = flag.Duration("internStringCacheExpireDuration", 6*time.Minute, "The expiry duration for caches for interned strings. "+
		"See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache")
)

type internStringMap struct {
	mutableLock  sync.Mutex
	mutable      map[string]string
	mutableReads uint64

	readonly atomic.Pointer[map[string]internStringMapEntry]
}

type internStringMapEntry struct {
	deadline uint64
	s        string
}

func newInternStringMap() *internStringMap {
	m := &internStringMap{
		mutable: make(map[string]string),
	}
	readonly := make(map[string]internStringMapEntry)
	m.readonly.Store(&readonly)

	go func() {
		cleanupInterval := timeutil.AddJitterToDuration(*cacheExpireDuration) / 2
		ticker := time.NewTicker(cleanupInterval)
		for range ticker.C {
			m.cleanup()
		}
	}()

	return m
}

func (m *internStringMap) getReadonly() map[string]internStringMapEntry {
	return *m.readonly.Load()
}

func (m *internStringMap) intern(s string) string {
	if isSkipCache(s) {
		return strings.Clone(s)
	}

	readonly := m.getReadonly()
	e, ok := readonly[s]
	if ok {
		// Fast path - the string has been found in readonly map.
		return e.s
	}

	// Slower path - search for the string in mutable map under the lock.
	m.mutableLock.Lock()
	sInterned, ok := m.mutable[s]
	if !ok {
		// Verify whether s has been already registered by concurrent goroutines in m.readonly
		readonly = m.getReadonly()
		e, ok = readonly[s]
		if !ok {
			// Slowest path - register the string in mutable map.
			// Make a new copy for s in order to remove references from possible bigger string s refers to.
			sInterned = strings.Clone(s)
			m.mutable[sInterned] = sInterned
		} else {
			sInterned = e.s
		}
	}
	m.mutableReads++
	if m.mutableReads > uint64(len(readonly)) {
		m.migrateMutableToReadonlyLocked()
		m.mutableReads = 0
	}
	m.mutableLock.Unlock()

	return sInterned
}

func (m *internStringMap) migrateMutableToReadonlyLocked() {
	readonly := m.getReadonly()
	readonlyCopy := make(map[string]internStringMapEntry, len(readonly)+len(m.mutable))
	for k, e := range readonly {
		readonlyCopy[k] = e
	}
	deadline := fasttime.UnixTimestamp() + uint64(cacheExpireDuration.Seconds()+0.5)
	for k, s := range m.mutable {
		readonlyCopy[k] = internStringMapEntry{
			s:        s,
			deadline: deadline,
		}
	}
	m.mutable = make(map[string]string)
	m.readonly.Store(&readonlyCopy)
}

func (m *internStringMap) cleanup() {
	readonly := m.getReadonly()
	currentTime := fasttime.UnixTimestamp()
	needCleanup := false
	for _, e := range readonly {
		if e.deadline <= currentTime {
			needCleanup = true
			break
		}
	}
	if !needCleanup {
		return
	}

	readonlyCopy := make(map[string]internStringMapEntry, len(readonly))
	for k, e := range readonly {
		if e.deadline > currentTime {
			readonlyCopy[k] = e
		}
	}
	m.readonly.Store(&readonlyCopy)
}

func isSkipCache(s string) bool {
	return *disableCache || len(s) > *internStringMaxLen
}

// InternBytes interns b as a string
func InternBytes(b []byte) string {
	s := ToUnsafeString(b)
	return InternString(s)
}

// InternString returns interned s.
//
// This may be needed for reducing the amounts of allocated memory.
func InternString(s string) string {
	return ism.intern(s)
}

var ism = newInternStringMap()
