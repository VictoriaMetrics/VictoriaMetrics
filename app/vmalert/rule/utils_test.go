package rule

import (
	"net/http"
	"testing"
)

func TestRequestToCurl(t *testing.T) {
	f := func(req *http.Request, exp string) {
		t.Helper()
		got := requestToCurl(req)
		if got != exp {
			t.Fatalf("expected to have %q; got %q instead", exp, got)
		}
	}

	newReq := func(url string, queryParams ...string) *http.Request {
		t.Helper()
		r, err := http.NewRequest(http.MethodPost, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		params := r.URL.Query()
		for i := 0; i < len(queryParams); i += 2 {
			params.Add(queryParams[i], queryParams[i+1])
		}
		r.URL.RawQuery = params.Encode()

		return r
	}

	req := newReq("foo.com")
	f(req, "curl -X POST 'http://foo.com'")

	req = newReq("foo.com")
	req.Header.Set("foo", "bar")
	req.Header.Set("baz", "qux")
	f(req, "curl -X POST -H 'Baz: qux' -H 'Foo: bar' 'http://foo.com'")

	req = newReq("foo.com", "query", "up", "step", "10")
	f(req, "curl -X POST 'http://foo.com?query=up&step=10'")

	req = newReq("http://foo.com", "query", "up", "step", "10")
	f(req, "curl -X POST 'http://foo.com?query=up&step=10'")

	req = newReq("https://foo.com", "query", "up", "step", "10")
	f(req, "curl -k -X POST 'https://foo.com?query=up&step=10'")

	req = newReq("https://user:pass@foo.com", "query", "up", "step", "10")
	f(req, "curl -k -X POST 'https://user:xxxxx@foo.com?query=up&step=10'")

	req = newReq("https://user:pass@foo.com")
	req.Header.Set("Authorization", "Bearer 123456")
	f(req, "curl -k -X POST -H 'Authorization: <secret>' 'https://user:xxxxx@foo.com'")

	req = newReq("https://user:pass@foo.com")
	req.Header.Set("Authorization", "Basic 123456")
	f(req, "curl -k -X POST -H 'Authorization: <secret>' 'https://user:xxxxx@foo.com'")

	req = newReq("https://foo.com")
	req.Header.Set("My-Password", "mypassword")
	f(req, "curl -k -X POST -H 'My-Password: <secret>' 'https://foo.com'")

	req = newReq("https://foo.com")
	req.Header.Set("key-for", "my-new-key")
	f(req, "curl -k -X POST -H 'Key-For: <secret>' 'https://foo.com'")

	req = newReq("https://foo.com")
	req.Header.Set("My-Secret-Org", "secret-organization")
	f(req, "curl -k -X POST -H 'My-Secret-Org: <secret>' 'https://foo.com'")

	req = newReq("https://foo.com")
	req.Header.Set("Token", "secret-token")
	f(req, "curl -k -X POST -H 'Token: <secret>' 'https://foo.com'")
}
