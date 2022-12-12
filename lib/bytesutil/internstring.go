package bytesutil

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// InternString returns interned s.
//
// This may be needed for reducing the amounts of allocated memory.
func InternString(s string) string {
	ct := fasttime.UnixTimestamp()
	if v, ok := internStringsMap.Load(s); ok {
		e := v.(*ismEntry)
		if atomic.LoadUint64(&e.lastAccessTime)+10 < ct {
			// Reduce the frequency of e.lastAccessTime update to once per 10 seconds
			// in order to improve the fast path speed on systems with many CPU cores.
			atomic.StoreUint64(&e.lastAccessTime, ct)
		}
		return e.s
	}
	// Make a new copy for s in order to remove references from possible bigger string s refers to.
	sCopy := strings.Clone(s)
	e := &ismEntry{
		lastAccessTime: ct,
		s:              sCopy,
	}
	internStringsMap.Store(sCopy, e)

	if atomic.LoadUint64(&internStringsMapLastCleanupTime)+61 < ct {
		// Perform a global cleanup for internStringsMap by removing items, which weren't accessed
		// during the last 5 minutes.
		atomic.StoreUint64(&internStringsMapLastCleanupTime, ct)
		m := &internStringsMap
		m.Range(func(k, v interface{}) bool {
			e := v.(*ismEntry)
			if atomic.LoadUint64(&e.lastAccessTime)+5*60 < ct {
				m.Delete(k)
			}
			return true
		})
	}

	return sCopy
}

type ismEntry struct {
	lastAccessTime uint64
	s              string
}

var (
	internStringsMap                sync.Map
	internStringsMapLastCleanupTime uint64
)
