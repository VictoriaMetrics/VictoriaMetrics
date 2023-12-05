package main

import (
	"net/url"
	"path"
	"strings"
)

func mergeURLs(uiURL, requestURI *url.URL, dropSrcPathPrefixParts int) *url.URL {
	targetURL := *uiURL
	srcPath := dropPrefixParts(requestURI.Path, dropSrcPathPrefixParts)
	if strings.HasPrefix(srcPath, "/") {
		targetURL.Path = strings.TrimSuffix(targetURL.Path, "/")
	}
	targetURL.Path += srcPath
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

func dropPrefixParts(path string, parts int) string {
	if parts <= 0 {
		return path
	}
	for parts > 0 {
		path = strings.TrimPrefix(path, "/")
		n := strings.IndexByte(path, '/')
		if n < 0 {
			return ""
		}
		path = path[n:]
		parts--
	}
	return path
}

func (ui *UserInfo) getURLPrefixAndHeaders(u *url.URL) (*URLPrefix, HeadersConf, []int, int) {
	for _, e := range ui.URLMaps {
		for _, sp := range e.SrcPaths {
			if sp.match(u.Path) {
				return e.URLPrefix, e.HeadersConf, e.RetryStatusCodes, e.DropSrcPathPrefixParts
			}
		}
	}
	if ui.URLPrefix != nil {
		return ui.URLPrefix, ui.HeadersConf, ui.RetryStatusCodes, ui.DropSrcPathPrefixParts
	}
	return nil, HeadersConf{}, nil, 0
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
