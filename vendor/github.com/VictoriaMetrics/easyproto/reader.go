package easyproto

import (
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
)

// FieldContext represents a single protobuf-encoded field after NextField() call.
type FieldContext struct {
	// FieldNum is the number of protobuf field read after NextField() call.
	FieldNum uint32

	// wireType is the wire type for the given field
	wireType wireType

	// data is probobuf-encoded field data for wireType=wireTypeLen
	data []byte

	// intValue contains int value for wireType!=wireTypeLen
	intValue uint64
}

// NextField reads the next field from protobuf-encoded src.
//
// It returns the tail left after reading the next field from src.
//
// It is unsafe modifying src while FieldContext is in use.
func (fc *FieldContext) NextField(src []byte) ([]byte, error) {
	if len(src) >= 2 {
		n := uint16(src[0])<<8 | uint16(src[1])
		if (n&0x8080 == 0) && (n&0x0700 == (uint16(wireTypeLen) << 8)) {
			// Fast path - read message with the length smaller than 0x80 bytes.
			msgLen := int(n & 0xff)
			src = src[2:]
			if len(src) < msgLen {
				return src, fmt.Errorf("cannot read field for from %d bytes; need at least %d bytes", len(src), msgLen)
			}
			fc.FieldNum = uint32(n >> (8 + 3))
			fc.wireType = wireTypeLen
			fc.data = src[:msgLen]
			src = src[msgLen:]
			return src, nil
		}
	}

	// Read field tag. See https://protobuf.dev/programming-guides/encoding/#structure
	if len(src) == 0 {
		return src, fmt.Errorf("cannot unmarshal field from empty message")
	}

	var fieldNum uint64
	tag := uint64(src[0])
	if tag < 0x80 {
		src = src[1:]
		fieldNum = tag >> 3
	} else {
		var offset int
		tag, offset = binary.Uvarint(src)
		if offset <= 0 {
			return src, fmt.Errorf("cannot unmarshal field tag from uvarint")
		}
		src = src[offset:]
		fieldNum = tag >> 3
		if fieldNum > math.MaxUint32 {
			return src, fmt.Errorf("fieldNum=%d is bigger than uint32max=%d", fieldNum, uint64(math.MaxUint32))
		}
	}

	wt := wireType(tag & 0x07)

	fc.FieldNum = uint32(fieldNum)
	fc.wireType = wt

	// Read the remaining data
	if wt == wireTypeLen {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return src, fmt.Errorf("cannot read message length for field #%d", fieldNum)
		}
		src = src[offset:]
		if uint64(len(src)) < u64 {
			return src, fmt.Errorf("cannot read data for field #%d from %d bytes; need at least %d bytes", fieldNum, len(src), u64)
		}
		fc.data = src[:u64]
		src = src[u64:]
		return src, nil
	}
	if wt == wireTypeVarint {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return src, fmt.Errorf("cannot read varint after field tag for field #%d", fieldNum)
		}
		src = src[offset:]
		fc.intValue = u64
		return src, nil
	}
	if wt == wireTypeI64 {
		if len(src) < 8 {
			return src, fmt.Errorf("cannot read i64 for field #%d", fieldNum)
		}
		u64 := binary.LittleEndian.Uint64(src)
		src = src[8:]
		fc.intValue = u64
		return src, nil
	}
	if wt == wireTypeI32 {
		if len(src) < 4 {
			return src, fmt.Errorf("cannot read i32 for field #%d", fieldNum)
		}
		u32 := binary.LittleEndian.Uint32(src)
		src = src[4:]
		fc.intValue = uint64(u32)
		return src, nil
	}
	return src, fmt.Errorf("unknown wireType=%d", wt)
}

