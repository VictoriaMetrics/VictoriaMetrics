package filestream

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestWriteRead(t *testing.T) {
	testWriteRead(t, false, "")
	testWriteRead(t, true, "")
	testWriteRead(t, false, "foobar")
	testWriteRead(t, true, "foobar")
	testWriteRead(t, false, "a\nb\nc\n")
	testWriteRead(t, true, "a\nb\nc\n")

	var bb bytes.Buffer
	for bb.Len() < 3*dontNeedBlockSize {
		fmt.Fprintf(&bb, "line %d\n", bb.Len())
	}
	testStr := bb.String()

	testWriteRead(t, false, testStr)
	testWriteRead(t, true, testStr)
}

func testWriteRead(t *testing.T, nocache bool, testStr string) {
	t.Helper()

	w, err := Create("./nocache_test.txt", nocache)
	if err != nil {
		t.Fatalf("cannot create file: %s", err)
	}
	defer func() {
		_ = os.Remove("./nocache_test.txt")
	}()

	if _, err := fmt.Fprintf(w, "%s", testStr); err != nil {
		t.Fatalf("unexpected error when writing testStr: %s", err)
	}
	w.MustClose()

	r, err := Open("./nocache_test.txt", nocache)
	if err != nil {
		t.Fatalf("cannot open file: %s", err)
	}
	buf := make([]byte, len(testStr))
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("unexpected error when reading: %s", err)
	}
	if string(buf) != testStr {
		t.Fatalf("unexpected data read: got\n%x; want\n%x", buf, testStr)
	}
	r.MustClose()
}
