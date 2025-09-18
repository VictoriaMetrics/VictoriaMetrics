package httputil

import (
	"net/http"
	"strings"
)

// GetArray returns an array of comma-separated values from r with the argKey quey arg or with headerKey header.
func GetArray(r *http.Request, argKey, headerKey string) []string {
	v := GetRequestValue(r, argKey, headerKey)
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}

// GetRequestValue returns r value for the given argKey query arg or for the given headerKey header.
func GetRequestValue(r *http.Request, argKey, headerKey string) string {
	v := r.FormValue(argKey)
	if v == "" {
		v = r.Header.Get(headerKey)
	}
	return v
}
