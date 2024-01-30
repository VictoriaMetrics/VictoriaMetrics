package easyproto

import (
	"encoding/binary"
	"math"
	"math/bits"
	"sync"
)

// MarshalerPool is a pool of Marshaler structs.
type MarshalerPool struct {
	p sync.Pool
}

// Get obtains a Marshaler from the pool.
//
// The returned Marshaler can be returned to the pool via Put after it is no longer needed.
func (mp *MarshalerPool) Get() *Marshaler {
	v := mp.p.Get()
	if v == nil {
		return &Marshaler{}
	}
	return v.(*Marshaler)
}

// Put returns the given m to the pool.
//
// m cannot be used after returning to the pool.
func (mp *MarshalerPool) Put(m *Marshaler) {
	m.Reset()
	mp.p.Put(m)
}

// Marshaler helps marshaling arbitrary protobuf messages.
//
// Construct message with Append* functions at MessageMarshaler() and then call Marshal* for marshaling the constructed message.
//
// It is unsafe to use a single Marshaler instance from multiple concurrently running goroutines.
//
// It is recommended re-cycling Marshaler via MarshalerPool in order to reduce memory allocations.
type Marshaler struct {
	// mm contains the root MessageMarshaler.
	mm *MessageMarshaler

	// buf contains temporary data needed for marshaling the protobuf message.
	buf []byte

	// fs contains fields for the currently marshaled message.
	fs []field

	// mms contains MessageMarshaler structs for the currently marshaled message.
	mms []MessageMarshaler
}

// MessageMarshaler helps constructing protobuf message for marshaling.
//
// MessageMarshaler must be obtained via Marshaler.MessageMarshaler().
type MessageMarshaler struct {
	// m is the parent Marshaler for the given MessageMarshaler.
	m *Marshaler

	// tag contains protobuf message tag for the given MessageMarshaler.
	tag uint64

	// firstFieldIdx contains the index of the first field in the Marshaler.fs, which belongs to MessageMarshaler.
	firstFieldIdx int

	// lastFieldIdx is the index of the last field in the Marshaler.fs, which belongs to MessageMarshaler.
	lastFieldIdx int
}

func (mm *MessageMarshaler) reset() {
	mm.m = nil
	mm.tag = 0
	mm.firstFieldIdx = -1
	mm.lastFieldIdx = -1
}

type field struct {
	// messageSize is the size of marshaled protobuf message for the given field.
	messageSize uint64

	// dataStart is the start offset of field data at Marshaler.buf.
	dataStart int

	// dataEnd is the end offset of field data at Marshaler.buf.
	dataEnd int

	// nextFieldIdx contains an index of the next field in Marshaler.fs.
	nextFieldIdx int

	// childMessageMarshalerIdx contains an index of child MessageMarshaler in Marshaler.mms.
	childMessageMarshalerIdx int
}

func (f *field) reset() {
	f.messageSize = 0
	f.dataStart = 0
	f.dataEnd = 0
	f.nextFieldIdx = -1
	f.childMessageMarshalerIdx = -1
}

// Reset resets m, so it can be re-used.
func (m *Marshaler) Reset() {
	m.mm = nil
	m.buf = m.buf[:0]

	// There is no need in resetting individual fields, since they are reset in newFieldIndex()
	m.fs = m.fs[:0]

	// There is no need in resetting individual MessageMarshaler items, since they are reset in newMessageMarshalerIndex()
	m.mms = m.mms[:0]
}

// MarshalWithLen marshals m, appends its length together with the marshaled m to dst and returns the result.
//
// E.g. appends length-delimited protobuf message to dst.
// The length of the resulting message can be read via UnmarshalMessageLen() function.
//
// See also Marshal.
func (m *Marshaler) MarshalWithLen(dst []byte) []byte {
	if m.mm == nil {
		dst = marshalVarUint64(dst, 0)
		return dst
	}
	if firstFieldIdx := m.mm.firstFieldIdx; firstFieldIdx >= 0 {
		f := &m.fs[firstFieldIdx]
		messageSize := f.initMessageSize(m)
		if cap(dst) == 0 {
			dst = make([]byte, messageSize+10)
			dst = dst[:0]
		}
		dst = marshalVarUint64(dst, messageSize)
		dst = f.marshal(dst, m)
	}
	return dst
}

// Marshal appends marshaled protobuf m to dst and returns the result.
//
// The marshaled message can be read via FieldContext.NextField().
//
// See also MarshalWithLen.
func (m *Marshaler) Marshal(dst []byte) []byte {
	if m.mm == nil {
		// Nothing to marshal
		return dst
	}
	if firstFieldIdx := m.mm.firstFieldIdx; firstFieldIdx >= 0 {
		f := &m.fs[firstFieldIdx]
		messageSize := f.initMessageSize(m)
		if cap(dst) == 0 {
			dst = make([]byte, messageSize)
			dst = dst[:0]
		}
		dst = f.marshal(dst, m)
	}
	return dst
}

