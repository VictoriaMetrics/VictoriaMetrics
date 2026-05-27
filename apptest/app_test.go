package apptest

import (
	"reflect"
	"testing"
)

func TestSetDefaultFlags(t *testing.T) {
	t.Run("does-not-mutate-input-map", func(t *testing.T) {
		original := map[string]string{
			"-foo": "1",
			"-bar": "2",
		}
		want := map[string]string{
			"-foo": "1",
			"-bar": "2",
		}

		flags := setDefaultFlags([]string{"-foo=3"}, original)

		if !reflect.DeepEqual(original, want) {
			t.Fatalf("setDefaultFlags mutated the input map: got %v, want %v", original, want)
		}

		wantFlags := []string{"-foo=3", "-bar=2"}
		if !reflect.DeepEqual(flags, wantFlags) {
			t.Fatalf("unexpected flags: got %v, want %v", flags, wantFlags)
		}
	})

	t.Run("nil-default-flags", func(t *testing.T) {
		flags := setDefaultFlags([]string{"-foo=1"}, nil)
		want := []string{"-foo=1"}
		if !reflect.DeepEqual(flags, want) {
			t.Fatalf("unexpected flags: got %v, want %v", flags, want)
		}
	})

	t.Run("all-defaults-already-present", func(t *testing.T) {
		original := map[string]string{
			"-foo": "1",
		}
		flags := setDefaultFlags([]string{"-foo=1"}, original)
		want := []string{"-foo=1"}
		if !reflect.DeepEqual(flags, want) {
			t.Fatalf("unexpected flags: got %v, want %v", flags, want)
		}
	})
}
