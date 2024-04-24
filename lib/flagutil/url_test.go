package flagutil

import (
	"os"
	"testing"
)

func TestNewURL(t *testing.T) {
	u := &URL{}
	f := func(s, exp string) {
		t.Helper()
		if err := u.Set(s); err != nil {
			t.Fatalf("failed to set %q value: %s", s, err)
		}
		if u.String() != exp {
			t.Fatalf("expected to get %q; got %q instead", exp, u.String())
		}
	}

	f("", "")
	f("http://foo:8428", "http://foo:8428")
	f("http://username:password@foo:8428", "http://xxxxx:xxxxx@foo:8428")
	f("http://foo:8428?authToken=bar", "http://foo:8428?authToken=xxxxx")
	f("http://username:password@foo:8428?authToken=bar", "http://xxxxx:xxxxx@foo:8428?authToken=xxxxx")

	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(file.Name()) }()

	writeToFile(t, file.Name(), "http://foo:8428")
	f("file://"+file.Name(), "http://foo:8428")

	writeToFile(t, file.Name(), "http://xxxxx:password@foo:8428?authToken=bar")
	f("file://"+file.Name(), "http://xxxxx:xxxxx@foo:8428?authToken=xxxxx")
}

func writeToFile(t *testing.T, file, b string) {
	t.Helper()
	err := os.WriteFile(file, []byte(b), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
