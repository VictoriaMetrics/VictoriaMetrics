package flagutil

import (
	"strings"
)

// RegisterSecretFlag registers flagName as secret.
//
// This function must be called before starting logging.
// It cannot be called from concurrent goroutines.
//
// Secret flags aren't exported at `/metrics` page.
func RegisterSecretFlag(flagName string) {
	lname := strings.ToLower(flagName)
	secretFlags[lname] = true
}

var secretFlags = make(map[string]bool)

// secretFlagsList contains names of flags with secret values obtained from
// the `-secret.flags` command-line option.
var secretFlagsList = NewArrayString("secret.flags",
	"Comma-separated list of flag names with secret values. Values for these flags are hidden in logs and on /metrics page")

func init() {
	for _, f := range *secretFlagsList {
		RegisterSecretFlag(f)
	}
}

// ApplySecretFlags registers flags from `-secret.flags` after they are parsed.
//
// This function must be called after flag.Parse and before starting logging.
func ApplySecretFlags() {
	for _, f := range *secretFlagsList {
		RegisterSecretFlag(f)
	}
}

// IsSecretFlag returns true of s contains flag name with secret value, which shouldn't be exposed.
func IsSecretFlag(s string) bool {
	if strings.Contains(s, "pass") || strings.Contains(s, "key") || strings.Contains(s, "secret") || strings.Contains(s, "token") {
		return true
	}
	return secretFlags[s]
}
