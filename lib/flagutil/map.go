package flagutil

import (
	"flag"
	"fmt"
	"strings"
)

type MapString map[string]string

// String returns a string representation of the map.
func (m *MapString) String() string {
	if m == nil {
		return ""
	}
	return fmt.Sprintf("%v", *m)
}

// Set parses the given value into a map.
func (m *MapString) Set(value string) error {
	if *m == nil {
		*m = make(map[string]string)
	}
	for _, pair := range parseArrayValues(value) {
		key, value, err := parseMapValue(pair)
		if err != nil {
			return err
		}
		(*m)[key] = value
	}
	return nil
}

func parseMapValue(s string) (string, string, error) {
	kv := strings.SplitN(s, ":", 2)
	if len(kv) != 2 {
		return "", "", fmt.Errorf("invalid map value '%s' values must be 'key:value'", s)
	}

	return kv[0], kv[1], nil
}

// NewMapString returns a new MapString with the given name and description.
func NewMapString(name, description string) *MapString {
	description += fmt.Sprintf("\nSupports multiple flags with the following syntax: -%s=key:value", name)
	var m MapString
	flag.Var(&m, name, description)
	return &m
}
