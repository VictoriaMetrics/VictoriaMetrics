package flagutil

import "strings"

// Array holds an array of flag values
type Array []string

// String implements flag.Value interface
func (a *Array) String() string {
	return strings.Join(*a, ",")
}

// Set implements flag.Value interface
func (a *Array) Set(value string) error {
	values := strings.Split(value, ",")
	*a = append(*a, values...)
	return nil
}
