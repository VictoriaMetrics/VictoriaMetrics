package logsql

import (
	"bytes"
	"strings"
	"testing"
)

func TestSortWriter(t *testing.T) {
	f := func(maxBufLen int, data string, expectedResult string) {
		t.Helper()

		var bb bytes.Buffer
		sw := getSortWriter()
		sw.Init(&bb, maxBufLen)

		for _, s := range strings.Split(data, "\n") {
			sw.MustWrite([]byte(s + "\n"))
		}
		sw.FinalFlush()
		putSortWriter(sw)

		result := bb.String()
		if result != expectedResult {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, expectedResult)
		}
	}

	f(100, "", "")
	f(100, "{}", "{}\n")

	data := `{"_time":"def","_msg":"xxx"}
{"_time":"abc","_msg":"foo"}`
	resultExpected := `{"_time":"abc","_msg":"foo"}
{"_time":"def","_msg":"xxx"}
`
	f(100, data, resultExpected)
	f(10, data, data+"\n")
}