// MessageMarshaler returns message marshaler for the given m.
func (m *Marshaler) MessageMarshaler() *MessageMarshaler {
	if mm := m.mm; mm != nil {
		return mm
	}
	idx := m.newMessageMarshalerIndex()
	mm := &m.mms[idx]
	m.mm = mm
	return mm
}

func (m *Marshaler) newMessageMarshalerIndex() int {
	mms := m.mms
	mmsLen := len(mms)
	if cap(mms) > mmsLen {
		mms = mms[:mmsLen+1]
	} else {
		mms = append(mms, MessageMarshaler{})
	}
	m.mms = mms
	mm := &mms[mmsLen]
	mm.reset()
	mm.m = m
	return mmsLen
}

func (m *Marshaler) newFieldIndex() int {
	fs := m.fs
	fsLen := len(fs)
	if cap(fs) > fsLen {
		fs = fs[:fsLen+1]
	} else {
		fs = append(fs, field{})
	}
	m.fs = fs
	fs[fsLen].reset()
	return fsLen
}

// AppendInt32 appends the given int32 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendInt32(fieldNum uint32, i32 int32) {
	mm.AppendUint64(fieldNum, uint64(uint32(i32)))
}

// AppendInt64 appends the given int64 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendInt64(fieldNum uint32, i64 int64) {
	mm.AppendUint64(fieldNum, uint64(i64))
}

// AppendUint32 appends the given uint32 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendUint32(fieldNum, u32 uint32) {
	mm.AppendUint64(fieldNum, uint64(u32))
}

// AppendUint64 appends the given uint64 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendUint64(fieldNum uint32, u64 uint64) {
	tag := makeTag(fieldNum, wireTypeVarint)

	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	if tag < 0x80 {
		dst = append(dst, byte(tag))
	} else {
		dst = marshalVarUint64(dst, tag)
	}
	dst = marshalVarUint64(dst, u64)
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

// AppendSint32 appends the given sint32 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendSint32(fieldNum uint32, i32 int32) {
	u64 := uint64(encodeZigZagInt32(i32))
	mm.AppendUint64(fieldNum, u64)
}

// AppendSint64 appends the given sint64 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendSint64(fieldNum uint32, i64 int64) {
	u64 := encodeZigZagInt64(i64)
	mm.AppendUint64(fieldNum, u64)
}

// AppendBool appends the given bool value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendBool(fieldNum uint32, v bool) {
	u64 := uint64(0)
	if v {
		u64 = 1
	}
	mm.AppendUint64(fieldNum, u64)
}

// AppendFixed64 appends fixed64 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendFixed64(fieldNum uint32, u64 uint64) {
	tag := makeTag(fieldNum, wireTypeI64)

	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	if tag < 0x80 {
		dst = append(dst, byte(tag))
	} else {
		dst = marshalVarUint64(dst, tag)
	}
	dst = marshalUint64(dst, u64)
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

// AppendSfixed64 appends sfixed64 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendSfixed64(fieldNum uint32, i64 int64) {
	mm.AppendFixed64(fieldNum, uint64(i64))
}

// AppendDouble appends double value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendDouble(fieldNum uint32, f float64) {
	u64 := math.Float64bits(f)
	mm.AppendFixed64(fieldNum, u64)
}

// AppendString appends string value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendString(fieldNum uint32, s string) {
	tag := makeTag(fieldNum, wireTypeLen)

	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	sLen := len(s)
	if tag < 0x80 && sLen < 0x80 {
		dst = append(dst, byte(tag), byte(sLen))
	} else {
		dst = marshalVarUint64(dst, tag)
		dst = marshalVarUint64(dst, uint64(sLen))
	}
	dst = append(dst, s...)
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

// AppendBytes appends bytes value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendBytes(fieldNum uint32, b []byte) {
	s := unsafeBytesToString(b)
	mm.AppendString(fieldNum, s)
}

// AppendMessage appends protobuf message with the given fieldNum to m.
//
// The function returns the MessageMarshaler for constructing the appended message.
func (mm *MessageMarshaler) AppendMessage(fieldNum uint32) *MessageMarshaler {
	tag := makeTag(fieldNum, wireTypeLen)

	f := mm.newField()
	m := mm.m
	f.childMessageMarshalerIdx = m.newMessageMarshalerIndex()
	mmChild := &m.mms[f.childMessageMarshalerIdx]
	mmChild.tag = tag
	return mmChild
}

