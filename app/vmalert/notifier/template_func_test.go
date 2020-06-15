package notifier

import (
	"net/url"
	"testing"
)

func TestAlertSourceFunc(t *testing.T) {
	base, _ := url.Parse("https://victoriametrics.com/path")
	fn, err := AlertSourceFunc(base, "", false)
	if fn != nil || err != nil {
		t.Errorf("expected nil functionc AND nil error got %+v, %s", fn, err)
	}
	_, err = AlertSourceFunc(base, `explore?{{$expr|foo}}\"}}`, true)
	if err == nil {
		t.Errorf("expected template validation error")
	}
	fn, err = AlertSourceFunc(base, `path?value={{$value}}`, true)
	if err != nil {
		t.Errorf("unxeptected error %s", err)
	}
	u := fn(Alert{
		Value: 42,
	})
	if exp := "https://victoriametrics.com/path/path?value=42"; u != exp {
		t.Errorf("unexpected URL want %s, got %s", exp, u)
	}
}
