package flagutil

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// DictInt allows specifying a dictionary of named ints in the form `name1:value1,...,nameN:valueN`.
type DictInt struct {
	defaultValue int
	kvs          []kIntValue
}

type kIntValue struct {
	k string
	v int
}

// NewDictInt creates DictInt with the given name, defaultValue and description.
func NewDictInt(name string, defaultValue int, description string) *DictInt {
	description += fmt.Sprintf(" (default %d)", defaultValue)
	description += "\nSupports an `array` of `key:value` entries separated by comma or specified via multiple flags."
	di := &DictInt{
		defaultValue: defaultValue,
	}
	flag.Var(di, name, description)
	return di
}

// String implements flag.Value interface
func (di *DictInt) String() string {
	kvs := di.kvs
	if len(kvs) == 1 && kvs[0].k == "" {
		// Short form - a single int value
		return strconv.Itoa(kvs[0].v)
	}

	formattedResults := make([]string, len(kvs))
	for i, kv := range kvs {
		formattedResults[i] = fmt.Sprintf("%s:%d", kv.k, kv.v)
	}
	return strings.Join(formattedResults, ",")
}

// Set implements flag.Value interface
func (di *DictInt) Set(value string) error {
	values := parseArrayValues(value)
	if len(di.kvs) == 0 && len(values) == 1 && strings.IndexByte(values[0], ':') < 0 {
		v, err := strconv.Atoi(values[0])
		if err != nil {
			return err
		}
		di.kvs = append(di.kvs, kIntValue{
			v: v,
		})
		di.defaultValue = v
		return nil
	}
	for _, x := range values {
		n := strings.IndexByte(x, ':')
		if n < 0 {
			return fmt.Errorf("missing ':' in %q", x)
		}
		k := x[:n]
		v, err := strconv.Atoi(x[n+1:])
		if err != nil {
			return fmt.Errorf("cannot parse value for key=%q: %w", k, err)
		}
		if di.contains(k) {
			return fmt.Errorf("duplicate value for key=%q: %d", k, v)
		}
		di.kvs = append(di.kvs, kIntValue{
			k: k,
			v: v,
		})
	}
	return nil
}

func (di *DictInt) contains(key string) bool {
	for _, kv := range di.kvs {
		if kv.k == key {
			return true
		}
	}
	return false
}

// Get returns value for the given key.
//
// Default value is returned if key isn't found in di.
func (di *DictInt) Get(key string) int {
	for _, kv := range di.kvs {
		if kv.k == key {
			return kv.v
		}
	}
	return di.defaultValue
}

// ParseJSONMap parses s, which must contain JSON map of {"k1":"v1",...,"kN":"vN"}
func ParseJSONMap(s string) (map[string]string, error) {
	if s == "" {
		// Special case
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}
