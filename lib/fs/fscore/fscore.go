package fscore

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	httpFetchTimeout = 60 * time.Second
	// maxHTTPBodySize caps a config or rule file fetched over HTTP.
	maxHTTPBodySize = 64 * 1024 * 1024
)

var httpFetchClient = &http.Client{
	Timeout: httpFetchTimeout,
}

// ReadPasswordFromFileOrHTTP reads password for the give path.
//
// The path can be an url - then the password is read from url.
func ReadPasswordFromFileOrHTTP(path string) (string, error) {
	data, err := ReadFileOrHTTP(path)
	if err != nil {
		return "", err
	}
	pass := strings.TrimRightFunc(string(data), unicode.IsSpace)
	return pass, nil
}

// ReadFileOrHTTP reads path either from local filesystem or from http if path starts with http or https.
func ReadFileOrHTTP(path string) ([]byte, error) {
	if isHTTPURL(path) {
		return readHTTPBody(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}
	return data, nil
}

func readHTTPBody(path string) ([]byte, error) {
	resp, err := httpFetchClient.Get(path)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch %q: %w", path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		if len(data) > 4*1024 {
			data = data[:4*1024]
		}
		return nil, fmt.Errorf("unexpected status code when fetching %q: %d, expecting %d; response: %q", path, resp.StatusCode, http.StatusOK, data)
	}
	if len(data) > maxHTTPBodySize {
		return nil, fmt.Errorf("too big response from %q: exceeds %d bytes", path, maxHTTPBodySize)
	}
	return data, nil
}

// GetFilepath returns full path to file for the given baseDir and path.
func GetFilepath(baseDir, path string) string {
	if filepath.IsAbs(path) || isHTTPURL(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

// isHTTPURL checks if a given targetURL is valid and contains a valid http scheme
func isHTTPURL(targetURL string) bool {
	parsed, err := url.Parse(targetURL)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""

}