// UnmarshalMessageLen unmarshals protobuf message length from src.
//
// It returns the tail left after unmarshaling message length from src.
//
// It is expected that src is marshaled with Marshaler.MarshalWithLen().
//
// False is returned if message length cannot be unmarshaled from src.
func UnmarshalMessageLen(src []byte) (int, []byte, bool) {
	u64, offset := binary.Uvarint(src)
	if offset <= 0 {
		return 0, src, false
	}
	src = src[offset:]
	if u64 > math.MaxInt32 {
		return 0, src, false
	}
	return int(u64), src, true
}

// wireType is the type of of protobuf-encoded field
//
// See https://protobuf.dev/programming-guides/encoding/#structure
type wireType byte

const (
	// VARINT type - one of int32, int64, uint32, uint64, sint32, sint64, bool, enum
	wireTypeVarint = wireType(0)

	// I64 type
	wireTypeI64 = wireType(1)

	// Len type
	wireTypeLen = wireType(2)

	// I32 type
	wireTypeI32 = wireType(5)
)

func (wt wireType) String() string {
	switch wt {
	case wireTypeVarint:
		return "varint"
	case wireTypeI64:
		return "i64"
	case wireTypeLen:
		return "len"
	case wireTypeI32:
		return "i32"
	default:
		return fmt.Sprintf("unknown (%d)", int(wt))
	}
}

// Int32 returns int32 value for fc.
//
// False is returned if fc doesn't contain int32 value.
func (fc *FieldContext) Int32() (int32, bool) {
	if fc.wireType != wireTypeVarint {
		return 0, false
	}
	return getInt32(fc.intValue)
}

// Int64 returns int64 value for fc.
//
// False is returned if fc doesn't contain int64 value.
func (fc *FieldContext) Int64() (int64, bool) {
	if fc.wireType != wireTypeVarint {
		return 0, false
	}
	return int64(fc.intValue), true
}

// Uint32 returns uint32 value for fc.
//
// False is returned if fc doesn't contain uint32 value.
func (fc *FieldContext) Uint32() (uint32, bool) {
	if fc.wireType != wireTypeVarint {
		return 0, false
	}
	return getUint32(fc.intValue)
}

// Uint64 returns uint64 value for fc.
//
// False is returned if fc doesn't contain uint64 value.
func (fc *FieldContext) Uint64() (uint64, bool) {
	if fc.wireType != wireTypeVarint {
		return 0, false
	}
	return fc.intValue, true
}

// Sint32 returns sint32 value for fc.
//
// False is returned if fc doesn't contain sint32 value.
func (fc *FieldContext) Sint32() (int32, bool) {
	if fc.wireType != wireTypeVarint {
		return 0, false
	}
	u32, ok := getUint32(fc.intValue)
	if !ok {
		return 0, false
	}
	i32 := decodeZigZagInt32(u32)
	return i32, true
}

// Sint64 returns sint64 value for fc.
//
// False is returned if fc doesn't contain sint64 value.
func (fc *FieldContext) Sint64() (int64, bool) {
	if fc.wireType != wireTypeVarint {
		return 0, false
	}
	i64 := decodeZigZagInt64(fc.intValue)
	return i64, true
}

// Bool returns bool value for fc.
//
// False is returned in the second result if fc doesn't contain bool value.
func (fc *FieldContext) Bool() (bool, bool) {
	if fc.wireType != wireTypeVarint {
		return false, false
	}
	return getBool(fc.intValue)
}

// Enum returns enum value for fc.
//
// False is returned if fc doesn't contain enum value.
func (fc *FieldContext) Enum() (int32, bool) {
	if fc.wireType != wireTypeVarint {
		return 0, false
	}
	return getInt32(fc.intValue)
}

// Fixed64 returns fixed64 value for fc.
//
// False is returned if fc doesn't contain fixed64 value.
func (fc *FieldContext) Fixed64() (uint64, bool) {
	if fc.wireType != wireTypeI64 {
		return 0, false
	}
	return fc.intValue, true
}

// Sfixed64 returns sfixed64 value for fc.
//
// False is returned if fc doesn't contain sfixed64 value.
func (fc *FieldContext) Sfixed64() (int64, bool) {
	if fc.wireType != wireTypeI64 {
		return 0, false
	}
	return int64(fc.intValue), true
}

