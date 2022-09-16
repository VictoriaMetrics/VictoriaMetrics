// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package pb

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	_ "github.com/gogo/protobuf/gogoproto" // comment justifying it
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *ClientGroupedStats) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Service":
			z.Service, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Name":
			z.Name, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Resource":
			z.Resource, err = dc.ReadString()
			if err != nil {
				return
			}
		case "HTTPStatusCode":
			z.HTTPStatusCode, err = dc.ReadUint32()
			if err != nil {
				return
			}
		case "Type":
			z.Type, err = dc.ReadString()
			if err != nil {
				return
			}
		case "DBType":
			z.DBType, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Hits":
			z.Hits, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "Errors":
			z.Errors, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "Duration":
			z.Duration, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "OkSummary":
			z.OkSummary, err = dc.ReadBytes(z.OkSummary)
			if err != nil {
				return
			}
		case "ErrorSummary":
			z.ErrorSummary, err = dc.ReadBytes(z.ErrorSummary)
			if err != nil {
				return
			}
		case "Synthetics":
			z.Synthetics, err = dc.ReadBool()
			if err != nil {
				return
			}
		case "TopLevelHits":
			z.TopLevelHits, err = dc.ReadUint64()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *ClientGroupedStats) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 13
	// write "Service"
	err = en.Append(0x8d, 0xa7, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Service)
	if err != nil {
		return
	}
	// write "Name"
	err = en.Append(0xa4, 0x4e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Name)
	if err != nil {
		return
	}
	// write "Resource"
	err = en.Append(0xa8, 0x52, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Resource)
	if err != nil {
		return
	}
	// write "HTTPStatusCode"
	err = en.Append(0xae, 0x48, 0x54, 0x54, 0x50, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x43, 0x6f, 0x64, 0x65)
	if err != nil {
		return
	}
	err = en.WriteUint32(z.HTTPStatusCode)
	if err != nil {
		return
	}
	// write "Type"
	err = en.Append(0xa4, 0x54, 0x79, 0x70, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Type)
	if err != nil {
		return
	}
	// write "DBType"
	err = en.Append(0xa6, 0x44, 0x42, 0x54, 0x79, 0x70, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.DBType)
	if err != nil {
		return
	}
	// write "Hits"
	err = en.Append(0xa4, 0x48, 0x69, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteUint64(z.Hits)
	if err != nil {
		return
	}
	// write "Errors"
	err = en.Append(0xa6, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x73)
	if err != nil {
		return
	}
	err = en.WriteUint64(z.Errors)
	if err != nil {
		return
	}
	// write "Duration"
	err = en.Append(0xa8, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteUint64(z.Duration)
	if err != nil {
		return
	}
	// write "OkSummary"
	err = en.Append(0xa9, 0x4f, 0x6b, 0x53, 0x75, 0x6d, 0x6d, 0x61, 0x72, 0x79)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.OkSummary)
	if err != nil {
		return
	}
	// write "ErrorSummary"
	err = en.Append(0xac, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x53, 0x75, 0x6d, 0x6d, 0x61, 0x72, 0x79)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.ErrorSummary)
	if err != nil {
		return
	}
	// write "Synthetics"
	err = en.Append(0xaa, 0x53, 0x79, 0x6e, 0x74, 0x68, 0x65, 0x74, 0x69, 0x63, 0x73)
	if err != nil {
		return
	}
	err = en.WriteBool(z.Synthetics)
	if err != nil {
		return
	}
	// write "TopLevelHits"
	err = en.Append(0xac, 0x54, 0x6f, 0x70, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x48, 0x69, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteUint64(z.TopLevelHits)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ClientGroupedStats) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 13
	// string "Service"
	o = append(o, 0x8d, 0xa7, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65)
	o = msgp.AppendString(o, z.Service)
	// string "Name"
	o = append(o, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Name)
	// string "Resource"
	o = append(o, 0xa8, 0x52, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65)
	o = msgp.AppendString(o, z.Resource)
	// string "HTTPStatusCode"
	o = append(o, 0xae, 0x48, 0x54, 0x54, 0x50, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x43, 0x6f, 0x64, 0x65)
	o = msgp.AppendUint32(o, z.HTTPStatusCode)
	// string "Type"
	o = append(o, 0xa4, 0x54, 0x79, 0x70, 0x65)
	o = msgp.AppendString(o, z.Type)
	// string "DBType"
	o = append(o, 0xa6, 0x44, 0x42, 0x54, 0x79, 0x70, 0x65)
	o = msgp.AppendString(o, z.DBType)
	// string "Hits"
	o = append(o, 0xa4, 0x48, 0x69, 0x74, 0x73)
	o = msgp.AppendUint64(o, z.Hits)
	// string "Errors"
	o = append(o, 0xa6, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x73)
	o = msgp.AppendUint64(o, z.Errors)
	// string "Duration"
	o = append(o, 0xa8, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	o = msgp.AppendUint64(o, z.Duration)
	// string "OkSummary"
	o = append(o, 0xa9, 0x4f, 0x6b, 0x53, 0x75, 0x6d, 0x6d, 0x61, 0x72, 0x79)
	o = msgp.AppendBytes(o, z.OkSummary)
	// string "ErrorSummary"
	o = append(o, 0xac, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x53, 0x75, 0x6d, 0x6d, 0x61, 0x72, 0x79)
	o = msgp.AppendBytes(o, z.ErrorSummary)
	// string "Synthetics"
	o = append(o, 0xaa, 0x53, 0x79, 0x6e, 0x74, 0x68, 0x65, 0x74, 0x69, 0x63, 0x73)
	o = msgp.AppendBool(o, z.Synthetics)
	// string "TopLevelHits"
	o = append(o, 0xac, 0x54, 0x6f, 0x70, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x48, 0x69, 0x74, 0x73)
	o = msgp.AppendUint64(o, z.TopLevelHits)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ClientGroupedStats) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Service":
			z.Service, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Name":
			z.Name, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Resource":
			z.Resource, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "HTTPStatusCode":
			z.HTTPStatusCode, bts, err = msgp.ReadUint32Bytes(bts)
			if err != nil {
				return
			}
		case "Type":
			z.Type, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "DBType":
			z.DBType, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Hits":
			z.Hits, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "Errors":
			z.Errors, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "Duration":
			z.Duration, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "OkSummary":
			z.OkSummary, bts, err = msgp.ReadBytesBytes(bts, z.OkSummary)
			if err != nil {
				return
			}
		case "ErrorSummary":
			z.ErrorSummary, bts, err = msgp.ReadBytesBytes(bts, z.ErrorSummary)
			if err != nil {
				return
			}
		case "Synthetics":
			z.Synthetics, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				return
			}
		case "TopLevelHits":
			z.TopLevelHits, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ClientGroupedStats) Msgsize() (s int) {
	s = 1 + 8 + msgp.StringPrefixSize + len(z.Service) + 5 + msgp.StringPrefixSize + len(z.Name) + 9 + msgp.StringPrefixSize + len(z.Resource) + 15 + msgp.Uint32Size + 5 + msgp.StringPrefixSize + len(z.Type) + 7 + msgp.StringPrefixSize + len(z.DBType) + 5 + msgp.Uint64Size + 7 + msgp.Uint64Size + 9 + msgp.Uint64Size + 10 + msgp.BytesPrefixSize + len(z.OkSummary) + 13 + msgp.BytesPrefixSize + len(z.ErrorSummary) + 11 + msgp.BoolSize + 13 + msgp.Uint64Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ClientStatsBucket) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Start":
			z.Start, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "Duration":
			z.Duration, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "Stats":
			var zb0002 uint32
			zb0002, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Stats) >= int(zb0002) {
				z.Stats = (z.Stats)[:zb0002]
			} else {
				z.Stats = make([]ClientGroupedStats, zb0002)
			}
			for za0001 := range z.Stats {
				err = z.Stats[za0001].DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		case "AgentTimeShift":
			z.AgentTimeShift, err = dc.ReadInt64()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *ClientStatsBucket) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "Start"
	err = en.Append(0x84, 0xa5, 0x53, 0x74, 0x61, 0x72, 0x74)
	if err != nil {
		return
	}
	err = en.WriteUint64(z.Start)
	if err != nil {
		return
	}
	// write "Duration"
	err = en.Append(0xa8, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteUint64(z.Duration)
	if err != nil {
		return
	}
	// write "Stats"
	err = en.Append(0xa5, 0x53, 0x74, 0x61, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Stats)))
	if err != nil {
		return
	}
	for za0001 := range z.Stats {
		err = z.Stats[za0001].EncodeMsg(en)
		if err != nil {
			return
		}
	}
	// write "AgentTimeShift"
	err = en.Append(0xae, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x68, 0x69, 0x66, 0x74)
	if err != nil {
		return
	}
	err = en.WriteInt64(z.AgentTimeShift)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ClientStatsBucket) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "Start"
	o = append(o, 0x84, 0xa5, 0x53, 0x74, 0x61, 0x72, 0x74)
	o = msgp.AppendUint64(o, z.Start)
	// string "Duration"
	o = append(o, 0xa8, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	o = msgp.AppendUint64(o, z.Duration)
	// string "Stats"
	o = append(o, 0xa5, 0x53, 0x74, 0x61, 0x74, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Stats)))
	for za0001 := range z.Stats {
		o, err = z.Stats[za0001].MarshalMsg(o)
		if err != nil {
			return
		}
	}
	// string "AgentTimeShift"
	o = append(o, 0xae, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x68, 0x69, 0x66, 0x74)
	o = msgp.AppendInt64(o, z.AgentTimeShift)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ClientStatsBucket) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Start":
			z.Start, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "Duration":
			z.Duration, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "Stats":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Stats) >= int(zb0002) {
				z.Stats = (z.Stats)[:zb0002]
			} else {
				z.Stats = make([]ClientGroupedStats, zb0002)
			}
			for za0001 := range z.Stats {
				bts, err = z.Stats[za0001].UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		case "AgentTimeShift":
			z.AgentTimeShift, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ClientStatsBucket) Msgsize() (s int) {
	s = 1 + 6 + msgp.Uint64Size + 9 + msgp.Uint64Size + 6 + msgp.ArrayHeaderSize
	for za0001 := range z.Stats {
		s += z.Stats[za0001].Msgsize()
	}
	s += 15 + msgp.Int64Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ClientStatsPayload) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Hostname":
			z.Hostname, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Env":
			z.Env, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Version":
			z.Version, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Stats":
			var zb0002 uint32
			zb0002, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Stats) >= int(zb0002) {
				z.Stats = (z.Stats)[:zb0002]
			} else {
				z.Stats = make([]ClientStatsBucket, zb0002)
			}
			for za0001 := range z.Stats {
				err = z.Stats[za0001].DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		case "Lang":
			z.Lang, err = dc.ReadString()
			if err != nil {
				return
			}
		case "TracerVersion":
			z.TracerVersion, err = dc.ReadString()
			if err != nil {
				return
			}
		case "RuntimeID":
			z.RuntimeID, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Sequence":
			z.Sequence, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "AgentAggregation":
			z.AgentAggregation, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Service":
			z.Service, err = dc.ReadString()
			if err != nil {
				return
			}
		case "ContainerID":
			z.ContainerID, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Tags":
			var zb0003 uint32
			zb0003, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Tags) >= int(zb0003) {
				z.Tags = (z.Tags)[:zb0003]
			} else {
				z.Tags = make([]string, zb0003)
			}
			for za0002 := range z.Tags {
				z.Tags[za0002], err = dc.ReadString()
				if err != nil {
					return
				}
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *ClientStatsPayload) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 12
	// write "Hostname"
	err = en.Append(0x8c, 0xa8, 0x48, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Hostname)
	if err != nil {
		return
	}
	// write "Env"
	err = en.Append(0xa3, 0x45, 0x6e, 0x76)
	if err != nil {
		return
	}
	err = en.WriteString(z.Env)
	if err != nil {
		return
	}
	// write "Version"
	err = en.Append(0xa7, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteString(z.Version)
	if err != nil {
		return
	}
	// write "Stats"
	err = en.Append(0xa5, 0x53, 0x74, 0x61, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Stats)))
	if err != nil {
		return
	}
	for za0001 := range z.Stats {
		err = z.Stats[za0001].EncodeMsg(en)
		if err != nil {
			return
		}
	}
	// write "Lang"
	err = en.Append(0xa4, 0x4c, 0x61, 0x6e, 0x67)
	if err != nil {
		return
	}
	err = en.WriteString(z.Lang)
	if err != nil {
		return
	}
	// write "TracerVersion"
	err = en.Append(0xad, 0x54, 0x72, 0x61, 0x63, 0x65, 0x72, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteString(z.TracerVersion)
	if err != nil {
		return
	}
	// write "RuntimeID"
	err = en.Append(0xa9, 0x52, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x49, 0x44)
	if err != nil {
		return
	}
	err = en.WriteString(z.RuntimeID)
	if err != nil {
		return
	}
	// write "Sequence"
	err = en.Append(0xa8, 0x53, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65)
	if err != nil {
		return
	}
	err = en.WriteUint64(z.Sequence)
	if err != nil {
		return
	}
	// write "AgentAggregation"
	err = en.Append(0xb0, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x41, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteString(z.AgentAggregation)
	if err != nil {
		return
	}
	// write "Service"
	err = en.Append(0xa7, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Service)
	if err != nil {
		return
	}
	// write "ContainerID"
	err = en.Append(0xab, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x49, 0x44)
	if err != nil {
		return
	}
	err = en.WriteString(z.ContainerID)
	if err != nil {
		return
	}
	// write "Tags"
	err = en.Append(0xa4, 0x54, 0x61, 0x67, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Tags)))
	if err != nil {
		return
	}
	for za0002 := range z.Tags {
		err = en.WriteString(z.Tags[za0002])
		if err != nil {
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *ClientStatsPayload) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 12
	// string "Hostname"
	o = append(o, 0x8c, 0xa8, 0x48, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Hostname)
	// string "Env"
	o = append(o, 0xa3, 0x45, 0x6e, 0x76)
	o = msgp.AppendString(o, z.Env)
	// string "Version"
	o = append(o, 0xa7, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	o = msgp.AppendString(o, z.Version)
	// string "Stats"
	o = append(o, 0xa5, 0x53, 0x74, 0x61, 0x74, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Stats)))
	for za0001 := range z.Stats {
		o, err = z.Stats[za0001].MarshalMsg(o)
		if err != nil {
			return
		}
	}
	// string "Lang"
	o = append(o, 0xa4, 0x4c, 0x61, 0x6e, 0x67)
	o = msgp.AppendString(o, z.Lang)
	// string "TracerVersion"
	o = append(o, 0xad, 0x54, 0x72, 0x61, 0x63, 0x65, 0x72, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	o = msgp.AppendString(o, z.TracerVersion)
	// string "RuntimeID"
	o = append(o, 0xa9, 0x52, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x49, 0x44)
	o = msgp.AppendString(o, z.RuntimeID)
	// string "Sequence"
	o = append(o, 0xa8, 0x53, 0x65, 0x71, 0x75, 0x65, 0x6e, 0x63, 0x65)
	o = msgp.AppendUint64(o, z.Sequence)
	// string "AgentAggregation"
	o = append(o, 0xb0, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x41, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	o = msgp.AppendString(o, z.AgentAggregation)
	// string "Service"
	o = append(o, 0xa7, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65)
	o = msgp.AppendString(o, z.Service)
	// string "ContainerID"
	o = append(o, 0xab, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x49, 0x44)
	o = msgp.AppendString(o, z.ContainerID)
	// string "Tags"
	o = append(o, 0xa4, 0x54, 0x61, 0x67, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Tags)))
	for za0002 := range z.Tags {
		o = msgp.AppendString(o, z.Tags[za0002])
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ClientStatsPayload) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Hostname":
			z.Hostname, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Env":
			z.Env, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Version":
			z.Version, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Stats":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Stats) >= int(zb0002) {
				z.Stats = (z.Stats)[:zb0002]
			} else {
				z.Stats = make([]ClientStatsBucket, zb0002)
			}
			for za0001 := range z.Stats {
				bts, err = z.Stats[za0001].UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		case "Lang":
			z.Lang, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "TracerVersion":
			z.TracerVersion, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "RuntimeID":
			z.RuntimeID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Sequence":
			z.Sequence, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "AgentAggregation":
			z.AgentAggregation, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Service":
			z.Service, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "ContainerID":
			z.ContainerID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Tags":
			var zb0003 uint32
			zb0003, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Tags) >= int(zb0003) {
				z.Tags = (z.Tags)[:zb0003]
			} else {
				z.Tags = make([]string, zb0003)
			}
			for za0002 := range z.Tags {
				z.Tags[za0002], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *ClientStatsPayload) Msgsize() (s int) {
	s = 1 + 9 + msgp.StringPrefixSize + len(z.Hostname) + 4 + msgp.StringPrefixSize + len(z.Env) + 8 + msgp.StringPrefixSize + len(z.Version) + 6 + msgp.ArrayHeaderSize
	for za0001 := range z.Stats {
		s += z.Stats[za0001].Msgsize()
	}
	s += 5 + msgp.StringPrefixSize + len(z.Lang) + 14 + msgp.StringPrefixSize + len(z.TracerVersion) + 10 + msgp.StringPrefixSize + len(z.RuntimeID) + 9 + msgp.Uint64Size + 17 + msgp.StringPrefixSize + len(z.AgentAggregation) + 8 + msgp.StringPrefixSize + len(z.Service) + 12 + msgp.StringPrefixSize + len(z.ContainerID) + 5 + msgp.ArrayHeaderSize
	for za0002 := range z.Tags {
		s += msgp.StringPrefixSize + len(z.Tags[za0002])
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *StatsPayload) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "AgentHostname":
			z.AgentHostname, err = dc.ReadString()
			if err != nil {
				return
			}
		case "AgentEnv":
			z.AgentEnv, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Stats":
			var zb0002 uint32
			zb0002, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Stats) >= int(zb0002) {
				z.Stats = (z.Stats)[:zb0002]
			} else {
				z.Stats = make([]ClientStatsPayload, zb0002)
			}
			for za0001 := range z.Stats {
				err = z.Stats[za0001].DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		case "AgentVersion":
			z.AgentVersion, err = dc.ReadString()
			if err != nil {
				return
			}
		case "ClientComputed":
			z.ClientComputed, err = dc.ReadBool()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *StatsPayload) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 5
	// write "AgentHostname"
	err = en.Append(0x85, 0xad, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x48, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.AgentHostname)
	if err != nil {
		return
	}
	// write "AgentEnv"
	err = en.Append(0xa8, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x45, 0x6e, 0x76)
	if err != nil {
		return
	}
	err = en.WriteString(z.AgentEnv)
	if err != nil {
		return
	}
	// write "Stats"
	err = en.Append(0xa5, 0x53, 0x74, 0x61, 0x74, 0x73)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Stats)))
	if err != nil {
		return
	}
	for za0001 := range z.Stats {
		err = z.Stats[za0001].EncodeMsg(en)
		if err != nil {
			return
		}
	}
	// write "AgentVersion"
	err = en.Append(0xac, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	if err != nil {
		return
	}
	err = en.WriteString(z.AgentVersion)
	if err != nil {
		return
	}
	// write "ClientComputed"
	err = en.Append(0xae, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x64)
	if err != nil {
		return
	}
	err = en.WriteBool(z.ClientComputed)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *StatsPayload) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 5
	// string "AgentHostname"
	o = append(o, 0x85, 0xad, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x48, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.AgentHostname)
	// string "AgentEnv"
	o = append(o, 0xa8, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x45, 0x6e, 0x76)
	o = msgp.AppendString(o, z.AgentEnv)
	// string "Stats"
	o = append(o, 0xa5, 0x53, 0x74, 0x61, 0x74, 0x73)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Stats)))
	for za0001 := range z.Stats {
		o, err = z.Stats[za0001].MarshalMsg(o)
		if err != nil {
			return
		}
	}
	// string "AgentVersion"
	o = append(o, 0xac, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e)
	o = msgp.AppendString(o, z.AgentVersion)
	// string "ClientComputed"
	o = append(o, 0xae, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x64)
	o = msgp.AppendBool(o, z.ClientComputed)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *StatsPayload) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "AgentHostname":
			z.AgentHostname, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "AgentEnv":
			z.AgentEnv, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Stats":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Stats) >= int(zb0002) {
				z.Stats = (z.Stats)[:zb0002]
			} else {
				z.Stats = make([]ClientStatsPayload, zb0002)
			}
			for za0001 := range z.Stats {
				bts, err = z.Stats[za0001].UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		case "AgentVersion":
			z.AgentVersion, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "ClientComputed":
			z.ClientComputed, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *StatsPayload) Msgsize() (s int) {
	s = 1 + 14 + msgp.StringPrefixSize + len(z.AgentHostname) + 9 + msgp.StringPrefixSize + len(z.AgentEnv) + 6 + msgp.ArrayHeaderSize
	for za0001 := range z.Stats {
		s += z.Stats[za0001].Msgsize()
	}
	s += 13 + msgp.StringPrefixSize + len(z.AgentVersion) + 15 + msgp.BoolSize
	return
}
