package main

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func createTargetURL(ui *UserInfo, uOrig *url.URL) (string, error) {
	u, err := url.Parse(uOrig.String())
	if err != nil {
		return "", fmt.Errorf("cannot make a copy of %q: %w", u, err)
	}
	// Prevent from attacks with using `..` in r.URL.Path
	u.Path = path.Clean(u.Path)
	if !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}
	for _, e := range ui.URLMap {
		for _, sp := range e.SrcPaths {
			if sp.match(u.Path) {
				return e.URLPrefix + u.RequestURI(), nil
			}
		}
	}
	if len(ui.URLPrefix) > 0 {
		return ui.URLPrefix + u.RequestURI(), nil
	}
	return "", fmt.Errorf("missing route for %q", u)
}
