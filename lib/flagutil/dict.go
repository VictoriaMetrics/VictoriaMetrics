package flagutil

import (
	"encoding/json"
	"flag"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type dictValue[V any, T any] interface {
	Get() T
	flag.Value
	*V
}

type intDictValue int

// Set sets int flag value from string
func (v *intDictValue) Set(i string) error {
	value, err := strconv.Atoi(i)
	if err != nil {
		return err
	}
	*v = intDictValue(value)
	return nil
}

// Get int flag value
func (v *intDictValue) Get() int {
	return int(*v)
}

// String returns string value
func (v *intDictValue) String() string {
	return strconv.Itoa(int(*v))
}

type boolDictValue bool

// Set sets bool flag value from string
func (v *boolDictValue) Set(i string) error {
	value, err := strconv.ParseBool(i)
	if err != nil {
		return err
	}
	*v = boolDictValue(value)
	return nil
}

// Get bool flag value
func (v *boolDictValue) Get() bool {
	return bool(*v)
}

// String returns string value
func (v *boolDictValue) String() string {
	return strconv.FormatBool(bool(*v))
}

type strDictValue string

// Set sets string flag value from string
func (v *strDictValue) Set(i string) error {
	*v = strDictValue(i)
	return nil
}

// Get string flag value
func (v *strDictValue) Get() string {
	return string(*v)
}

// String returns string value
func (v *strDictValue) String() string {
	return string(*v)
}

type durationDictValue time.Duration

// Set sets time.Duration flag value from string
func (v *durationDictValue) Set(i string) error {
	value, err := time.ParseDuration(i)
	if err != nil {
		return err
	}
	*v = durationDictValue(value)
	return nil
}

// Get time.Duration flag value
func (v *durationDictValue) Get() time.Duration {
	return time.Duration(*v)
}

// String returns string value
func (v *durationDictValue) String() string {
	return time.Duration(*v).String()
}

// DictValue allows specifying a dictionary of named values in the form `name1<separator>value1,...,nameN<separator>valueN`.
type DictValue[V any, T any, P dictValue[V, T]] struct {
	defaultValue T
	delimiter    byte
	kvs          []kv[V, T, P]
	ignoreFn     func(string) bool
}

type kv[V any, T any, P dictValue[V, T]] struct {
	k  string
	vs []P
}

// NewDictInt creates DictValue of type int with the given name, defaultValue and description.
func NewDictInt(name string, defaultValue int, delimiter byte, description string) *DictValue[intDictValue, int, *intDictValue] {
	return newDictValue[intDictValue](name, defaultValue, delimiter, description)
}

// NewDictBool creates DictValue of type bool with the given name, defaultValue and description.
func NewDictBool(name string, defaultValue bool, delimiter byte, description string) *DictValue[boolDictValue, bool, *boolDictValue] {
	return newDictValue[boolDictValue](name, defaultValue, delimiter, description)
}

// NewDictDuration creates DictValue of type time.Duration with the given name, defaultValue and description.
func NewDictDuration(name string, defaultValue time.Duration, delimiter byte, description string) *DictValue[durationDictValue, time.Duration, *durationDictValue] {
	return newDictValue[durationDictValue](name, defaultValue, delimiter, description)
}

// NewDictString creates DictValue of type string with the given name, defaultValue and description.
func NewDictString(name string, defaultValue string, delimiter byte, description string, ignoreFn ...func(string) bool) *DictValue[strDictValue, string, *strDictValue] {
	return newDictValue[strDictValue](name, defaultValue, delimiter, description, ignoreFn...)
}

// NewDictBytes creates DictValue of bytes with the given name, defaultValue and description.
func NewDictBytes(name string, defaultValue int64, delimiter byte, description string) *DictValue[Bytes, int64, *Bytes] {
	return newDictValue[Bytes](name, defaultValue, delimiter, description)
}

// newDictValue creates DictValue with the given name, defaultValue and description.
func newDictValue[V any, T any, P dictValue[V, T]](name string, defaultValue T, delimiter byte, description string, ignoreFn ...func(string) bool) *DictValue[V, T, P] {
	df := &DictValue[V, T, P]{
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
func (df *DictValue[V, T, P]) String() string {
	kvs := df.kvs
	if len(kvs) == 1 && kvs[0].k == "" && len(kvs[0].vs) == 1 {
		// Short form - a single value
		return fmt.Sprintf("%v", kvs[0].vs[0])
	}

	formattedResults := []string{}
	for _, kv := range kvs {
		for _, v := range kv.vs {
			if kv.k == "" {
				formattedResults = append(formattedResults, v.String())
			} else {
				formattedResults = append(formattedResults, fmt.Sprintf("%s%s%s", kv.k, string(df.delimiter), v.String()))
			}
		}
	}
	return strings.Join(formattedResults, ",")
}

// Set implements flag.Value interface
func (df *DictValue[V, T, P]) Set(value string) error {
	values := parseArrayValues(value)
	var v P
	for _, x := range values {
		k := x[:0]
		vRaw := x
		n := strings.IndexByte(x, df.delimiter)
		if n >= 0 && !df.ignoreFn(x[:n]) {
			k = x[:n]
			vRaw = x[n+1:]
		}

		v = new(V)
		err := v.Set(vRaw)
		if err != nil {
			return fmt.Errorf("cannot parse value for key=%q: %w", k, err)
		}
		idx := slices.IndexFunc(df.kvs, func(kv kv[V, T, P]) bool {
			return kv.k == k
		})
		if idx == -1 {
			df.kvs = append(df.kvs, kv[V, T, P]{
				k:  k,
				vs: []P{v},
			})
		} else {
			df.kvs[idx].vs = append(df.kvs[idx].vs, v)
		}
	}
	return nil
}

// GetAll returns all values for a given key.
func (df *DictValue[V, T, P]) GetAll(key string) []T {
	var values []T
	for _, kv := range df.kvs {
		if kv.k == key {
			for _, v := range kv.vs {
				values = append(values, v.Get())
			}
			break
		}
	}
	return values
}

// Get returns value for the given key and index idx.
func (df *DictValue[V, T, P]) Get(key string, idx int) (T, error) {
	var values []T
	var value T
	for _, kv := range df.kvs {
		if kv.k == key {
			for _, v := range kv.vs {
				values = append(values, v.Get())
			}
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
func (df *DictValue[V, T, P]) FormatValue(key string, value string) string {
	var prefix string
	if key != "" {
		prefix = fmt.Sprintf("%s%s", key, string(df.delimiter))
	}
	return fmt.Sprintf("%s%s", prefix, value)
}

// First returns value for the given key.
//
// Default value is returned if key isn't found in df.
func (df *DictValue[V, T, P]) First(key string) T {
	for _, kv := range df.kvs {
		if kv.k == key {
			return kv.vs[0].Get()
		}
	}
	return df.defaultValue
}

// Keys returns unique dict keys
func (df *DictValue[V, T, P]) Keys() []string {
	output := make([]string, len(df.kvs))
	for i, kv := range df.kvs {
		output[i] = kv.k
	}
	return output
}

// AllValuesFlat returns unique dict keys
func (df *DictValue[V, T, P]) AllValuesFlat() []T {
	var output []T
	for _, kv := range df.kvs {
		for _, v := range kv.vs {
			output = append(output, v.Get())
		}
	}
	return output
}

// GetOptionalArg returns optional arg under the given key and values argIdx.
func (df *DictValue[V, T, P]) GetOptionalArg(key string, argIdx int) T {
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
