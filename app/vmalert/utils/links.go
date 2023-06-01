package utils

import (
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
)

const prefix = "/vmalert/"

// Prefix returns "/vmalert/" prefix if it is missing in the path.
func Prefix(path string) string {
	pp := httpserver.GetPathPrefix()
	path = strings.TrimLeft(path, pp)
	if strings.HasPrefix(path, prefix) {
		return pp
	}
	res, err := url.JoinPath(pp, prefix)
	if err != nil {
		return path
	}
	return res
}
