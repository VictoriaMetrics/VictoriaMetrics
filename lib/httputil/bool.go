package httputil

import (
	"flag"
	"net/http"
	"strings"
)

var (
	denyPartialResponse = flag.Bool("search.denyPartialResponse", false, "Whether to deny partial responses if a part of -storageNode instances fail to perform queries; "+
		"this trades availability over consistency; see also -search.maxQueryDuration")
)

// GetBool returns boolean value from the given argKey query arg.
func GetBool(r *http.Request, argKey string) bool {
	argValue := r.FormValue(argKey)
	switch strings.ToLower(argValue) {
	case "", "0", "f", "false", "no":
		return false
	default:
		return true
	}
}

// GetDenyPartialResponse returns whether partial responses are denied.
func GetDenyPartialResponse(r *http.Request) bool {
	if *denyPartialResponse {
		return true
	}
	if r == nil {
		return false
	}
	return GetBool(r, "deny_partial_response")
}
