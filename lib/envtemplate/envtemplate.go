package envtemplate

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/valyala/fasttemplate"
)

// Replace replaces `%{ENV_VAR}` placeholders in b with the corresponding ENV_VAR values.
//
// Error is returned if ENV_VAR isn't set for some `%{ENV_VAR}` placeholder.
func Replace(b []byte) ([]byte, error) {
	if !bytes.Contains(b, []byte("%{")) {
		// Fast path - nothing to replace.
		return b, nil
	}
	s, err := fasttemplate.ExecuteFuncStringWithErr(string(b), "%{", "}", func(w io.Writer, tag string) (int, error) {
		v, ok := os.LookupEnv(tag)
		if !ok {
			return 0, fmt.Errorf("missing %q environment variable", tag)
		}
		return w.Write([]byte(v))
	})
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}
