package logsql

import (
	"bytes"
	"strings"
	"testing"
)

func TestSortWriter(t *testing.T) {
	f := func(maxBufLen, maxLines int, data string, expectedResult string) {
		t.Helper()

		var bb bytes.Buffer
		sw := getSortWriter()
		sw.Init(&bb, maxBufLen, maxLines)
		for _, s := range strings.Split(data, "\n") {
			if !sw.TryWrite([]byte(s + "\n")) {
				break
			}
		}
		sw.FinalFlush()
		putSortWriter(sw)

		result := bb.String()
		if result != expectedResult {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, expectedResult)
		}
	}

	f(100, 0, "", "")
	f(100, 0, "{}", "{}\n")

	data := `{"_time":"def","_msg":"xxx"}
{"_time":"abc","_msg":"foo"}`
	resultExpected := `{"_time":"abc","_msg":"foo"}
{"_time":"def","_msg":"xxx"}
`
	f(100, 0, data, resultExpected)
	f(10, 0, data, data+"\n")

	// Test with the maxLines
	f(100, 1, data, `{"_time":"abc","_msg":"foo"}`+"\n")
	f(10, 1, data, `{"_time":"def","_msg":"xxx"}`+"\n")
	f(10, 2, data, data+"\n")
	f(100, 2, data, resultExpected)
}
