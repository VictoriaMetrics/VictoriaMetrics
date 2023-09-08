package main

import (
	"net/url"
	"path"
	"strings"
)

func mergeURLs(uiURL, requestURI *url.URL) *url.URL {
	targetURL := *uiURL
	if strings.HasPrefix(requestURI.Path, "/") {
		targetURL.Path = strings.TrimSuffix(targetURL.Path, "/")
	}
	targetURL.Path += requestURI.Path
	requestParams := requestURI.Query()
	// fast path
	if len(requestParams) == 0 {
		return &targetURL
	}
	// merge query parameters from requests.
	uiParams := targetURL.Query()
	for k, v := range requestParams {
		// skip clashed query params from original request
		if exist := uiParams.Get(k); len(exist) > 0 {
			continue
		}
		for i := range v {
			uiParams.Add(k, v[i])
		}
	}
	targetURL.RawQuery = uiParams.Encode()
	return &targetURL
}

func (ui *UserInfo) getURLPrefixAndHeaders(u *url.URL) (*URLPrefix, HeadersConf, []int) {
	for _, e := range ui.URLMaps {
		for _, sp := range e.SrcPaths {
			if sp.match(u.Path) {
				return e.URLPrefix, e.HeadersConf, e.RetryStatusCodes
			}
		}
	}
	if ui.URLPrefix != nil {
		return ui.URLPrefix, ui.HeadersConf, ui.RetryStatusCodes
	}
	return nil, HeadersConf{}, nil
}

func normalizeURL(uOrig *url.URL) *url.URL {
	u := *uOrig
	// Prevent from attacks with using `..` in r.URL.Path
	u.Path = path.Clean(u.Path)
	if !strings.HasSuffix(u.Path, "/") && strings.HasSuffix(uOrig.Path, "/") {
		// The path.Clean() removes trailing slash.
		// Return it back if needed.
		// This should fix https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1752
		u.Path += "/"
	}
	if !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}
	if u.Path == "/" {
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/1554
		u.Path = ""
	}
	return &u
}
