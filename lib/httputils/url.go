package httputils

import (
	"net/url"
	"regexp"
	"strings"
)

var secretWordsRe = regexp.MustCompile("auth|pass|key|secret|token")

// RedactedURL redacts sensitive information like basic auth credentials
// and query parameters containing sensitive info from a URL object (*url.URL).
// It searches for query parameter names  containing words commonly associated
// with authentication credentials (like "auth", "pass", "key", "secret", or "token").
// These words are matched in a case-insensitive manner. If there is a match, such sensitive information will be mased with 'xxxxx'
func RedactedURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	ru := *u
	values := ru.Query()
	for k, vs := range values {
		if secretWordsRe.MatchString(strings.ToLower(k)) {
			for i := range vs {
				vs[i] = "xxxxx"
			}
		}
	}
	ru.RawQuery = values.Encode()
	if _, has := ru.User.Password(); has {
		ru.User = url.UserPassword("xxxxx", "xxxxx")
	}
	return ru.String()
}