// Double returns dobule value for fc.
//
// False is returned if fc doesn't contain double value.
func (fc *FieldContext) Double() (float64, bool) {
	if fc.wireType != wireTypeI64 {
		return 0, false
	}
	v := math.Float64frombits(fc.intValue)
	return v, true
}

// String returns string value for fc.
//
// The returned string is valid while the underlying buffer isn't changed.
//
// False is returned if fc doesn't contain string value.
func (fc *FieldContext) String() (string, bool) {
	if fc.wireType != wireTypeLen {
		return "", false
	}
	s := unsafeBytesToString(fc.data)
	return s, true
}

// Bytes returns bytes value for fc.
//
// The returned byte slice is valid while the underlying buffer isn't changed.
//
// False is returned if fc doesn't contain bytes value.
func (fc *FieldContext) Bytes() ([]byte, bool) {
	if fc.wireType != wireTypeLen {
		return nil, false
	}
	return fc.data, true
}

// MessageData returns protobuf message data for fc.
//
// False is returned if fc doesn't contain message data.
func (fc *FieldContext) MessageData() ([]byte, bool) {
	if fc.wireType != wireTypeLen {
		return nil, false
	}
	return fc.data, true
}

// Fixed32 returns fixed32 value for fc.
//
// False is returned if fc doesn't contain fixed32 value.
func (fc *FieldContext) Fixed32() (uint32, bool) {
	if fc.wireType != wireTypeI32 {
		return 0, false
	}
	u32 := mustGetUint32(fc.intValue)
	return u32, true
}

// Sfixed32 returns sfixed32 value for fc.
//
// False is returned if fc doesn't contain sfixed value.
func (fc *FieldContext) Sfixed32() (int32, bool) {
	if fc.wireType != wireTypeI32 {
		return 0, false
	}
	i32 := mustGetInt32(fc.intValue)
	return i32, true
}

// Float returns float value for fc.
//
// False is returned if fc doesn't contain float value.
func (fc *FieldContext) Float() (float32, bool) {
	if fc.wireType != wireTypeI32 {
		return 0, false
	}
	u32 := mustGetUint32(fc.intValue)
	v := math.Float32frombits(u32)
	return v, true
}

// UnpackInt32s unpacks int32 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain int32 values.
func (fc *FieldContext) UnpackInt32s(dst []int32) ([]int32, bool) {
	if fc.wireType == wireTypeVarint {
		i32, ok := getInt32(fc.intValue)
		if !ok {
			return dst, false
		}
		dst = append(dst, i32)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return dstOrig, false
		}
		src = src[offset:]
		i32, ok := getInt32(u64)
		if !ok {
			return dstOrig, false
		}
		dst = append(dst, i32)
	}
	return dst, true
}

// UnpackInt64s unpacks int64 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain int64 values.
func (fc *FieldContext) UnpackInt64s(dst []int64) ([]int64, bool) {
	if fc.wireType == wireTypeVarint {
		dst = append(dst, int64(fc.intValue))
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return dstOrig, false
		}
		src = src[offset:]
		dst = append(dst, int64(u64))
	}
	return dst, true
}

// UnpackUint32s unpacks uint32 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain uint32 values.
func (fc *FieldContext) UnpackUint32s(dst []uint32) ([]uint32, bool) {
	if fc.wireType == wireTypeVarint {
		u32, ok := getUint32(fc.intValue)
		if !ok {
			return dst, false
		}
		dst = append(dst, u32)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return dstOrig, false
		}
		src = src[offset:]
		u32, ok := getUint32(u64)
		if !ok {
			return dstOrig, false
		}
		dst = append(dst, u32)
	}
	return dst, true
}

