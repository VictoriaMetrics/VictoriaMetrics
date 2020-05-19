package main

import (
	"net/url"
	"path"
	"strings"
)

func createTargetURL(prefix string, u *url.URL) string {
	// Prevent from attacks with using `..` in r.URL.Path
	u.Path = path.Clean(u.Path)
	if !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}
	return prefix + u.RequestURI()
}
