package discoveryutils

import (
	"fmt"
	"testing"
	"time"
)

func TestSanitizeLabelNameSerial(t *testing.T) {
	if err := testSanitizeLabelName(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestSanitizeLabelNameParallel(t *testing.T) {
	goroutines := 5
	ch := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			ch <- testSanitizeLabelName()
		}()
	}
	tch := time.After(5 * time.Second)
	for i := 0; i < goroutines; i++ {
		select {
		case <-tch:
			t.Fatalf("timeout!")
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		}
	}
}

func testSanitizeLabelName() error {
	f := func(name, expectedSanitizedName string) error {
		for i := 0; i < 5; i++ {
			sanitizedName := SanitizeLabelName(name)
			if sanitizedName != expectedSanitizedName {
				return fmt.Errorf("unexpected sanitized label name %q; got %q; want %q", name, sanitizedName, expectedSanitizedName)
			}
		}
		return nil
	}
	if err := f("", ""); err != nil {
		return err
	}
	if err := f("foo", "foo"); err != nil {
		return err
	}
	if err := f("foo-bar/baz", "foo_bar_baz"); err != nil {
		return err
	}
	return nil
}