// AppendFixed32 appends fixed32 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendFixed32(fieldNum, u32 uint32) {
	tag := makeTag(fieldNum, wireTypeI32)

	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	if tag < 0x80 {
		dst = append(dst, byte(tag))
	} else {
		dst = marshalVarUint64(dst, tag)
	}
	dst = marshalUint32(dst, u32)
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

// AppendSfixed32 appends sfixed32 value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendSfixed32(fieldNum uint32, i32 int32) {
	mm.AppendFixed32(fieldNum, uint32(i32))
}

// AppendFloat appends float value under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendFloat(fieldNum uint32, f float32) {
	u32 := math.Float32bits(f)
	mm.AppendFixed32(fieldNum, u32)
}

// AppendInt32s appends the given int32 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendInt32s(fieldNum uint32, i32s []int32) {
	child := mm.AppendMessage(fieldNum)
	child.appendInt32s(i32s)
}

// AppendInt64s appends the given int64 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendInt64s(fieldNum uint32, i64s []int64) {
	child := mm.AppendMessage(fieldNum)
	child.appendInt64s(i64s)
}

// AppendUint32s appends the given uint32 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendUint32s(fieldNum uint32, u32s []uint32) {
	child := mm.AppendMessage(fieldNum)
	child.appendUint32s(u32s)
}

// AppendUint64s appends the given uint64 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendUint64s(fieldNum uint32, u64s []uint64) {
	child := mm.AppendMessage(fieldNum)
	child.appendUint64s(u64s)
}

// AppendSint32s appends the given sint32 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendSint32s(fieldNum uint32, i32s []int32) {
	child := mm.AppendMessage(fieldNum)
	child.appendSint32s(i32s)
}

// AppendSint64s appends the given sint64 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendSint64s(fieldNum uint32, i64s []int64) {
	child := mm.AppendMessage(fieldNum)
	child.appendSint64s(i64s)
}

// AppendBools appends the given bool values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendBools(fieldNum uint32, bs []bool) {
	child := mm.AppendMessage(fieldNum)
	child.appendBools(bs)
}

// AppendFixed64s appends the given fixed64 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendFixed64s(fieldNum uint32, u64s []uint64) {
	child := mm.AppendMessage(fieldNum)
	child.appendFixed64s(u64s)
}

// AppendSfixed64s appends the given sfixed64 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendSfixed64s(fieldNum uint32, i64s []int64) {
	child := mm.AppendMessage(fieldNum)
	child.appendSfixed64s(i64s)
}

// AppendDoubles appends the given double values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendDoubles(fieldNum uint32, fs []float64) {
	child := mm.AppendMessage(fieldNum)
	child.appendDoubles(fs)
}

// AppendFixed32s appends the given fixed32 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendFixed32s(fieldNum uint32, u32s []uint32) {
	child := mm.AppendMessage(fieldNum)
	child.appendFixed32s(u32s)
}

// AppendSfixed32s appends the given sfixed32 values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendSfixed32s(fieldNum uint32, i32s []int32) {
	child := mm.AppendMessage(fieldNum)
	child.appendSfixed32s(i32s)
}

// AppendFloats appends the given float values under the given fieldNum to mm.
func (mm *MessageMarshaler) AppendFloats(fieldNum uint32, fs []float32) {
	child := mm.AppendMessage(fieldNum)
	child.appendFloats(fs)
}

