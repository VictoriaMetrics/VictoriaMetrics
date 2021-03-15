package flagutil

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// NewBytes returns new `bytes` flag with the given name, defaultValue and description.
func NewBytes(name string, defaultValue int, description string) *Bytes {
	description += "\nSupports the following optional suffixes for `size` values: KB, MB, GB, KiB, MiB, GiB"
	b := Bytes{
		N:           defaultValue,
		valueString: fmt.Sprintf("%d", defaultValue),
	}
	flag.Var(&b, name, description)
	return &b
}

// Bytes is a flag for holding size in bytes.
//
// It supports the following optional suffixes for values: KB, MB, GB, KiB, MiB, GiB.
type Bytes struct {
	// N contains parsed value for the given flag.
	N int

	valueString string
}

// String implements flag.Value interface
func (b *Bytes) String() string {
	return b.valueString
}

// Set implements flag.Value interface
func (b *Bytes) Set(value string) error {
	value = normalizeBytesString(value)
	switch {
	case strings.HasSuffix(value, "KB"):
		f, err := strconv.ParseFloat(value[:len(value)-2], 64)
		if err != nil {
			return err
		}
		b.N = int(f * 1000)
		b.valueString = value
		return nil
	case strings.HasSuffix(value, "MB"):
		f, err := strconv.ParseFloat(value[:len(value)-2], 64)
		if err != nil {
			return err
		}
		b.N = int(f * 1000 * 1000)
		b.valueString = value
		return nil
	case strings.HasSuffix(value, "GB"):
		f, err := strconv.ParseFloat(value[:len(value)-2], 64)
		if err != nil {
			return err
		}
		b.N = int(f * 1000 * 1000 * 1000)
		b.valueString = value
		return nil
	case strings.HasSuffix(value, "KiB"):
		f, err := strconv.ParseFloat(value[:len(value)-3], 64)
		if err != nil {
			return err
		}
		b.N = int(f * 1024)
		b.valueString = value
		return nil
	case strings.HasSuffix(value, "MiB"):
		f, err := strconv.ParseFloat(value[:len(value)-3], 64)
		if err != nil {
			return err
		}
		b.N = int(f * 1024 * 1024)
		b.valueString = value
		return nil
	case strings.HasSuffix(value, "GiB"):
		f, err := strconv.ParseFloat(value[:len(value)-3], 64)
		if err != nil {
			return err
		}
		b.N = int(f * 1024 * 1024 * 1024)
		b.valueString = value
		return nil
	default:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		b.N = int(f)
		b.valueString = value
		return nil
	}
}

func normalizeBytesString(s string) string {
	s = strings.ToUpper(s)
	return strings.ReplaceAll(s, "I", "i")
}
