package prefixfilter

import (
	"slices"
	"strings"
	"sync"
)

// GetFilter returns Filter from the pool.
//
// Return the filter to the pool via PutFilter when it is no longer needed.
func GetFilter() *Filter {
	v := filterPool.Get()
	if v == nil {
		return &Filter{}
	}
	return v.(*Filter)
}

// PutFilter returns f to the pool.
//
// f cannot be used after PutFilter call.
func PutFilter(f *Filter) {
	f.Reset()
	filterPool.Put(f)
}

var filterPool sync.Pool

// Filter allow filtering by full strings and by prefixes ending with '*'.
type Filter struct {
	allow filter
	deny  filter
}

// Reset resets f to the initial zero state.
func (f *Filter) Reset() {
	f.allow.reset()
	f.deny.reset()
}

// MatchString returns true if s matches f.
func (f *Filter) MatchString(s string) bool {
	if f == nil {
		return false
	}
	if !f.allow.matchString(s) {
		return false
	}
	return !f.deny.matchString(s)
}

// AddAllowFilters adds the given filters to allowlist at f.
//
// Every filter may end with '*'. In this case it matches all the strings starting with the prefix before '*'.
func (f *Filter) AddAllowFilters(filters []string) {
	for _, filter := range filters {
		f.AddAllowFilter(filter)
	}
}

// AddAllowFilter adds the given filter to allowlist at f.
//
// The filter may end with '*'. In this case it matches all the strings starting with the prefix before '*'.
func (f *Filter) AddAllowFilter(filter string) {
	f.allow.addFilter(filter)
	f.deny.removeFilter(filter)
}

// AddDenyFilters adds the given filters to denylist at f.
//
// Every filter may end with '*'. In this case it stops matching all the strings starting with the prefix before '*'.
func (f *Filter) AddDenyFilters(filters []string) {
	for _, filter := range filters {
		f.AddDenyFilter(filter)
	}
}

// AddDenyFilter adds the the given filter to denylist at f.
//
// Every filter may end with '*'. In this case it stops matching all the strings starting with the prefix before '*'.
func (f *Filter) AddDenyFilter(filter string) {
	f.allow.removeFilter(filter)
	f.deny.addFilter(filter)
}

type filter struct {
	fullStrings []string
	wildcards   []string
}

func (f *filter) reset() {
	f.fullStrings = f.fullStrings[:0]
	f.wildcards = f.wildcards[:0]
}

func (f *filter) addFilter(filter string) {
	if !IsWildcardFilter(filter) {
		f.addFullString(filter)
		return
	}

	wildcard := filter[:len(filter)-1]
	for _, wc := range f.wildcards {
		if strings.HasPrefix(wildcard, wc) {
			// Stronger wildcard is already registered
			return
		}
	}

	f.dropWildcard(wildcard)
	f.wildcards = append(f.wildcards, wildcard)
}

func (f *filter) removeFilter(filter string) {
	if !IsWildcardFilter(filter) {
		f.removeFullString(filter)
		return
	}

	wildcard := filter[:len(filter)-1]
	f.dropWildcard(wildcard)
}

func (f *filter) dropWildcard(wildcard string) {
	// drop the wildcard together with weaker wildcards
	newWildcards := f.wildcards[:0]
	for _, wc := range f.wildcards {
		if !strings.HasPrefix(wc, wildcard) {
			newWildcards = append(newWildcards, wc)
		}
	}
	f.wildcards = newWildcards

	// drop fill strings matching the wildcard
	newFullStrings := f.fullStrings[:0]
	for _, s := range f.fullStrings {
		if !strings.HasPrefix(s, wildcard) {
			newFullStrings = append(newFullStrings, s)
		}
	}
	f.fullStrings = newFullStrings
}

func (f *filter) addFullString(s string) {
	if !f.matchString(s) {
		f.fullStrings = append(f.fullStrings, s)
	}
}

func (f *filter) removeFullString(s string) {
	if !slices.Contains(f.fullStrings, s) {
		// The s cannot be removed from wildcards
		return
	}

	newFullStrings := f.fullStrings[:0]
	for _, f := range f.fullStrings {
		if f != s {
			newFullStrings = append(newFullStrings, f)
		}
	}
	f.fullStrings = newFullStrings
}

func (f *filter) matchString(s string) bool {
	if len(f.wildcards) == 0 && len(f.fullStrings) == 0 {
		/// Fast path for common case when there are no filters.
		return false
	}

	// Slower path for regular case.
	for _, wildcard := range f.wildcards {
		if strings.HasPrefix(s, wildcard) {
			return true
		}
	}
	return slices.Contains(f.fullStrings, s)
}

// IsWildcardFilter returns true if the filter ends with '*', e.g. it matches any string containing the prefix in front of '*'.
func IsWildcardFilter(filter string) bool {
	return strings.HasSuffix(filter, "*")
}

// MatchFilter returns true if s matches filter.
func MatchFilter(filter, s string) bool {
	if !IsWildcardFilter(filter) {
		return filter == s
	}
	wildcard := filter[:len(filter)-1]
	return strings.HasPrefix(s, wildcard)
}

// MatchFilters returns true if s matches any filter from filters.
func MatchFilters(filters []string, s string) bool {
	for _, filter := range filters {
		if MatchFilter(filter, s) {
			return true
		}
	}
	return false
}

// AppendReplace replaces srcFilter prefix with dstFilter prefix at s, appends the result to dst and returns it.
func AppendReplace(dst []byte, srcFilter, dstFilter, s string) []byte {
	if !IsWildcardFilter(srcFilter) {
		if s == srcFilter {
			return append(dst, dstFilter...)
		}
		return append(dst, s...)
	}

	srcPrefix := srcFilter[:len(srcFilter)-1]
	if !strings.HasPrefix(s, srcPrefix) {
		return append(dst, s...)
	}
	if !IsWildcardFilter(dstFilter) {
		return append(dst, dstFilter...)
	}

	dstPrefix := dstFilter[:len(dstFilter)-1]
	dst = append(dst, dstPrefix...)
	return append(dst, s[len(srcPrefix):]...)
}
