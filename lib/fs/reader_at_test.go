package fs

import (
	"fmt"
	"os"
	"testing"
)

func TestReaderAt(t *testing.T) {
	for _, bufSize := range []int{1, 1e1, 1e2, 1e3, 1e4, 1e5} {
		t.Run(fmt.Sprintf("%d", bufSize), func(t *testing.T) {
			testReaderAt(t, bufSize)
		})
	}
}

func testReaderAt(t *testing.T, bufSize int) {
	path := "TestReaderAt"
	const fileSize = 8 * 1024 * 1024
	data := make([]byte, fileSize)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("cannot create %q: %s", path, err)
	}
	defer MustRemoveAll(path)
	r := MustOpenReaderAt(path)
	defer r.MustClose()

	buf := make([]byte, bufSize)
	for i := 0; i < fileSize-bufSize; i += bufSize {
		offset := int64(i)
		r.MustReadAt(buf[:0], offset)
		r.MustReadAt(buf, offset)
	}
}
