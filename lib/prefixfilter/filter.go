package prefixfilter

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

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

// Clone returns a copy of f.
func (f *Filter) Clone() *Filter {
	var dst Filter
	dst.allow.copyFrom(&f.allow)
	dst.deny.copyFrom(&f.deny)
	return &dst
}

// String returns human-readable representation of f.
func (f *Filter) String() string {
	allow := f.GetAllowFilters()
	deny := f.GetDenyFilters()

	return fmt.Sprintf("allow=[%s], deny=[%s]", joinQuotedStrings(allow), joinQuotedStrings(deny))
}

func joinQuotedStrings(a []string) string {
	tmp := make([]string, len(a))
	for i, s := range a {
		tmp[i] = strconv.Quote(s)
	}
	return strings.Join(tmp, ",")
}

// GetAllowStrings returns the list of allow strings if there are no wildcard filters in the allow list
//
// It returns false if there are wildcard filters
func (f *Filter) GetAllowStrings() ([]string, bool) {
	if len(f.allow.wildcards) == 0 {
		return f.allow.fullStrings, true
	}
	return nil, false
}

// GetAllowFilters returns allow filters from f.
func (f *Filter) GetAllowFilters() []string {
	return f.allow.getFilters()
}

// GetDenyFilters returns deny filters from f.
func (f *Filter) GetDenyFilters() []string {
	return f.deny.getFilters()
}

// MatchNothing returns true if f doesn't match anything.
func (f *Filter) MatchNothing() bool {
	return f.allow.matchNothing()
}

// MatchAll returns true if f matches any string.
func (f *Filter) MatchAll() bool {
	if !f.allow.matchAll() {
		return false
	}
	return f.deny.matchNothing()
}

// MatchStringOrWildcard returns true if s matches f.
//
// s may be either a regular string or a wildcard ending with '*'.
// If s is a wildcard, then true is returned if at least a single string matching this wildcard matches f.
func (f *Filter) MatchStringOrWildcard(s string) bool {
	if !IsWildcardFilter(s) {
		return f.MatchString(s)
	}

	wildcard := s[:len(s)-1]
	if !f.allow.matchWildcardFilter(wildcard) {
		return false
	}
	return !f.deny.matchWildcard(wildcard)
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

func (f *Filter) normalize() {
	if len(f.allow.wildcards) == 0 {
		f.deny.reset()
	}
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
	f.deny.removeFilter(filter, true)

	f.normalize()
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
	if !f.MatchStringOrWildcard(filter) {
		// Nothing to deny.
		return
	}

	f.allow.removeFilter(filter, false)
	f.deny.addFilter(filter)

	f.normalize()
}

type filter struct {
	fullStrings []string
	wildcards   []string
}

func (f *filter) reset() {
	f.fullStrings = f.fullStrings[:0]
	f.wildcards = f.wildcards[:0]
}

func (f *filter) copyFrom(src *filter) {
	f.fullStrings = append(f.fullStrings[:0], src.fullStrings...)
	f.wildcards = append(f.wildcards[:0], src.wildcards...)
}

func (f *filter) getFilters() []string {
	filters := append([]string{}, f.fullStrings...)
	for _, wc := range f.wildcards {
		filter := wc + "*"
		filters = append(filters, filter)
	}
	sort.Strings(filters)
	return filters
}

func (f *filter) matchAll() bool {
	return slices.Contains(f.wildcards, "")
}

func (f *filter) matchNothing() bool {
	return len(f.fullStrings) == 0 && len(f.wildcards) == 0
}

func (f *filter) addFilter(filter string) {
	if !IsWildcardFilter(filter) {
		f.addFullString(filter)
		return
	}

	wildcard := filter[:len(filter)-1]
	f.addWildcard(wildcard)
}

func (f *filter) addWildcard(wildcard string) {
	if !f.matchWildcard(wildcard) {
		f.dropWildcard(wildcard)
		f.wildcards = append(f.wildcards, wildcard)
	}
}

func (f *filter) removeFilter(filter string, removeBroaderWildcards bool) {
	if !IsWildcardFilter(filter) {
		f.removeFullString(filter)
	} else {
		wildcard := filter[:len(filter)-1]
		f.dropWildcard(wildcard)
	}

	if removeBroaderWildcards {
		s := strings.TrimSuffix(filter, "*")
		newWildcards := f.wildcards[:0]
		for _, wc := range f.wildcards {
			if !strings.HasPrefix(s, wc) {
				newWildcards = append(newWildcards, wc)
			}
		}
		f.wildcards = newWildcards
	}
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

	// drop full strings matching the wildcard
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
	if f.matchNothing() {
		/// Fast path for common case when there are no filters.
		return false
	}

	// Slower path for regular case.
	if f.matchWildcard(s) {
		return true
	}
	return slices.Contains(f.fullStrings, s)
}

func (f *filter) matchWildcardFilter(wildcard string) bool {
	for _, wc := range f.wildcards {
		if strings.HasPrefix(wildcard, wc) || strings.HasPrefix(wc, wildcard) {
			return true
		}
	}
	for _, s := range f.fullStrings {
		if strings.HasPrefix(s, wildcard) {
			return true
		}
	}
	return false
}

func (f *filter) matchWildcard(wildcard string) bool {
	for _, wc := range f.wildcards {
		if strings.HasPrefix(wildcard, wc) {
			return true
		}
	}
	return false
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

// MatchAll returns true if filters match any string
func MatchAll(filters []string) bool {
	return slices.Contains(filters, "*")
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

	srcSuffix := s[len(srcPrefix):]
	dstPrefix := dstFilter[:len(dstFilter)-1]

	dstLen := len(dst)
	dst = slicesutil.SetLength(dst, dstLen+len(dstPrefix)+len(srcSuffix))
	copy(dst[dstLen:], dstPrefix)
	copy(dst[dstLen+len(dstPrefix):], srcSuffix)
	return dst
}
