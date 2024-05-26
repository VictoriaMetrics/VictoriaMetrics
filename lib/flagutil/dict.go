package flagutil

import (
	"encoding/json"
	"flag"
	"fmt"
	"slices"
	"strings"
)

// DictValue allows specifying a dictionary of named values in the form `name1:value1,...,nameN:valueN`.
type DictValue[V any] struct {
	defaultValue V
	delimiter    byte
	kvs          []kv[V]
	ignoreFn     func(string) bool
}

type kv[V any] struct {
	k  string
	vs []V
}

func parseDictValue[V any](value string) (V, error) {
	var result V
	_, err := fmt.Sscanf(value, "%v", &result)
	return result, err
}

// NewDictValue creates DictValue with the given name, defaultValue and description.
func NewDictValue[V any](name string, defaultValue V, delimiter byte, description string, ignoreFn ...func(string) bool) *DictValue[V] {
	df := &DictValue[V]{
		defaultValue: defaultValue,
		delimiter:    delimiter,
	}
	if len(ignoreFn) > 0 {
		df.ignoreFn = ignoreFn[0]
	} else {
		df.ignoreFn = func(_ string) bool { return false }
	}
	description += fmt.Sprintf(" (default %v)", defaultValue)
	description += "\nSupports an `array` of `key:value` entries separated by comma or specified via multiple flags."
	flag.Var(df, name, description)
	return df
}

// String implements flag.Value interface
func (df *DictValue[V]) String() string {
	kvs := df.kvs
	if len(kvs) == 1 && kvs[0].k == "" && len(kvs[0].vs) == 1 {
		// Short form - a single value
		return fmt.Sprintf("%v", kvs[0].vs[0])
	}

	formattedResults := []string{}
	for _, kv := range kvs {
		for _, v := range kv.vs {
			if kv.k == "" {
				formattedResults = append(formattedResults, fmt.Sprintf("%v", v))
			} else {
				formattedResults = append(formattedResults, fmt.Sprintf("%s%s%v", kv.k, string(df.delimiter), v))
			}
		}
	}
	return strings.Join(formattedResults, ",")
}

// Set implements flag.Value interface
func (df *DictValue[V]) Set(value string) error {
	values := parseArrayValues(value)
	for _, x := range values {
		k := x[:0]
		vRaw := x
		n := strings.IndexByte(x, df.delimiter)
		if n >= 0 && !df.ignoreFn(x[:n]) {
			k = x[:n]
			vRaw = x[n+1:]
		}

		v, err := parseDictValue[V](vRaw)
		if err != nil {
			return fmt.Errorf("cannot parse value for key=%q: %w", k, err)
		}
		idx := slices.IndexFunc(df.kvs, func(kv kv[V]) bool {
			return kv.k == k
		})
		if idx == -1 {
			df.kvs = append(df.kvs, kv[V]{
				k:  k,
				vs: []V{v},
			})
		} else {
			df.kvs[idx].vs = append(df.kvs[idx].vs, v)
		}
	}
	return nil
}

// GetAll returns all values for a given key.
func (df *DictValue[V]) GetAll(key string) []V {
	var values []V
	for _, kv := range df.kvs {
		if kv.k == key {
			values = kv.vs
			break
		}
	}
	return values
}

// Get returns value for the given key and index idx.
func (df *DictValue[V]) Get(key string, idx int) (V, error) {
	var values []V
	var value V
	for _, kv := range df.kvs {
		if kv.k == key {
			values = kv.vs
			break
		}
	}
	if key == "" {
		if len(values) == 0 {
			return value, fmt.Errorf("has no keyless values")
		} else if len(values) <= idx {
			return value, fmt.Errorf("has no keyless value at index %d", idx)
		}
	} else {
		if len(values) == 0 {
			return value, fmt.Errorf("has no values under key %q", key)
		} else if len(values) <= idx {
			return value, fmt.Errorf("has no value udner key %q at index %d", key, idx)
		}
	}
	return values[idx], nil
}

// FormatValue formats input value according to rules of a flag
func (df *DictValue[V]) FormatValue(key string, value string) string {
	var prefix string
	if key != "" {
		prefix = fmt.Sprintf("%s%s", key, string(df.delimiter))
	}
	return fmt.Sprintf("%s%s", prefix, value)
}

// First returns value for the given key.
//
// Default value is returned if key isn't found in df.
func (df *DictValue[V]) First(key string) V {
	for _, kv := range df.kvs {
		if kv.k == key {
			return kv.vs[0]
		}
	}
	return df.defaultValue
}

// Keys returns unique dict keys
func (df *DictValue[V]) Keys() []string {
	output := make([]string, len(df.kvs))
	for i, kv := range df.kvs {
		output[i] = kv.k
	}
	return output
}

// AllValuesFlat returns unique dict keys
func (df *DictValue[V]) AllValuesFlat() []V {
	var output []V
	for _, kv := range df.kvs {
		output = append(output, kv.vs...)
	}
	return output
}

// GetOptionalArg returns optional arg under the given key and values argIdx.
func (df *DictValue[V]) GetOptionalArg(key string, argIdx int) V {
	value, err := df.Get(key, argIdx)
	if err != nil {
		return df.defaultValue
	}
	return value
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
