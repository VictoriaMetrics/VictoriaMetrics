package main

import (
	"net/http"
	"testing"
)

func TestRequestToCurl(t *testing.T) {
	f := func(req *http.Request, exp string) {
		got := requestToCurl(req)
		if got != exp {
			t.Fatalf("expected to have %q; got %q instead", exp, got)
		}
	}

	req, _ := http.NewRequest(http.MethodPost, "foo.com", nil)
	f(req, "curl -X POST 'http://foo.com'")

	req, _ = http.NewRequest(http.MethodGet, "https://foo.com", nil)
	f(req, "curl -k -X GET 'https://foo.com'")

	req, _ = http.NewRequest(http.MethodPost, "foo.com", nil)
	req.Header.Set("foo", "bar")
	req.Header.Set("baz", "qux")
	f(req, "curl -X POST -H 'Baz: qux' -H 'Foo: bar' 'http://foo.com'")

	req, _ = http.NewRequest(http.MethodPost, "foo.com", nil)
	params := req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -X POST 'http://foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "http://foo.com", nil)
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -X POST 'http://foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "https://foo.com", nil)
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -k -X POST 'https://foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "https://user:pass@foo.com", nil)
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -k -X POST 'https://user:xxxxx@foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "https://user:pass@foo.com", nil)
	req.Header.Set("Authorisation", "Bearer 123456")
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -k -X POST -H 'Authorisation: Bearer <secret>' 'https://user:xxxxx@foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "https://user:pass@foo.com", nil)
	req.Header.Set("Authorisation", "Basic 123456")
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -k -X POST -H 'Authorisation: Basic <secret>' 'https://user:xxxxx@foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "https://foo.com", nil)
	req.Header.Set("My-Password", "mypassword")
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -k -X POST -H 'My-Password: <secret>' 'https://foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "https://foo.com", nil)
	req.Header.Set("key-for", "my-new-key")
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -k -X POST -H 'Key-For: <secret>' 'https://foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "https://foo.com", nil)
	req.Header.Set("My-Secret-Org", "secret-organisation")
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -k -X POST -H 'My-Secret-Org: <secret>' 'https://foo.com?query=up&step=10'")

	req, _ = http.NewRequest(http.MethodPost, "https://foo.com", nil)
	req.Header.Set("Token", "secret-token")
	params = req.URL.Query()
	params.Add("query", "up")
	params.Add("step", "10")
	req.URL.RawQuery = params.Encode()
	f(req, "curl -k -X POST -H 'Token: <secret>' 'https://foo.com?query=up&step=10'")
}
