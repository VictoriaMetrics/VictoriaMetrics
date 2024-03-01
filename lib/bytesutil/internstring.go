package bytesutil

import (
	"flag"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

var (
	internStringMaxLen = flag.Int("internStringMaxLen", 500, "The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. "+
		"See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration")
	disableCache = flag.Bool("internStringDisableCache", false, "Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. "+
		"See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen")
	cacheExpireDuration = flag.Duration("internStringCacheExpireDuration", 6*time.Minute, "The expiry duration for caches for interned strings. "+
		"See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache")
)

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
	if isSkipCache(s) {
		// Make a new copy for s in order to remove references from possible bigger string s refers to.
		// This also protects from cases when s points to unsafe string - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3227
		return strings.Clone(s)
	}

	ct := fasttime.UnixTimestamp()
	if v, ok := internStringsMap.Load(s); ok {
		e := v.(*ismEntry)
		if e.lastAccessTime.Load()+10 < ct {
			// Reduce the frequency of e.lastAccessTime update to once per 10 seconds
			// in order to improve the fast path speed on systems with many CPU cores.
			e.lastAccessTime.Store(ct)
		}
		return e.s
	}
	// Make a new copy for s in order to remove references from possible bigger string s refers to.
	sCopy := strings.Clone(s)
	e := &ismEntry{
		s: sCopy,
	}
	e.lastAccessTime.Store(ct)
	internStringsMap.Store(sCopy, e)

	if needCleanup(&internStringsMapLastCleanupTime, ct) {
		// Perform a global cleanup for internStringsMap by removing items, which weren't accessed during the last 5 minutes.
		m := &internStringsMap
		deadline := ct - uint64(cacheExpireDuration.Seconds())
		m.Range(func(k, v interface{}) bool {
			e := v.(*ismEntry)
			if e.lastAccessTime.Load() < deadline {
				m.Delete(k)
			}
			return true
		})
	}

	return sCopy
}

type ismEntry struct {
	lastAccessTime atomic.Uint64
	s              string
}

var (
	internStringsMap                sync.Map
	internStringsMapLastCleanupTime atomic.Uint64
)
