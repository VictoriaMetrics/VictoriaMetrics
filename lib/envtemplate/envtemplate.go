package envtemplate

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/valyala/fasttemplate"
)

// ReplaceBytes replaces `%{ENV_VAR}` placeholders in b with the corresponding ENV_VAR values.
//
// Error is returned if ENV_VAR isn't set for some `%{ENV_VAR}` placeholder.
func ReplaceBytes(b []byte) ([]byte, error) {
	result, err := expand(envVars, string(b))
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

// ReplaceString replaces `%{ENV_VAR}` placeholders in b with the corresponding ENV_VAR values.
//
// Error is returned if ENV_VAR isn't set for some `%{ENV_VAR}` placeholder.
func ReplaceString(s string) (string, error) {
	result, err := expand(envVars, s)
	if err != nil {
		return "", err
	}
	return result, nil
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
			valueExpanded, err := expand(m, value)
			if err != nil {
				// Do not use lib/logger here, since it is uninitialized yet.
				log.Fatalf("cannot expand %q env var value %q: %s", name, value, err)
			}
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

func expand(m map[string]string, s string) (string, error) {
	if !strings.Contains(s, "%{") {
		// Fast path - nothing to expand
		return s, nil
	}
	result, err := fasttemplate.ExecuteFuncStringWithErr(s, "%{", "}", func(w io.Writer, tag string) (int, error) {
		if !isValidEnvVarName(tag) {
			return fmt.Fprintf(w, "%%{%s}", tag)
		}
		v, ok := m[tag]
		if !ok {
			return 0, fmt.Errorf("missing %q env var", tag)
		}
		return fmt.Fprintf(w, "%s", v)
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

func isValidEnvVarName(s string) bool {
	return envVarNameRegex.MatchString(s)
}

// envVarNameRegex is used for validating environment variable names.
//
// Allow dashes and dots in env var names - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3999
var envVarNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_\-.]*$`)
