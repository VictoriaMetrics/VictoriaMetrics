package envutil

import (
	"os"
	"strconv"
)

// GetenvBool retrieves the value of the environment variable named by the key,
// attempts to convert the value to bool type and returns the result. In order
// for conversion to succeed, the value must be any value supported by
// strconv.ParseBool() function, otherwise the function will return false.
func GetenvBool(key string) bool {
	s := os.Getenv(key)
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}
