package main

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func mergeURLs(uiURL string, requestURI *url.URL) (string, error) {
	prefixURL, err := url.Parse(uiURL)
	if err != nil {
		return "", fmt.Errorf("BUG - cannot parse userInfo url: %q, err: %w", uiURL, err)
	}
	prefixURL.Path += requestURI.Path
	requestParams := requestURI.Query()
	// fast path
	if len(requestParams) == 0 {
		return prefixURL.String(), nil
	}
	// merge query parameters from requests.
	userInfoParams := prefixURL.Query()
	for k, v := range requestParams {
		// skip clashed query params from original request
		if exist := userInfoParams.Get(k); len(exist) > 0 {
			continue
		}
		for i := range v {
			userInfoParams.Add(k, v[i])
		}
	}
	prefixURL.RawQuery = userInfoParams.Encode()
	return prefixURL.String(), nil
}

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
				return mergeURLs(e.URLPrefix, u)
			}
		}
	}
	if len(ui.URLPrefix) > 0 {
		return mergeURLs(ui.URLPrefix, u)
	}
	return "", fmt.Errorf("missing route for %q", u)
}
