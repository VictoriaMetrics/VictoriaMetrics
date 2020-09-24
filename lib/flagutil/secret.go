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

// IsSecretFlag returns true of s contains flag name with secret value, which shouldn't be exposed.
func IsSecretFlag(s string) bool {
	if strings.Contains(s, "pass") || strings.Contains(s, "key") || strings.Contains(s, "secret") || strings.Contains(s, "token") {
		return true
	}
	return secretFlags[s]
}