func (mm *MessageMarshaler) appendInt32s(i32s []int32) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, i32 := range i32s {
		dst = marshalVarUint64(dst, uint64(uint32(i32)))
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendUint32s(u32s []uint32) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, u32 := range u32s {
		dst = marshalVarUint64(dst, uint64(u32))
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendSint32s(i32s []int32) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, i32 := range i32s {
		u64 := uint64(encodeZigZagInt32(i32))
		dst = marshalVarUint64(dst, u64)
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendInt64s(i64s []int64) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, i64 := range i64s {
		dst = marshalVarUint64(dst, uint64(i64))
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendUint64s(u64s []uint64) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, u64 := range u64s {
		dst = marshalVarUint64(dst, u64)
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendSint64s(i64s []int64) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, i64 := range i64s {
		u64 := encodeZigZagInt64(i64)
		dst = marshalVarUint64(dst, u64)
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendBools(bs []bool) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, b := range bs {
		u64 := uint64(0)
		if b {
			u64 = 1
		}
		dst = marshalVarUint64(dst, u64)
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendFixed64s(u64s []uint64) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, u64 := range u64s {
		dst = marshalUint64(dst, u64)
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendSfixed64s(i64s []int64) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, i64 := range i64s {
		dst = marshalUint64(dst, uint64(i64))
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendFixed32s(u32s []uint32) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, u32 := range u32s {
		dst = marshalUint32(dst, u32)
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendSfixed32s(i32s []int32) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, i32 := range i32s {
		dst = marshalUint32(dst, uint32(i32))
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendDoubles(fs []float64) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, f := range fs {
		u64 := math.Float64bits(f)
		dst = marshalUint64(dst, u64)
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendFloats(fs []float32) {
	m := mm.m
	dst := m.buf
	dstLen := len(dst)
	for _, f := range fs {
		u32 := math.Float32bits(f)
		dst = marshalUint32(dst, u32)
	}
	m.buf = dst

	mm.appendField(m, dstLen, len(dst))
}

func (mm *MessageMarshaler) appendField(m *Marshaler, dataStart, dataEnd int) {
	if lastFieldIdx := mm.lastFieldIdx; lastFieldIdx >= 0 {
		if f := &m.fs[lastFieldIdx]; f.childMessageMarshalerIdx == -1 && f.dataEnd == dataStart {
			f.dataEnd = dataEnd
			return
		}
	}
	f := mm.newField()
	f.dataStart = dataStart
	f.dataEnd = dataEnd
}

func (mm *MessageMarshaler) newField() *field {
	m := mm.m
	idx := m.newFieldIndex()
	f := &m.fs[idx]
	if lastFieldIdx := mm.lastFieldIdx; lastFieldIdx >= 0 {
		m.fs[lastFieldIdx].nextFieldIdx = idx
	} else {
		mm.firstFieldIdx = idx
	}
	mm.lastFieldIdx = idx
	return f
}

func (f *field) initMessageSize(m *Marshaler) uint64 {
	n := uint64(0)
	for {
		if childMessageMarshalerIdx := f.childMessageMarshalerIdx; childMessageMarshalerIdx < 0 {
			n += uint64(f.dataEnd - f.dataStart)
		} else {
			mmChild := m.mms[childMessageMarshalerIdx]
			if tag := mmChild.tag; tag < 0x80 {
				n++
			} else {
				n += varuintLen(tag)
			}
			messageSize := uint64(0)
			if firstFieldIdx := mmChild.firstFieldIdx; firstFieldIdx >= 0 {
				messageSize = m.fs[firstFieldIdx].initMessageSize(m)
			}
			n += messageSize
			if messageSize < 0x80 {
				n++
			} else {
				n += varuintLen(messageSize)
			}
			f.messageSize = messageSize
		}
		nextFieldIdx := f.nextFieldIdx
		if nextFieldIdx < 0 {
			return n
		}
		f = &m.fs[nextFieldIdx]
	}
}

func (f *field) marshal(dst []byte, m *Marshaler) []byte {
	for {
		if childMessageMarshalerIdx := f.childMessageMarshalerIdx; childMessageMarshalerIdx < 0 {
			data := m.buf[f.dataStart:f.dataEnd]
			dst = append(dst, data...)
		} else {
			mmChild := m.mms[childMessageMarshalerIdx]
			tag := mmChild.tag
			messageSize := f.messageSize
			if tag < 0x80 && messageSize < 0x80 {
				dst = append(dst, byte(tag), byte(messageSize))
			} else {
				dst = marshalVarUint64(dst, mmChild.tag)
				dst = marshalVarUint64(dst, f.messageSize)
			}
			if firstFieldIdx := mmChild.firstFieldIdx; firstFieldIdx >= 0 {
				dst = m.fs[firstFieldIdx].marshal(dst, m)
			}
		}
		nextFieldIdx := f.nextFieldIdx
		if nextFieldIdx < 0 {
			return dst
		}
		f = &m.fs[nextFieldIdx]
	}
}

func marshalUint64(dst []byte, u64 uint64) []byte {
	return binary.LittleEndian.AppendUint64(dst, u64)
}

func marshalUint32(dst []byte, u32 uint32) []byte {
	return binary.LittleEndian.AppendUint32(dst, u32)
}

func marshalVarUint64(dst []byte, u64 uint64) []byte {
	if u64 < 0x80 {
		// Fast path
		dst = append(dst, byte(u64))
		return dst
	}
	for u64 > 0x7f {
		dst = append(dst, 0x80|byte(u64))
		u64 >>= 7
	}
	dst = append(dst, byte(u64))
	return dst
}

func encodeZigZagInt64(i64 int64) uint64 {
	return uint64((i64 << 1) ^ (i64 >> 63))
}

func encodeZigZagInt32(i32 int32) uint32 {
	return uint32((i32 << 1) ^ (i32 >> 31))
}

func makeTag(fieldNum uint32, wt wireType) uint64 {
	return (uint64(fieldNum) << 3) | uint64(wt)
}

// varuintLen returns the number of bytes needed for varuint-encoding of u64.
//
// Note that it returns 0 for u64=0, so this case must be handled separately.
func varuintLen(u64 uint64) uint64 {
	return uint64(((byte(bits.Len64(u64))) + 6) / 7)
}
