package httputil

import (
	"fmt"
	"net/url"
)

// CheckURL checks whether urlStr contains valid URL.
func CheckURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("url cannot be empty")
	}
	if _, err := url.Parse(urlStr); err != nil {
		return fmt.Errorf("failed to parse url %q: %w", urlStr, err)
	}
	return nil
}
