package main

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

func (up *URLPrefix) mergeURLs(requestURI *url.URL) *url.URL {
	pu := up.getNextURL()
	return mergeURLs(pu, requestURI)
}

func mergeURLs(uiURL, requestURI *url.URL) *url.URL {
	targetURL := *uiURL
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

func createTargetURL(ui *UserInfo, uOrig *url.URL) (*url.URL, []Header, error) {
	u := *uOrig
	// Prevent from attacks with using `..` in r.URL.Path
	u.Path = path.Clean(u.Path)
	if !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	for _, e := range ui.URLMap {
		for _, sp := range e.SrcPaths {
			if sp.match(u.Path) {
				return e.URLPrefix.mergeURLs(&u), e.Headers, nil
			}
		}
	}
	if ui.URLPrefix != nil {
		return ui.URLPrefix.mergeURLs(&u), ui.Headers, nil
	}
	missingRouteRequests.Inc()
	return nil, nil, fmt.Errorf("missing route for %q", u.String())
}
