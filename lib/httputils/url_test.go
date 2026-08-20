package httputils

import (
	"net/url"
	"testing"
)

func TestRedactedURL(t *testing.T) {
	tests := []struct {
		name     string
		inputURL string
		expected string
	}{
		{
			name:     "empty URL",
			inputURL: "",
			expected: "",
		},
		{
			name:     "no secrets",
			inputURL: "https://example.com/path/to/resource",
			expected: "https://example.com/path/to/resource",
		},
		{
			name:     "secret query parameter",
			inputURL: "https://example.com/path/to/resource?authKey=foobar",
			expected: "https://example.com/path/to/resource?authKey=xxxxx",
		},
		{
			name:     "secret query parameters (case insensitive)",
			inputURL: "https://example.com/path/to/resource?TOKEN=foobar",
			expected: "https://example.com/path/to/resource?TOKEN=xxxxx",
		},
		{
			name:     "with basic auth secrets",
			inputURL: "https://username:secretPassword@example.com/path/to/resource",
			expected: "https://xxxxx:xxxxx@example.com/path/to/resource",
		},
		{
			name:     "with basic auth and query parameters secrets",
			inputURL: "https://username:secretPassword@example.com/path/to/resource?authKey=foobar",
			expected: "https://xxxxx:xxxxx@example.com/path/to/resource?authKey=xxxxx",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsedURL, err := url.Parse(test.inputURL)
			if err != nil {
				t.Errorf("Unexpected error %v", err)
				return
			}
			actual := RedactedURL(parsedURL)
			if actual != test.expected {
				t.Errorf("Expected: %s, Actual: %s", test.expected, actual)
			}
		})
	}
}
