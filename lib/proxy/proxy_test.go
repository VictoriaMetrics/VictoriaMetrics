package proxy

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestURLParseSuccess(t *testing.T) {
	f := func(src string) {
		t.Helper()
		var u URL
		if err := yaml.Unmarshal([]byte(src), &u); err != nil {
			t.Fatalf("unexpected error for url: %s: %s", src, err)
		}
	}
	f("http://some-url/path")
	f("https://some-url/path")
	f("socks5://some-url/path")
	f("tls+socks5://some-sock-path")
}

func TestParseFail(t *testing.T) {
	f := func(src string) {
		t.Helper()
		var u URL
		if err := yaml.Unmarshal([]byte(src), &u); err == nil {
			t.Fatalf("want error for url: %s", src)
		}
	}
	f("bad-scheme://my-url")
	f("unix://my-socket.sock")
	f("http://some-url:bad-port")
}
