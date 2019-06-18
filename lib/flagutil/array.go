package flagutil

import (
	"flag"
	"strings"
)

// NewArray returns new Array with the given name and descprition.
func NewArray(name, description string) *Array {
	var a Array
	flag.Var(&a, name, description)
	return &a
}

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
