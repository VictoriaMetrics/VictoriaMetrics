package flagutil

import (
	"flag"
	"os"
	"testing"
)

var fooFlag Array

func init() {
	os.Args = append(os.Args, "--fooFlag=foo", "--fooFlag=bar")
	flag.Var(&fooFlag, "fooFlag", "test")
}

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestArray(t *testing.T) {
	expected := map[string]struct{}{
		"foo": {},
		"bar": {},
	}
	if len(expected) != len(fooFlag) {
		t.Errorf("len array flag (%d) is not equal to %d", len(fooFlag), len(expected))
	}
	for _, i := range fooFlag {
		if _, ok := expected[i]; !ok {
			t.Errorf("unexpected item in array %v", i)
		}
	}
}
