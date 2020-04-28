package bsonx

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsonrw/bsonrwtest"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestDefaultValueEncoders(t *testing.T) {
	var pcx PrimitiveCodecs

	var wrong = func(string, string) string { return "wrong" }

	type subtest struct {
		name   string
		val    interface{}
		ectx   *bsoncodec.EncodeContext
		llvrw  *bsonrwtest.ValueReaderWriter
		invoke bsonrwtest.Invoked
		err    error
	}

	testCases := []struct {
		name     string
		ve       bsoncodec.ValueEncoder
		subtests []subtest
	}{
		{
			"ValueEncodeValue",
			bsoncodec.ValueEncoderFunc(pcx.ValueEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					bsoncodec.ValueEncoderError{Name: "ValueEncodeValue", Types: []reflect.Type{tValue}, Received: reflect.ValueOf(wrong)},
				},
				{"empty value", Val{}, nil, nil, bsonrwtest.WriteNull, nil},
				{
					"success",
					Null(),
					&bsoncodec.EncodeContext{Registry: DefaultRegistry},
					&bsonrwtest.ValueReaderWriter{},
					bsonrwtest.WriteNull,
					nil,
				},
			},
		},
		{
			"ElementSliceEncodeValue",
			bsoncodec.ValueEncoderFunc(pcx.ElementSliceEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					bsoncodec.ValueEncoderError{
						Name:     "ElementSliceEncodeValue",
						Types:    []reflect.Type{tElementSlice},
						Received: reflect.ValueOf(wrong),
					},
				},
			},
		},
		{
			"ArrayEncodeValue",
			bsoncodec.ValueEncoderFunc(pcx.ArrayEncodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					bsoncodec.ValueEncoderError{Name: "ArrayEncodeValue", Types: []reflect.Type{tArray}, Received: reflect.ValueOf(wrong)},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, subtest := range tc.subtests {
				t.Run(subtest.name, func(t *testing.T) {
					var ec bsoncodec.EncodeContext
					if subtest.ectx != nil {
						ec = *subtest.ectx
					}
					llvrw := new(bsonrwtest.ValueReaderWriter)
					if subtest.llvrw != nil {
						llvrw = subtest.llvrw
					}
					llvrw.T = t
					err := tc.ve.EncodeValue(ec, llvrw, reflect.ValueOf(subtest.val))
					if !compareErrors(err, subtest.err) {
						t.Errorf("Errors do not match. got %v; want %v", err, subtest.err)
					}
					invoked := llvrw.Invoked
					if !cmp.Equal(invoked, subtest.invoke) {
						t.Errorf("Incorrect method invoked. got %v; want %v", invoked, subtest.invoke)
					}
				})
			}
		})
	}

	t.Run("DocumentEncodeValue", func(t *testing.T) {
		t.Run("ValueEncoderError", func(t *testing.T) {
			val := reflect.ValueOf(bool(true))
			want := bsoncodec.ValueEncoderError{Name: "DocumentEncodeValue", Types: []reflect.Type{tDocument}, Received: val}
			got := (PrimitiveCodecs{}).DocumentEncodeValue(bsoncodec.EncodeContext{}, nil, val)
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("WriteDocument Error", func(t *testing.T) {
			want := errors.New("WriteDocument Error")
			llvrw := &bsonrwtest.ValueReaderWriter{
				T:        t,
				Err:      want,
				ErrAfter: bsonrwtest.WriteDocument,
			}
			got := (PrimitiveCodecs{}).DocumentEncodeValue(bsoncodec.EncodeContext{}, llvrw, reflect.MakeSlice(tDocument, 0, 0))
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("encodeDocument errors", func(t *testing.T) {
			ec := bsoncodec.EncodeContext{}
			err := errors.New("encodeDocument error")
			oid := primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
			testCases := []struct {
				name  string
				ec    bsoncodec.EncodeContext
				llvrw *bsonrwtest.ValueReaderWriter
				doc   Doc
				err   error
			}{
				{
					"WriteDocumentElement",
					ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: errors.New("wde error"), ErrAfter: bsonrwtest.WriteDocumentElement},
					Doc{{"foo", Null()}},
					errors.New("wde error"),
				},
				{
					"WriteDouble", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteDouble},
					Doc{{"foo", Double(3.14159)}}, err,
				},
				{
					"WriteString", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteString},
					Doc{{"foo", String("bar")}}, err,
				},
				{
					"WriteDocument (Lookup)", bsoncodec.EncodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{T: t},
					Doc{{"foo", Document(Doc{{"bar", Null()}})}},
					bsoncodec.ErrNoEncoder{Type: tDocument},
				},
				{
					"WriteArray (Lookup)", bsoncodec.EncodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{T: t},
					Doc{{"foo", Array(Arr{Null()})}},
					bsoncodec.ErrNoEncoder{Type: tArray},
				},
				{
					"WriteBinary", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteBinaryWithSubtype},
					Doc{{"foo", Binary(0xFF, []byte{0x01, 0x02, 0x03})}}, err,
				},
				{
					"WriteUndefined", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteUndefined},
					Doc{{"foo", Undefined()}}, err,
				},
				{
					"WriteObjectID", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteObjectID},
					Doc{{"foo", ObjectID(oid)}}, err,
				},
				{
					"WriteBoolean", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteBoolean},
					Doc{{"foo", Boolean(true)}}, err,
				},
				{
					"WriteDateTime", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteDateTime},
					Doc{{"foo", DateTime(1234567890)}}, err,
				},
				{
					"WriteNull", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteNull},
					Doc{{"foo", Null()}}, err,
				},
				{
					"WriteRegex", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteRegex},
					Doc{{"foo", Regex("bar", "baz")}}, err,
				},
				{
					"WriteDBPointer", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteDBPointer},
					Doc{{"foo", DBPointer("bar", oid)}}, err,
				},
				{
					"WriteJavascript", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteJavascript},
					Doc{{"foo", JavaScript("var hello = 'world';")}}, err,
				},
				{
					"WriteSymbol", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteSymbol},
					Doc{{"foo", Symbol("symbolbaz")}}, err,
				},
				{
					"WriteCodeWithScope (Lookup)", bsoncodec.EncodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteCodeWithScope},
					Doc{{"foo", CodeWithScope("var hello = 'world';", Doc{}.Append("bar", Null()))}},
					err,
				},
				{
					"WriteInt32", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteInt32},
					Doc{{"foo", Int32(12345)}}, err,
				},
				{
					"WriteInt64", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteInt64},
					Doc{{"foo", Int64(1234567890)}}, err,
				},
				{
					"WriteTimestamp", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteTimestamp},
					Doc{{"foo", Timestamp(10, 20)}}, err,
				},
				{
					"WriteDecimal128", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteDecimal128},
					Doc{{"foo", Decimal128(primitive.NewDecimal128(10, 20))}}, err,
				},
				{
					"WriteMinKey", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteMinKey},
					Doc{{"foo", MinKey()}}, err,
				},
				{
					"WriteMaxKey", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteMaxKey},
					Doc{{"foo", MaxKey()}}, err,
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					err := (PrimitiveCodecs{}).DocumentEncodeValue(tc.ec, tc.llvrw, reflect.ValueOf(tc.doc))
					if !compareErrors(err, tc.err) {
						t.Errorf("Errors do not match. got %v; want %v", err, tc.err)
					}
				})
			}
		})

		t.Run("success", func(t *testing.T) {
			oid := primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
			d128 := primitive.NewDecimal128(10, 20)
			want := Doc{
				{"a", Double(3.14159)}, {"b", String("foo")},
				{"c", Document(Doc{{"aa", Null()}})}, {"d", Array(Arr{Null()})},
				{"e", Binary(0xFF, []byte{0x01, 0x02, 0x03})}, {"f", Undefined()},
				{"g", ObjectID(oid)}, {"h", Boolean(true)},
				{"i", DateTime(1234567890)}, {"j", Null()},
				{"k", Regex("foo", "abr")},
				{"l", DBPointer("foobar", oid)}, {"m", JavaScript("var hello = 'world';")},
				{"n", Symbol("bazqux")},
				{"o", CodeWithScope("var hello = 'world';", Doc{{"ab", Null()}})},
				{"p", Int32(12345)},
				{"q", Timestamp(10, 20)}, {"r", Int64(1234567890)}, {"s", Decimal128(d128)}, {"t", MinKey()}, {"u", MaxKey()},
			}
			got := Doc{}
			slc := make(bsonrw.SliceWriter, 0, 128)
			vw, err := bsonrw.NewBSONValueWriter(&slc)
			noerr(t, err)

			ec := bsoncodec.EncodeContext{Registry: DefaultRegistry}
			err = (PrimitiveCodecs{}).DocumentEncodeValue(ec, vw, reflect.ValueOf(want))
			noerr(t, err)
			got, err = ReadDoc(slc)
			noerr(t, err)
			if !got.Equal(want) {
				t.Error("Documents do not match")
				t.Errorf("\ngot :%v\nwant:%v", got, want)
			}
		})
	})

	t.Run("ArrayEncodeValue", func(t *testing.T) {
		t.Run("CodecEncodeError", func(t *testing.T) {
			val := reflect.ValueOf(bool(true))
			want := bsoncodec.ValueEncoderError{Name: "ArrayEncodeValue", Types: []reflect.Type{tArray}, Received: val}
			got := (PrimitiveCodecs{}).ArrayEncodeValue(bsoncodec.EncodeContext{}, nil, val)
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("WriteArray Error", func(t *testing.T) {
			want := errors.New("WriteArray Error")
			llvrw := &bsonrwtest.ValueReaderWriter{
				T:        t,
				Err:      want,
				ErrAfter: bsonrwtest.WriteArray,
			}
			got := (PrimitiveCodecs{}).ArrayEncodeValue(bsoncodec.EncodeContext{}, llvrw, reflect.MakeSlice(tArray, 0, 0))
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("encode array errors", func(t *testing.T) {
			ec := bsoncodec.EncodeContext{}
			err := errors.New("encode array error")
			oid := primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
			testCases := []struct {
				name  string
				ec    bsoncodec.EncodeContext
				llvrw *bsonrwtest.ValueReaderWriter
				arr   Arr
				err   error
			}{
				{
					"WriteDocumentElement",
					ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: errors.New("wde error"), ErrAfter: bsonrwtest.WriteArrayElement},
					Arr{Null()},
					errors.New("wde error"),
				},
				{
					"WriteDouble", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteDouble},
					Arr{Double(3.14159)}, err,
				},
				{
					"WriteString", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteString},
					Arr{String("bar")}, err,
				},
				{
					"WriteDocument (Lookup)", bsoncodec.EncodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{T: t},
					Arr{Document(Doc{{"bar", Null()}})},
					bsoncodec.ErrNoEncoder{Type: tDocument},
				},
				{
					"WriteArray (Lookup)", bsoncodec.EncodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{T: t},
					Arr{Array(Arr{Null()})},
					bsoncodec.ErrNoEncoder{Type: tArray},
				},
				{
					"WriteBinary", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteBinaryWithSubtype},
					Arr{Binary(0xFF, []byte{0x01, 0x02, 0x03})}, err,
				},
				{
					"WriteUndefined", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteUndefined},
					Arr{Undefined()}, err,
				},
				{
					"WriteObjectID", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteObjectID},
					Arr{ObjectID(oid)}, err,
				},
				{
					"WriteBoolean", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteBoolean},
					Arr{Boolean(true)}, err,
				},
				{
					"WriteDateTime", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteDateTime},
					Arr{DateTime(1234567890)}, err,
				},
				{
					"WriteNull", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteNull},
					Arr{Null()}, err,
				},
				{
					"WriteRegex", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteRegex},
					Arr{Regex("bar", "baz")}, err,
				},
				{
					"WriteDBPointer", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteDBPointer},
					Arr{DBPointer("bar", oid)}, err,
				},
				{
					"WriteJavascript", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteJavascript},
					Arr{JavaScript("var hello = 'world';")}, err,
				},
				{
					"WriteSymbol", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteSymbol},
					Arr{Symbol("symbolbaz")}, err,
				},
				{
					"WriteCodeWithScope (Lookup)", bsoncodec.EncodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteCodeWithScope},
					Arr{CodeWithScope("var hello = 'world';", Doc{{"bar", Null()}})},
					err,
				},
				{
					"WriteInt32", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteInt32},
					Arr{Int32(12345)}, err,
				},
				{
					"WriteInt64", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteInt64},
					Arr{Int64(1234567890)}, err,
				},
				{
					"WriteTimestamp", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteTimestamp},
					Arr{Timestamp(10, 20)}, err,
				},
				{
					"WriteDecimal128", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteDecimal128},
					Arr{Decimal128(primitive.NewDecimal128(10, 20))}, err,
				},
				{
					"WriteMinKey", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteMinKey},
					Arr{MinKey()}, err,
				},
				{
					"WriteMaxKey", ec,
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.WriteMaxKey},
					Arr{MaxKey()}, err,
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					err := (PrimitiveCodecs{}).ArrayEncodeValue(tc.ec, tc.llvrw, reflect.ValueOf(tc.arr))
					if !compareErrors(err, tc.err) {
						t.Errorf("Errors do not match. got %v; want %v", err, tc.err)
					}
				})
			}
		})

		t.Run("success", func(t *testing.T) {
			oid := primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
			d128 := primitive.NewDecimal128(10, 20)
			want := Arr{
				Double(3.14159), String("foo"), Document(Doc{{"aa", Null()}}),
				Array(Arr{Null()}),
				Binary(0xFF, []byte{0x01, 0x02, 0x03}), Undefined(),
				ObjectID(oid), Boolean(true), DateTime(1234567890), Null(), Regex("foo", "abr"),
				DBPointer("foobar", oid), JavaScript("var hello = 'world';"), Symbol("bazqux"),
				CodeWithScope("var hello = 'world';", Doc{{"ab", Null()}}), Int32(12345),
				Timestamp(10, 20), Int64(1234567890), Decimal128(d128), MinKey(), MaxKey(),
			}

			ec := bsoncodec.EncodeContext{Registry: DefaultRegistry}

			slc := make(bsonrw.SliceWriter, 0, 128)
			vw, err := bsonrw.NewBSONValueWriter(&slc)
			noerr(t, err)

			dr, err := vw.WriteDocument()
			noerr(t, err)
			vr, err := dr.WriteDocumentElement("foo")
			noerr(t, err)

			err = (PrimitiveCodecs{}).ArrayEncodeValue(ec, vr, reflect.ValueOf(want))
			noerr(t, err)

			err = dr.WriteDocumentEnd()
			noerr(t, err)

			val, err := bsoncore.Document(slc).LookupErr("foo")
			noerr(t, err)
			rgot := val.Array()
			doc, err := ReadDoc(rgot)
			noerr(t, err)
			got := make(Arr, 0)
			for _, elem := range doc {
				got = append(got, elem.Value)
			}
			if !got.Equal(want) {
				t.Error("Documents do not match")
				t.Errorf("\ngot :%v\nwant:%v", got, want)
			}
		})
	})
}

