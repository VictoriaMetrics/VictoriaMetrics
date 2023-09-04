package httputils

import (
	"net/http"
	"strings"
)

// GetArray returns an array of comma-separated values from r arg with the argKey name.
func GetArray(r *http.Request, argKey string) []string {
	v := r.FormValue(argKey)
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}
