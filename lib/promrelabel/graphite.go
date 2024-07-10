package promrelabel

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

var graphiteMatchesPool = &sync.Pool{
	New: func() any {
		return &graphiteMatches{}
	},
}

type graphiteMatches struct {
	a []string
}

type graphiteMatchTemplate struct {
	sOrig string
	parts []string
}

func (gmt *graphiteMatchTemplate) String() string {
	return gmt.sOrig
}

type graphiteLabelRule struct {
	grt         *graphiteReplaceTemplate
	targetLabel string
}

func (glr graphiteLabelRule) String() string {
	return fmt.Sprintf("replaceTemplate=%s, targetLabel=%s", glr.grt, glr.targetLabel)
}

func newGraphiteLabelRules(m map[string]string) []graphiteLabelRule {
	a := make([]graphiteLabelRule, 0, len(m))
	for labelName, replaceTemplate := range m {
		a = append(a, graphiteLabelRule{
			grt:         newGraphiteReplaceTemplate(replaceTemplate),
			targetLabel: labelName,
		})
	}
	return a
}

func newGraphiteMatchTemplate(s string) *graphiteMatchTemplate {
	sOrig := s
	var parts []string
	for {
		n := strings.IndexByte(s, '*')
		if n < 0 {
			parts = appendGraphiteMatchTemplateParts(parts, s)
			break
		}
		parts = appendGraphiteMatchTemplateParts(parts, s[:n])
		parts = appendGraphiteMatchTemplateParts(parts, "*")
		s = s[n+1:]
	}
	return &graphiteMatchTemplate{
		sOrig: sOrig,
		parts: parts,
	}
}

func appendGraphiteMatchTemplateParts(dst []string, s string) []string {
	if len(s) == 0 {
		// Skip empty part
		return dst
	}
	return append(dst, s)
}

// Match matches s against gmt.
//
// On success it adds matched captures to dst and returns it with true.
// On failure it returns false.
func (gmt *graphiteMatchTemplate) Match(dst []string, s string) ([]string, bool) {
	dst = append(dst, s)
	parts := gmt.parts
	if len(parts) > 0 {
		if p := parts[len(parts)-1]; p != "*" && !strings.HasSuffix(s, p) {
			// fast path - suffix mismatch
			return dst, false
		}
	}
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p != "*" {
			if !strings.HasPrefix(s, p) {
				// Cannot match the current part
				return dst, false
			}
			s = s[len(p):]
			continue
		}
		// Search for the matching substring for '*' part.
		if i+1 >= len(parts) {
			// Matching the last part.
			if strings.IndexByte(s, '.') >= 0 {
				// The '*' cannot match string with dots.
				return dst, false
			}
			dst = append(dst, s)
			return dst, true
		}
		// Search for the start of the next part.
		p = parts[i+1]
		i++
		n := strings.Index(s, p)
		if n < 0 {
			// Cannot match the next part
			return dst, false
		}
		tmp := s[:n]
		if strings.IndexByte(tmp, '.') >= 0 {
			// The '*' cannot match string with dots.
			return dst, false
		}
		dst = append(dst, tmp)
		s = s[n+len(p):]
	}
	return dst, len(s) == 0
}

type graphiteReplaceTemplate struct {
	sOrig string
	parts []graphiteReplaceTemplatePart
}

func (grt *graphiteReplaceTemplate) String() string {
	return grt.sOrig
}

type graphiteReplaceTemplatePart struct {
	n int
	s string
}

func newGraphiteReplaceTemplate(s string) *graphiteReplaceTemplate {
	sOrig := s
	var parts []graphiteReplaceTemplatePart
	for {
		n := strings.IndexByte(s, '$')
		if n < 0 {
			parts = appendGraphiteReplaceTemplateParts(parts, s, -1)
			break
		}
		if n > 0 {
			parts = appendGraphiteReplaceTemplateParts(parts, s[:n], -1)
		}
		s = s[n+1:]
		if len(s) > 0 && s[0] == '{' {
			// The index in the form ${123}
			n = strings.IndexByte(s, '}')
			if n < 0 {
				parts = appendGraphiteReplaceTemplateParts(parts, "$"+s, -1)
				break
			}
			idxStr := s[1:n]
			s = s[n+1:]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				parts = appendGraphiteReplaceTemplateParts(parts, "${"+idxStr+"}", -1)
			} else {
				parts = appendGraphiteReplaceTemplateParts(parts, "${"+idxStr+"}", idx)
			}
		} else {
			// The index in the form $123
			n := 0
			for n < len(s) && s[n] >= '0' && s[n] <= '9' {
				n++
			}
			idxStr := s[:n]
			s = s[n:]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				parts = appendGraphiteReplaceTemplateParts(parts, "$"+idxStr, -1)
			} else {
				parts = appendGraphiteReplaceTemplateParts(parts, "$"+idxStr, idx)
			}
		}
	}
	return &graphiteReplaceTemplate{
		sOrig: sOrig,
		parts: parts,
	}
}

// Expand expands grt with the given matches into dst and returns it.
func (grt *graphiteReplaceTemplate) Expand(dst []byte, matches []string) []byte {
	for _, part := range grt.parts {
		if n := part.n; n >= 0 && n < len(matches) {
			dst = append(dst, matches[n]...)
		} else {
			dst = append(dst, part.s...)
		}
	}
	return dst
}

func appendGraphiteReplaceTemplateParts(dst []graphiteReplaceTemplatePart, s string, n int) []graphiteReplaceTemplatePart {
	if len(s) > 0 {
		dst = append(dst, graphiteReplaceTemplatePart{
			s: s,
			n: n,
		})
	}
	return dst
}