// UnpackUint64s unpacks uint64 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain uint64 values.
func (fc *FieldContext) UnpackUint64s(dst []uint64) ([]uint64, bool) {
	if fc.wireType == wireTypeVarint {
		dst = append(dst, fc.intValue)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return dstOrig, false
		}
		src = src[offset:]
		dst = append(dst, u64)
	}
	return dst, true
}

// UnpackSint32s unpacks sint32 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain sint32 values.
func (fc *FieldContext) UnpackSint32s(dst []int32) ([]int32, bool) {
	if fc.wireType == wireTypeVarint {
		u32, ok := getUint32(fc.intValue)
		if !ok {
			return dst, false
		}
		i32 := decodeZigZagInt32(u32)
		dst = append(dst, i32)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return dstOrig, false
		}
		src = src[offset:]
		u32, ok := getUint32(u64)
		if !ok {
			return dstOrig, false
		}
		i32 := decodeZigZagInt32(u32)
		dst = append(dst, i32)
	}
	return dst, true
}

// UnpackSint64s unpacks sint64 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain sint64 values.
func (fc *FieldContext) UnpackSint64s(dst []int64) ([]int64, bool) {
	if fc.wireType == wireTypeVarint {
		i64 := decodeZigZagInt64(fc.intValue)
		dst = append(dst, i64)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return dstOrig, false
		}
		src = src[offset:]
		i64 := decodeZigZagInt64(u64)
		dst = append(dst, i64)
	}
	return dst, true
}

// UnpackBools unpacks bool values from fc, appends them to dst and returns the result.
//
// False is returned in the second result if fc doesn't contain bool values.
func (fc *FieldContext) UnpackBools(dst []bool) ([]bool, bool) {
	if fc.wireType == wireTypeVarint {
		v, ok := getBool(fc.intValue)
		if !ok {
			return dst, false
		}
		dst = append(dst, v)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		u64, offset := binary.Uvarint(src)
		if offset <= 0 {
			return dstOrig, false
		}
		src = src[offset:]
		v, ok := getBool(u64)
		if !ok {
			return dst, false
		}
		dst = append(dst, v)
	}
	return dst, true
}

// UnpackFixed64s unpacks fixed64 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain fixed64 values.
func (fc *FieldContext) UnpackFixed64s(dst []uint64) ([]uint64, bool) {
	if fc.wireType == wireTypeI64 {
		u64 := fc.intValue
		dst = append(dst, u64)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		if len(src) < 8 {
			return dstOrig, false
		}
		u64 := binary.LittleEndian.Uint64(src)
		src = src[8:]
		dst = append(dst, u64)
	}
	return dst, true
}

// UnpackSfixed64s unpacks sfixed64 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain sfixed64 values.
func (fc *FieldContext) UnpackSfixed64s(dst []int64) ([]int64, bool) {
	if fc.wireType == wireTypeI64 {
		u64 := fc.intValue
		dst = append(dst, int64(u64))
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		if len(src) < 8 {
			return dstOrig, false
		}
		u64 := binary.LittleEndian.Uint64(src)
		src = src[8:]
		dst = append(dst, int64(u64))
	}
	return dst, true
}

// UnpackDoubles unpacks double values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain double values.
func (fc *FieldContext) UnpackDoubles(dst []float64) ([]float64, bool) {
	if fc.wireType == wireTypeI64 {
		v := math.Float64frombits(fc.intValue)
		dst = append(dst, v)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		if len(src) < 8 {
			return dstOrig, false
		}
		u64 := binary.LittleEndian.Uint64(src)
		src = src[8:]
		v := math.Float64frombits(u64)
		dst = append(dst, v)
	}
	return dst, true
}

// UnpackFixed32s unpacks fixed32 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain fixed32 values.
func (fc *FieldContext) UnpackFixed32s(dst []uint32) ([]uint32, bool) {
	if fc.wireType == wireTypeI32 {
		u32 := mustGetUint32(fc.intValue)
		dst = append(dst, u32)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		if len(src) < 4 {
			return dstOrig, false
		}
		u32 := binary.LittleEndian.Uint32(src)
		src = src[4:]
		dst = append(dst, u32)
	}
	return dst, true
}

