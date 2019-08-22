package fastjson

import (
	"strconv"
	"strings"
)

// Del deletes the entry with the given key from o.
func (o *Object) Del(key string) {
	if o == nil {
		return
	}
	if !o.keysUnescaped && strings.IndexByte(key, '\\') < 0 {
		// Fast path - try searching for the key without object keys unescaping.
		for i, kv := range o.kvs {
			if kv.k == key {
				o.kvs = append(o.kvs[:i], o.kvs[i+1:]...)
				return
			}
		}
	}

	// Slow path - unescape object keys before item search.
	o.unescapeKeys()

	for i, kv := range o.kvs {
		if kv.k == key {
			o.kvs = append(o.kvs[:i], o.kvs[i+1:]...)
			return
		}
	}
}

// Del deletes the entry with the given key from array or object v.
func (v *Value) Del(key string) {
	if v == nil {
		return
	}
	if v.t == TypeObject {
		v.o.Del(key)
		return
	}
	if v.t == TypeArray {
		n, err := strconv.Atoi(key)
		if err != nil || n < 0 || n >= len(v.a) {
			return
		}
		v.a = append(v.a[:n], v.a[n+1:]...)
	}
}

// Set sets (key, value) entry in the o.
//
// The value must be unchanged during o lifetime.
func (o *Object) Set(key string, value *Value) {
	if o == nil {
		return
	}
	if value == nil {
		value = valueNull
	}
	o.unescapeKeys()

	// Try substituting already existing entry with the given key.
	for i := range o.kvs {
		kv := &o.kvs[i]
		if kv.k == key {
			kv.v = value
			return
		}
	}

	// Add new entry.
	kv := o.getKV()
	kv.k = key
	kv.v = value
}

// Set sets (key, value) entry in the array or object v.
//
// The value must be unchanged during v lifetime.
func (v *Value) Set(key string, value *Value) {
	if v == nil {
		return
	}
	if v.t == TypeObject {
		v.o.Set(key, value)
		return
	}
	if v.t == TypeArray {
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 {
			return
		}
		v.SetArrayItem(idx, value)
	}
}

// SetArrayItem sets the value in the array v at idx position.
//
// The value must be unchanged during v lifetime.
func (v *Value) SetArrayItem(idx int, value *Value) {
	if v == nil || v.t != TypeArray {
		return
	}
	for idx >= len(v.a) {
		v.a = append(v.a, valueNull)
	}
	v.a[idx] = value
}
