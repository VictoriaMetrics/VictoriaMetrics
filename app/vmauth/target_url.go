package main

import (
	"net/http"
	"net/url"
	"path"
	"slices"
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

func (ui *UserInfo) getURLPrefixAndHeaders(u *url.URL, h http.Header) (*URLPrefix, HeadersConf) {
	for _, e := range ui.URLMaps {
		if !matchAnyRegex(e.SrcHosts, u.Host) {
			continue
		}
		if !matchAnyRegex(e.SrcPaths, u.Path) {
			continue
		}
		if !matchAnyQueryArg(e.SrcQueryArgs, u.Query()) {
			continue
		}
		if !matchAnyHeader(e.SrcHeaders, h) {
			continue
		}

		return e.URLPrefix, e.HeadersConf
	}
	if ui.URLPrefix != nil {
		return ui.URLPrefix, ui.HeadersConf
	}
	return nil, HeadersConf{}
}

func matchAnyRegex(rs []*Regex, s string) bool {
	if len(rs) == 0 {
		return true
	}
	for _, r := range rs {
		if r.match(s) {
			return true
		}
	}
	return false
}

func matchAnyQueryArg(qas []*QueryArg, args url.Values) bool {
	if len(qas) == 0 {
		return true
	}
	for _, qa := range qas {
		vs, ok := args[qa.Name]
		if !ok {
			continue
		}
		for _, v := range vs {
			if qa.Value.match(v) {
				return true
			}
		}
	}
	return false
}

func matchAnyHeader(headers []*Header, h http.Header) bool {
	if len(headers) == 0 {
		return true
	}
	for _, header := range headers {
		if slices.Contains(h.Values(header.Name), header.Value) {
			return true
		}
	}
	return false
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
