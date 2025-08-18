package envtemplate

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/valyala/fasttemplate"
)

// ReplaceBytes replaces `%{ENV_VAR}` placeholders in b with the corresponding ENV_VAR values.
func ReplaceBytes(b []byte) []byte {
	result := expand(envVars, string(b))
	return []byte(result)
}

// ReplaceString replaces `%{ENV_VAR}` placeholders in b with the corresponding ENV_VAR values.
func ReplaceString(s string) string {
	return expand(envVars, s)
}

// LookupEnv returns the expanded environment variable value for the given name.
//
// The expanded means that `%{ENV_VAR}` placeholders in env var value are replaced
// with the corresponding ENV_VAR values (recursively).
//
// false is returned if environment variable isn't found.
func LookupEnv(name string) (string, bool) {
	value, ok := envVars[name]
	return value, ok
}

var envVars = func() map[string]string {
	envs := os.Environ()
	m := parseEnvVars(envs)
	return expandTemplates(m)
}()

func parseEnvVars(envs []string) map[string]string {
	m := make(map[string]string, len(envs))
	for _, env := range envs {
		n := strings.IndexByte(env, '=')
		if n < 0 {
			m[env] = ""
			continue
		}
		name := env[:n]
		value := env[n+1:]
		m[name] = value
	}
	return m
}

func expandTemplates(m map[string]string) map[string]string {
	for i := 0; i < len(m); i++ {
		mExpanded := make(map[string]string, len(m))
		expands := 0
		for name, value := range m {
			valueExpanded := expand(m, value)
			mExpanded[name] = valueExpanded
			if valueExpanded != value {
				expands++
			}
		}
		if expands == 0 {
			return mExpanded
		}
		m = mExpanded
	}
	return m
}

func expand(m map[string]string, s string) string {
	return fasttemplate.ExecuteFuncString(s, "%{", "}", func(w io.Writer, tag string) (int, error) {
		v, ok := m[tag]
		if !ok {
			// Cannot find the tag in m. Leave it as is.
			return fmt.Fprintf(w, "%%{%s}", tag)
		}
		return fmt.Fprintf(w, "%s", v)
	})
}
