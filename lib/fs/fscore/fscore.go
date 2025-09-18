package fscore

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

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
		// reads remote file via http or https, if url is given
		resp, err := http.Get(path)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch %q: %w", path, err)
		}
		data, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if len(data) > 4*1024 {
				data = data[:4*1024]
			}
			return nil, fmt.Errorf("unexpected status code when fetching %q: %d, expecting %d; response: %q", path, resp.StatusCode, http.StatusOK, data)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot read %q: %w", path, err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
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
