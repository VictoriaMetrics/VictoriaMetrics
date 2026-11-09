package httputil

import (
	"net/http"
	"strings"
)

// GetQueryArgOrHeaderArray returns an array of comma-separated values from r with the argKey query arg or with headerKey header.
func GetQueryArgOrHeaderArray(r *http.Request, argKey, headerKey string) []string {
	v := GetQueryArgOrHeader(r, argKey, headerKey)
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}

// GetQueryArgOrHeader returns r value for the given argKey query arg or for the given headerKey header.
//
// The argKey is read only from the request URL, since the request body may contain data,
// which must be read by the caller. See https://github.com/VictoriaMetrics/VictoriaLogs/issues/1816
func GetQueryArgOrHeader(r *http.Request, argKey, headerKey string) string {
	v := r.URL.Query().Get(argKey)
	if v == "" {
		v = r.Header.Get(headerKey)
	}
	return v
}