// UnpackSfixed32s unpacks sfixed32 values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain sfixed32 values.
func (fc *FieldContext) UnpackSfixed32s(dst []int32) ([]int32, bool) {
	if fc.wireType == wireTypeI32 {
		i32 := mustGetInt32(fc.intValue)
		dst = append(dst, i32)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		if len(src) < 4 {
			return dstOrig, false
		}
		u32 := binary.LittleEndian.Uint32(src)
		src = src[4:]
		dst = append(dst, int32(u32))
	}
	return dst, true
}

// UnpackFloats unpacks float values from fc, appends them to dst and returns the result.
//
// False is returned if fc doesn't contain float values.
func (fc *FieldContext) UnpackFloats(dst []float32) ([]float32, bool) {
	if fc.wireType == wireTypeI32 {
		u32 := mustGetUint32(fc.intValue)
		v := math.Float32frombits(u32)
		dst = append(dst, v)
		return dst, true
	}
	if fc.wireType != wireTypeLen {
		return dst, false
	}
	src := fc.data
	dstOrig := dst
	for len(src) > 0 {
		if len(src) < 4 {
			return dstOrig, false
		}
		u32 := binary.LittleEndian.Uint32(src)
		src = src[4:]
		v := math.Float32frombits(u32)
		dst = append(dst, v)
	}
	return dst, true
}

func (fc *FieldContext) getField(src []byte, fieldNum uint32, neededWireType wireType) (bool, error) {
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return false, fmt.Errorf("cannot read the next field: %w", err)
		}
		if fc.FieldNum != fieldNum {
			continue
		}
		if fc.wireType != neededWireType {
			return false, fmt.Errorf("fieldNum=%d contains unexpected wireType; got %s; want %s", fieldNum, fc.wireType, neededWireType)
		}
		return true, nil
	}
	return false, nil
}

// GetInt32 returns the int32 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetInt32(src []byte, fieldNum uint32) (n int32, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeVarint)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	n, ok = getInt32(fc.intValue)
	if !ok {
		return 0, false, fmt.Errorf("fieldNum=%d contains too big integer %d, which cannot be converted to int32", fieldNum, fc.intValue)
	}
	return n, true, nil
}

// GetInt64 returns the int64 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetInt64(src []byte, fieldNum uint32) (n int64, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeVarint)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	return int64(fc.intValue), true, nil
}

// GetUint32 returns the uint32 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetUint32(src []byte, fieldNum uint32) (n uint32, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeVarint)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	n, ok = getUint32(fc.intValue)
	if !ok {
		return 0, false, fmt.Errorf("fieldNum=%d contains too big integer %d, which cannot be converted to uint32", fieldNum, fc.intValue)
	}
	return n, true, nil
}

// GetUint64 returns the int64 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetUint64(src []byte, fieldNum uint32) (n uint64, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeVarint)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	return fc.intValue, true, nil
}

// GetSint32 returns sint32 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetSint32(src []byte, fieldNum uint32) (n int32, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeVarint)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	u32, ok := getUint32(fc.intValue)
	if !ok {
		return 0, false, fmt.Errorf("fieldNum=%d contains too big integer %d, which cannot be converted to uint32", fieldNum, fc.intValue)
	}
	n = decodeZigZagInt32(u32)
	return n, true, nil
}

// GetSint64 returns sint64 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetSint64(src []byte, fieldNum uint32) (n int64, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeVarint)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	n = decodeZigZagInt64(fc.intValue)
	return n, true, nil
}

// GetBool returns bool value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetBool(src []byte, fieldNum uint32) (b bool, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeVarint)
	if err != nil {
		return false, false, err
	}
	if !ok {
		return false, false, nil
	}
	b, ok = getBool(fc.intValue)
	if !ok {
		return false, false, fmt.Errorf("fieldNum=%d contains invalid integer %d, which cannot be converted to bool", fieldNum, fc.intValue)
	}
	return b, true, nil
}

