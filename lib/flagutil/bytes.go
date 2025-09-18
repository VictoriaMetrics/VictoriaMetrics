package flagutil

import (
	"flag"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// NewBytes returns new `bytes` flag with the given name, defaultValue and description.
func NewBytes(name string, defaultValue int64, description string) *Bytes {
	description += "\nSupports the following optional suffixes for `size` values: KB, MB, GB, TB, KiB, MiB, GiB, TiB"
	b := Bytes{
		N:           defaultValue,
		Name:        name,
		valueString: fmt.Sprintf("%d", defaultValue),
	}
	flag.Var(&b, name, description)
	return &b
}

// Bytes is a flag for holding size in bytes.
//
// It supports the following optional suffixes for values: KB, MB, GB, TB, KiB, MiB, GiB, TiB.
type Bytes struct {
	// N contains parsed value for the given flag.
	N int64

	// Name contains flag name.
	Name string

	valueString string
}

// IntN returns the stored value capped by int type.
func (b *Bytes) IntN() int {
	if b.N > math.MaxInt {
		return math.MaxInt
	}
	if b.N < math.MinInt {
		return math.MinInt
	}
	return int(b.N)
}

// String implements flag.Value interface
func (b *Bytes) String() string {
	return b.valueString
}

// Set implements flag.Value interface
func (b *Bytes) Set(value string) error {
	if value == "" {
		b.N = 0
		b.valueString = ""
		return nil
	}
	value = normalizeBytesString(value)
	n, err := parseBytes(value)
	if err != nil {
		return err
	}
	b.N = n
	b.valueString = value
	return nil
}

// ParseBytes returns int64 in bytes of parsed string with unit suffix
func ParseBytes(value string) (int64, error) {
	value = normalizeBytesString(value)
	return parseBytes(value)
}

func parseBytes(value string) (int64, error) {
	switch {
	case strings.HasSuffix(value, "KB"):
		f, err := strconv.ParseFloat(value[:len(value)-2], 64)
		if err != nil {
			return 0, err
		}
		return int64(f * 1000), nil
	case strings.HasSuffix(value, "MB"):
		f, err := strconv.ParseFloat(value[:len(value)-2], 64)
		if err != nil {
			return 0, err
		}
		return int64(f * 1000 * 1000), nil
	case strings.HasSuffix(value, "GB"):
		f, err := strconv.ParseFloat(value[:len(value)-2], 64)
		if err != nil {
			return 0, err
		}
		return int64(f * 1000 * 1000 * 1000), nil
	case strings.HasSuffix(value, "TB"):
		f, err := strconv.ParseFloat(value[:len(value)-2], 64)
		if err != nil {
			return 0, err
		}
		return int64(f * 1000 * 1000 * 1000 * 1000), nil
	case strings.HasSuffix(value, "KiB"):
		f, err := strconv.ParseFloat(value[:len(value)-3], 64)
		if err != nil {
			return 0, err
		}
		return int64(f * 1024), nil
	case strings.HasSuffix(value, "MiB"):
		f, err := strconv.ParseFloat(value[:len(value)-3], 64)
		if err != nil {
			return 0, err
		}
		return int64(f * 1024 * 1024), nil
	case strings.HasSuffix(value, "GiB"):
		f, err := strconv.ParseFloat(value[:len(value)-3], 64)
		if err != nil {
			return 0, err
		}
		return int64(f * 1024 * 1024 * 1024), nil
	case strings.HasSuffix(value, "TiB"):
		f, err := strconv.ParseFloat(value[:len(value)-3], 64)
		if err != nil {
			return 0, err
		}
		return int64(f * 1024 * 1024 * 1024 * 1024), nil
	default:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, err
		}
		return int64(f), nil
	}
}

func normalizeBytesString(s string) string {
	s = strings.ToUpper(s)
	return strings.ReplaceAll(s, "I", "i")
}