func TestDefaultValueDecoders(t *testing.T) {
	var pcx PrimitiveCodecs

	var wrong = func(string, string) string { return "wrong" }

	const cansetreflectiontest = "cansetreflectiontest"

	type subtest struct {
		name   string
		val    interface{}
		dctx   *bsoncodec.DecodeContext
		llvrw  *bsonrwtest.ValueReaderWriter
		invoke bsonrwtest.Invoked
		err    error
	}

	testCases := []struct {
		name     string
		vd       bsoncodec.ValueDecoder
		subtests []subtest
	}{
		{
			"ValueDecodeValue",
			bsoncodec.ValueDecoderFunc(pcx.ValueDecodeValue),
			[]subtest{
				{
					"wrong type",
					wrong,
					nil,
					nil,
					bsonrwtest.Nothing,
					bsoncodec.ValueDecoderError{
						Name:     "ValueDecodeValue",
						Types:    []reflect.Type{tValue},
						Received: reflect.ValueOf(wrong),
					},
				},
				{
					"invalid value",
					(*Val)(nil),
					nil,
					nil,
					bsonrwtest.Nothing,
					bsoncodec.ValueDecoderError{
						Name:     "ValueDecodeValue",
						Types:    []reflect.Type{tValue},
						Received: reflect.ValueOf((*Val)(nil)),
					},
				},
				{
					"success",
					Double(3.14159),
					&bsoncodec.DecodeContext{Registry: NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.Double, Return: float64(3.14159)},
					bsonrwtest.ReadDouble,
					nil,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, rc := range tc.subtests {
				t.Run(rc.name, func(t *testing.T) {
					var dc bsoncodec.DecodeContext
					if rc.dctx != nil {
						dc = *rc.dctx
					}
					llvrw := new(bsonrwtest.ValueReaderWriter)
					if rc.llvrw != nil {
						llvrw = rc.llvrw
					}
					llvrw.T = t
					// var got interface{}
					if rc.val == cansetreflectiontest { // We're doing a CanSet reflection test
						err := tc.vd.DecodeValue(dc, llvrw, reflect.Value{})
						if !compareErrors(err, rc.err) {
							t.Errorf("Errors do not match. got %v; want %v", err, rc.err)
						}

						val := reflect.New(reflect.TypeOf(rc.val)).Elem()
						err = tc.vd.DecodeValue(dc, llvrw, val)
						if !compareErrors(err, rc.err) {
							t.Errorf("Errors do not match. got %v; want %v", err, rc.err)
						}
						return
					}
					var val reflect.Value
					if rtype := reflect.TypeOf(rc.val); rtype != nil {
						val = reflect.New(rtype).Elem()
					}
					want := rc.val
					defer func() {
						if err := recover(); err != nil {
							fmt.Println(t.Name())
							panic(err)
						}
					}()
					err := tc.vd.DecodeValue(dc, llvrw, val)
					if !compareErrors(err, rc.err) {
						t.Errorf("Errors do not match. got %v; want %v", err, rc.err)
					}
					invoked := llvrw.Invoked
					if !cmp.Equal(invoked, rc.invoke) {
						t.Errorf("Incorrect method invoked. got %v; want %v", invoked, rc.invoke)
					}
					var got interface{}
					if val.IsValid() && val.CanInterface() {
						got = val.Interface()
					}
					if rc.err == nil && !cmp.Equal(got, want, cmp.Comparer(compareValues)) {
						t.Errorf("Values do not match. got (%T)%v; want (%T)%v", got, got, want, want)
					}
				})
			}
		})
	}

	t.Run("DocumentDecodeValue", func(t *testing.T) {
		t.Run("CodecDecodeError", func(t *testing.T) {
			val := reflect.New(reflect.TypeOf(false)).Elem()
			want := bsoncodec.ValueDecoderError{Name: "DocumentDecodeValue", Types: []reflect.Type{tDocument}, Received: val}
			got := pcx.DocumentDecodeValue(bsoncodec.DecodeContext{}, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.EmbeddedDocument}, val)
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("ReadDocument Error", func(t *testing.T) {
			want := errors.New("ReadDocument Error")
			llvrw := &bsonrwtest.ValueReaderWriter{
				T:        t,
				Err:      want,
				ErrAfter: bsonrwtest.ReadDocument,
				BSONType: bsontype.EmbeddedDocument,
			}
			got := pcx.DocumentDecodeValue(bsoncodec.DecodeContext{}, llvrw, reflect.New(reflect.TypeOf(Doc{})).Elem())
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("decodeDocument errors", func(t *testing.T) {
			dc := bsoncodec.DecodeContext{}
			err := errors.New("decodeDocument error")
			testCases := []struct {
				name  string
				dc    bsoncodec.DecodeContext
				llvrw *bsonrwtest.ValueReaderWriter
				err   error
			}{
				{
					"ReadElement",
					dc,
					&bsonrwtest.ValueReaderWriter{T: t, Err: errors.New("re error"), ErrAfter: bsonrwtest.ReadElement},
					errors.New("re error"),
				},
				{"ReadDouble", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadDouble, BSONType: bsontype.Double}, err},
				{"ReadString", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadString, BSONType: bsontype.String}, err},
				{"ReadBinary", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadBinary, BSONType: bsontype.Binary}, err},
				{"ReadUndefined", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadUndefined, BSONType: bsontype.Undefined}, err},
				{"ReadObjectID", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadObjectID, BSONType: bsontype.ObjectID}, err},
				{"ReadBoolean", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadBoolean, BSONType: bsontype.Boolean}, err},
				{"ReadDateTime", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadDateTime, BSONType: bsontype.DateTime}, err},
				{"ReadNull", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadNull, BSONType: bsontype.Null}, err},
				{"ReadRegex", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadRegex, BSONType: bsontype.Regex}, err},
				{"ReadDBPointer", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadDBPointer, BSONType: bsontype.DBPointer}, err},
				{"ReadJavascript", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadJavascript, BSONType: bsontype.JavaScript}, err},
				{"ReadSymbol", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadSymbol, BSONType: bsontype.Symbol}, err},
				{
					"ReadCodeWithScope (Lookup)", bsoncodec.DecodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadCodeWithScope, BSONType: bsontype.CodeWithScope},
					err,
				},
				{"ReadInt32", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadInt32, BSONType: bsontype.Int32}, err},
				{"ReadInt64", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadInt64, BSONType: bsontype.Int64}, err},
				{"ReadTimestamp", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadTimestamp, BSONType: bsontype.Timestamp}, err},
				{"ReadDecimal128", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadDecimal128, BSONType: bsontype.Decimal128}, err},
				{"ReadMinKey", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadMinKey, BSONType: bsontype.MinKey}, err},
				{"ReadMaxKey", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadMaxKey, BSONType: bsontype.MaxKey}, err},
				{"Invalid Type", dc, &bsonrwtest.ValueReaderWriter{T: t, BSONType: bsontype.Type(0)}, fmt.Errorf("Cannot read unknown BSON type %s", bsontype.Type(0))},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					err := pcx.DecodeDocument(tc.dc, tc.llvrw, new(Doc))
					if !compareErrors(err, tc.err) {
						t.Errorf("Errors do not match. got %v; want %v", err, tc.err)
					}
				})
			}
		})

		t.Run("success", func(t *testing.T) {
			oid := primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
			d128 := primitive.NewDecimal128(10, 20)
			want := Doc{
				{"a", Double(3.14159)}, {"b", String("foo")},
				{"c", Document(Doc{{"aa", Null()}})},
				{"d", Array(Arr{Null()})},
				{"e", Binary(0xFF, []byte{0x01, 0x02, 0x03})}, {"f", Undefined()},
				{"g", ObjectID(oid)}, {"h", Boolean(true)},
				{"i", DateTime(1234567890)}, {"j", Null()}, {"k", Regex("foo", "bar")},
				{"l", DBPointer("foobar", oid)}, {"m", JavaScript("var hello = 'world';")},
				{"n", Symbol("bazqux")},
				{"o", CodeWithScope("var hello = 'world';", Doc{{"ab", Null()}})},
				{"p", Int32(12345)},
				{"q", Timestamp(10, 20)}, {"r", Int64(1234567890)},
				{"s", Decimal128(d128)}, {"t", MinKey()}, {"u", MaxKey()},
			}
			got := reflect.New(reflect.TypeOf(Doc{})).Elem()
			dc := bsoncodec.DecodeContext{Registry: NewRegistryBuilder().Build()}
			b, err := want.MarshalBSON()
			noerr(t, err)
			err = pcx.DocumentDecodeValue(dc, bsonrw.NewBSONDocumentReader(b), got)
			noerr(t, err)
			if !got.Interface().(Doc).Equal(want) {
				t.Error("Documents do not match")
				t.Errorf("\ngot :%v\nwant:%v", got, want)
			}
		})
	})

	t.Run("ArrayDecodeValue", func(t *testing.T) {
		t.Run("CodecDecodeError", func(t *testing.T) {
			val := reflect.New(reflect.TypeOf(false)).Elem()
			want := bsoncodec.ValueDecoderError{Name: "ArrayDecodeValue", Types: []reflect.Type{tArray}, Received: val}
			got := pcx.ArrayDecodeValue(bsoncodec.DecodeContext{}, &bsonrwtest.ValueReaderWriter{BSONType: bsontype.Array}, val)
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("ReadArray Error", func(t *testing.T) {
			want := errors.New("ReadArray Error")
			llvrw := &bsonrwtest.ValueReaderWriter{
				T:        t,
				Err:      want,
				ErrAfter: bsonrwtest.ReadArray,
				BSONType: bsontype.Array,
			}
			got := pcx.ArrayDecodeValue(bsoncodec.DecodeContext{}, llvrw, reflect.New(tArray).Elem())
			if !compareErrors(got, want) {
				t.Errorf("Errors do not match. got %v; want %v", got, want)
			}
		})
		t.Run("decode array errors", func(t *testing.T) {
			dc := bsoncodec.DecodeContext{}
			err := errors.New("decode array error")
			testCases := []struct {
				name  string
				dc    bsoncodec.DecodeContext
				llvrw *bsonrwtest.ValueReaderWriter
				err   error
			}{
				{
					"ReadValue",
					dc,
					&bsonrwtest.ValueReaderWriter{T: t, Err: errors.New("re error"), ErrAfter: bsonrwtest.ReadValue},
					errors.New("re error"),
				},
				{"ReadDouble", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadDouble, BSONType: bsontype.Double}, err},
				{"ReadString", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadString, BSONType: bsontype.String}, err},
				{"ReadBinary", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadBinary, BSONType: bsontype.Binary}, err},
				{"ReadUndefined", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadUndefined, BSONType: bsontype.Undefined}, err},
				{"ReadObjectID", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadObjectID, BSONType: bsontype.ObjectID}, err},
				{"ReadBoolean", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadBoolean, BSONType: bsontype.Boolean}, err},
				{"ReadDateTime", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadDateTime, BSONType: bsontype.DateTime}, err},
				{"ReadNull", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadNull, BSONType: bsontype.Null}, err},
				{"ReadRegex", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadRegex, BSONType: bsontype.Regex}, err},
				{"ReadDBPointer", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadDBPointer, BSONType: bsontype.DBPointer}, err},
				{"ReadJavascript", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadJavascript, BSONType: bsontype.JavaScript}, err},
				{"ReadSymbol", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadSymbol, BSONType: bsontype.Symbol}, err},
				{
					"ReadCodeWithScope (Lookup)", bsoncodec.DecodeContext{Registry: bsoncodec.NewRegistryBuilder().Build()},
					&bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadCodeWithScope, BSONType: bsontype.CodeWithScope},
					err,
				},
				{"ReadInt32", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadInt32, BSONType: bsontype.Int32}, err},
				{"ReadInt64", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadInt64, BSONType: bsontype.Int64}, err},
				{"ReadTimestamp", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadTimestamp, BSONType: bsontype.Timestamp}, err},
				{"ReadDecimal128", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadDecimal128, BSONType: bsontype.Decimal128}, err},
				{"ReadMinKey", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadMinKey, BSONType: bsontype.MinKey}, err},
				{"ReadMaxKey", dc, &bsonrwtest.ValueReaderWriter{T: t, Err: err, ErrAfter: bsonrwtest.ReadMaxKey, BSONType: bsontype.MaxKey}, err},
				{"Invalid Type", dc, &bsonrwtest.ValueReaderWriter{T: t, BSONType: bsontype.Type(0)}, fmt.Errorf("Cannot read unknown BSON type %s", bsontype.Type(0))},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					err := pcx.ArrayDecodeValue(tc.dc, tc.llvrw, reflect.New(tArray).Elem())
					if !compareErrors(err, tc.err) {
						t.Errorf("Errors do not match. got %v; want %v", err, tc.err)
					}
				})
			}
		})

		t.Run("success", func(t *testing.T) {
			oid := primitive.ObjectID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C}
			d128 := primitive.NewDecimal128(10, 20)
			want := Arr{
				Double(3.14159), String("foo"), Document(Doc{{"aa", Null()}}),
				Array(Arr{Null()}),
				Binary(0xFF, []byte{0x01, 0x02, 0x03}), Undefined(),
				ObjectID(oid), Boolean(true), DateTime(1234567890), Null(), Regex("foo", "bar"),
				DBPointer("foobar", oid), JavaScript("var hello = 'world';"), Symbol("bazqux"),
				CodeWithScope("var hello = 'world';", Doc{{"ab", Null()}}), Int32(12345),
				Timestamp(10, 20), Int64(1234567890), Decimal128(d128), MinKey(), MaxKey(),
			}
			dc := bsoncodec.DecodeContext{Registry: NewRegistryBuilder().Build()}

			b, err := Doc{{"", Array(want)}}.MarshalBSON()
			noerr(t, err)
			dvr := bsonrw.NewBSONDocumentReader(b)
			dr, err := dvr.ReadDocument()
			noerr(t, err)
			_, vr, err := dr.ReadElement()
			noerr(t, err)

			val := reflect.New(tArray).Elem()
			err = pcx.ArrayDecodeValue(dc, vr, val)
			noerr(t, err)
			got := val.Interface().(Arr)
			if !got.Equal(want) {
				t.Error("Documents do not match")
				t.Errorf("\ngot :%v\nwant:%v", got, want)
			}
		})
	})

	t.Run("success path", func(t *testing.T) {
		testCases := []struct {
			name  string
			value interface{}
			b     []byte
			err   error
		}{
			{
				"map[string][]Element",
				map[string][]Elem{"Z": {{"A", Int32(1)}, {"B", Int32(2)}, {"EC", Int32(3)}}},
				docToBytes(Doc{{"Z", Document(Doc{{"A", Int32(1)}, {"B", Int32(2)}, {"EC", Int32(3)}})}}),
				nil,
			},
			{
				"map[string][]Value",
				map[string][]Val{"Z": {Int32(1), Int32(2), Int32(3)}},
				docToBytes(Doc{{"Z", Array(Arr{Int32(1), Int32(2), Int32(3)})}}),
				nil,
			},
			{
				"map[string]*Document",
				map[string]Doc{"Z": {{"foo", Null()}}},
				docToBytes(Doc{{"Z", Document(Doc{{"foo", Null()}})}}),
				nil,
			},
		}

		t.Run("Decode", func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					vr := bsonrw.NewBSONDocumentReader(tc.b)
					dec, err := bson.NewDecoderWithContext(bsoncodec.DecodeContext{Registry: DefaultRegistry}, vr)
					noerr(t, err)
					gotVal := reflect.New(reflect.TypeOf(tc.value))
					err = dec.Decode(gotVal.Interface())
					noerr(t, err)
					got := gotVal.Elem().Interface()
					want := tc.value
					if diff := cmp.Diff(
						got, want,
					); diff != "" {
						t.Errorf("difference:\n%s", diff)
						t.Errorf("Values are not equal.\ngot: %#v\nwant:%#v", got, want)
					}
				})
			}
		})
	})
}

func compareValues(v1, v2 Val) bool    { return v1.Equal(v2) }
func compareElements(e1, e2 Elem) bool { return e1.Equal(e2) }

func docToBytes(d Doc) []byte {
	b, err := d.MarshalBSON()
	if err != nil {
		panic(err)
	}
	return b
}
