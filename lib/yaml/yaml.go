package yaml

import (
	"bytes"
	"gopkg.in/yaml.v3"
	"io"
)

// Unmarshal decodes v from yaml formatted data byte slice
func Unmarshal(data []byte, v interface{}, isStrict bool) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(isStrict)
	if err := dec.Decode(v); err != nil {
		if err != io.EOF {
			return err
		}
	}
	return nil
}

// Marshal encodes v into yaml formatted byte slice
func Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
