package config

import (
	"fmt"
	"io"
	"net/http"
)

// readFromHTTP reads config from http path.
func readFromHTTP(paths []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, path := range paths {
		if _, ok := result[path]; ok {
			return nil, fmt.Errorf("duplicate found for url path %q: url path must be unique", path)
		}
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
			return nil, fmt.Errorf("cannot read %q: %s", path, err)
		}
		result[path] = data
	}
	return result, nil
}
