//go:build go1.20
// +build go1.20

package gzhttp

// shouldWrite1xxResponses indicates whether the current build supports writes of 1xx status codes.
func shouldWrite1xxResponses() bool {
	return true
}
