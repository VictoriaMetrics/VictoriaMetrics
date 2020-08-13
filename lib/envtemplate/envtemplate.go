package envtemplate

import (
	"bytes"
	"io"
	"os"

	"github.com/valyala/fasttemplate"
)

// Replace replaces `%{ENV_VAR}` placeholders in b with the corresponding ENV_VAR values.
func Replace(b []byte) []byte {
	if !bytes.Contains(b, []byte("%{")) {
		// Fast path - nothing to replace.
		return b
	}
	s := fasttemplate.ExecuteFuncString(string(b), "%{", "}", func(w io.Writer, tag string) (int, error) {
		v := os.Getenv(tag)
		if v == "" {
			v = "%{" + tag + "}"
		}
		return w.Write([]byte(v))
	})
	return []byte(s)
}