// GetEnum returns enum value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetEnum(src []byte, fieldNum uint32) (n int32, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeVarint)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	n, ok = getInt32(fc.intValue)
	if !ok {
		return 0, false, fmt.Errorf("fieldNum=%d contains invalid integer %d, which cannot be converted to enum", fieldNum, fc.intValue)
	}
	return n, true, nil
}

// GetFixed64 returns fixed64 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetFixed64(src []byte, fieldNum uint32) (n uint64, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeI64)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	return fc.intValue, true, nil
}

// GetSfixed64 returns sfixed64 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetSfixed64(src []byte, fieldNum uint32) (n int64, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeI64)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	return int64(fc.intValue), true, nil
}

// GetDouble returns double value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetDouble(src []byte, fieldNum uint32) (f float64, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeI64)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	f = math.Float64frombits(fc.intValue)
	return f, true, nil
}

// GetString returns string value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
// The returned string is valid until src is changed.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetString(src []byte, fieldNum uint32) (s string, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeLen)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return unsafeBytesToString(fc.data), true, nil
}

// GetBytes returns bytes slice for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
// The returned bytes slice is valid until src is changed.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetBytes(src []byte, fieldNum uint32) (b []byte, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeLen)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return fc.data, true, nil
}

// GetMessageData returns message data for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
// The returned message data is valid until src is changed.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetMessageData(src []byte, fieldNum uint32) (data []byte, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeLen)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return fc.data, true, nil
}

// GetFixed32 returns fixed32 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetFixed32(src []byte, fieldNum uint32) (n uint32, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeI32)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	n = mustGetUint32(fc.intValue)
	return n, true, nil
}

// GetSfixed32 returns sfixed32 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetSfixed32(src []byte, fieldNum uint32) (n int32, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeI32)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	n = mustGetInt32(fc.intValue)
	return n, true, nil
}

// GetFloat returns float32 value for the given fieldNum from protobuf-encoded message at src.
//
// ok=false is returned if src doesn't contain the given fieldNum.
//
// This function is useful when only a single message with the given fieldNum must be obtained from protobuf-encoded src.
// Otherwise use FieldContext for obtaining multiple message from protobuf-encoded src.
func GetFloat(src []byte, fieldNum uint32) (f float32, ok bool, err error) {
	var fc FieldContext
	ok, err = fc.getField(src, fieldNum, wireTypeI32)
	if err != nil {
		return 0, false, err
	}
	if !ok {
		return 0, false, nil
	}
	u32 := mustGetUint32(fc.intValue)
	f = math.Float32frombits(u32)
	return f, true, nil
}

func decodeZigZagInt64(u64 uint64) int64 {
	return int64(u64>>1) ^ (int64(u64<<63) >> 63)
}

func decodeZigZagInt32(u32 uint32) int32 {
	return int32(u32>>1) ^ (int32(u32<<31) >> 31)
}

func unsafeBytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func getInt32(u64 uint64) (int32, bool) {
	u32, ok := getUint32(u64)
	if !ok {
		return 0, false
	}
	return int32(u32), true
}

func getUint32(u64 uint64) (uint32, bool) {
	if u64 > math.MaxUint32 {
		return 0, false
	}
	return uint32(u64), true
}

func mustGetInt32(u64 uint64) int32 {
	u32 := mustGetUint32(u64)
	return int32(u32)
}

func mustGetUint32(u64 uint64) uint32 {
	u32, ok := getUint32(u64)
	if !ok {
		panic(fmt.Errorf("BUG: cannot get uint32 from %d", u64))
	}
	return u32
}

func getBool(u64 uint64) (bool, bool) {
	if u64 == 0 {
		return false, true
	}
	if u64 == 1 {
		return true, true
	}
	return false, false
}
