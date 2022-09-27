package discoveryutils

import (
	"sync"
	"sync/atomic"
)

// InternString returns interned s.
//
// This may be needed for reducing the amounts of allocated memory.
func InternString(s string) string {
	m := internStringsMap.Load().(*sync.Map)
	if v, ok := m.Load(s); ok {
		sp := v.(*string)
		return *sp
	}
	// Make a new copy for s in order to remove references from possible bigger string s refers to.
	sCopy := string(append([]byte{}, s...))
	m.Store(sCopy, &sCopy)
	n := atomic.AddUint64(&internStringsMapLen, 1)
	if n > 100e3 {
		atomic.StoreUint64(&internStringsMapLen, 0)
		internStringsMap.Store(&sync.Map{})
	}
	return sCopy
}

var (
	internStringsMap    atomic.Value
	internStringsMapLen uint64
)

func init() {
	internStringsMap.Store(&sync.Map{})
}
