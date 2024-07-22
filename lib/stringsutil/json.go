package stringsutil

import (
	"github.com/valyala/quicktemplate"
)

// JSONString returns JSON-quoted s.
func JSONString(s string) string {
	return string(quicktemplate.AppendJSONString(nil, s, true))
}
