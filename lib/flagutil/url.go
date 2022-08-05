package flagutil

import (
	"fmt"
	"net/url"
	"strings"
)

var urlFlags = make(map[string]bool)

// RegisterURLFlag registers flagName as a URL.
//
// This function must be called before starting logging.
// The underlying map is not safe to use concurrently.
//
// URL flags have user:pass removed before logging.
func RegisterURLFlag(flagName string) {
	lname := strings.ToLower(flagName)
	urlFlags[lname] = true
}

// IsURLFlag returns true if flag contains a flag name where the value
// should be treated as a URL.
func IsURLFlag(flagName string) bool {
	lname := strings.ToLower(flagName)
	if strings.Contains(lname, "url") {
		return true
	}
	return urlFlags[lname]
}

// RedactURLPassword redacts the password for a given url flag
func RedactURLFlagPassword(flagValue string) string {
	parsed, err := url.Parse(flagValue)
	if err != nil {
		return flagValue // not parsable, so do nothing
	}
	if parsed.User != nil {
		if pass, _ := parsed.User.Password(); pass != "" {
			parsed.User = url.UserPassword(parsed.User.Username(), "REDACTED")
		}
	}
	return fmt.Sprint(parsed)
}
