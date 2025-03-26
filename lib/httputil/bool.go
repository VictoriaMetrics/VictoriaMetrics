package httputil

import (
	"net/http"
	"strings"
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
