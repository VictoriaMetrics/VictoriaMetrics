// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bson

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsonrw/bsonrwtest"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestBasicDecode(t *testing.T) {
	for _, tc := range unmarshalingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			got := reflect.New(tc.sType).Elem()
			vr := bsonrw.NewBSONDocumentReader(tc.data)
			reg := DefaultRegistry
			decoder, err := reg.LookupDecoder(reflect.TypeOf(got))
			noerr(t, err)
			err = decoder.DecodeValue(bsoncodec.DecodeContext{Registry: reg}, vr, got)
			noerr(t, err)

			if !reflect.DeepEqual(got.Addr().Interface(), tc.want) {
				t.Errorf("Results do not match. got %+v; want %+v", got, tc.want)
			}
		})
	}
}

func TestDecoderv2(t *testing.T) {
	t.Run("Decode", func(t *testing.T) {
		for _, tc := range unmarshalingTestCases {
			t.Run(tc.name, func(t *testing.T) {
				got := reflect.New(tc.sType).Interface()
				vr := bsonrw.NewBSONDocumentReader(tc.data)
				var reg *bsoncodec.Registry
				if tc.reg != nil {
					reg = tc.reg
				} else {
					reg = DefaultRegistry
				}
				dec, err := NewDecoderWithContext(bsoncodec.DecodeContext{Registry: reg}, vr)
				noerr(t, err)
				err = dec.Decode(got)
				noerr(t, err)

				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("Results do not match. got %+v; want %+v", got, tc.want)
				}
			})
		}
		t.Run("lookup error", func(t *testing.T) {
			type certainlydoesntexistelsewhereihope func(string, string) string
			cdeih := func(string, string) string { return "certainlydoesntexistelsewhereihope" }
			dec, err := NewDecoder(bsonrw.NewBSONDocumentReader([]byte{}))
			noerr(t, err)
			want := bsoncodec.ErrNoDecoder{Type: reflect.TypeOf(cdeih)}
			got := dec.Decode(&cdeih)
			if !cmp.Equal(got, want, cmp.Comparer(compareErrors)) {
				t.Errorf("Received unexpected error. got %v; want %v", got, want)
			}
		})
		t.Run("Unmarshaler", func(t *testing.T) {
			testCases := []struct {
				name    string
				err     error
				vr      bsonrw.ValueReader
				invoked bool
			}{
				{
					"error",
					errors.New("Unmarshaler error"),
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.EmbeddedDocument, Err: bsonrw.ErrEOD, ErrAfter: bsonrwtest.ReadElement},
					true,
				},
				{
					"copy error",
					errors.New("copy error"),
					&bsonrwtest.ValueReaderWriter{Err: errors.New("copy error"), ErrAfter: bsonrwtest.ReadDocument},
					false,
				},
				{
					"success",
					nil,
					&bsonrwtest.ValueReaderWriter{BSONType: bsontype.EmbeddedDocument, Err: bsonrw.ErrEOD, ErrAfter: bsonrwtest.ReadElement},
					true,
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					unmarshaler := &testUnmarshaler{err: tc.err}
					dec, err := NewDecoder(tc.vr)
					noerr(t, err)
					got := dec.Decode(unmarshaler)
					want := tc.err
					if !compareErrors(got, want) {
						t.Errorf("Did not receive expected error. got %v; want %v", got, want)
					}
					if unmarshaler.invoked != tc.invoked {
						if tc.invoked {
							t.Error("Expected to have UnmarshalBSON invoked, but it wasn't.")
						} else {
							t.Error("Expected UnmarshalBSON to not be invoked, but it was.")
						}
					}
				})
			}

			t.Run("Unmarshaler/success bsonrw.ValueReader", func(t *testing.T) {
				want := bsoncore.BuildDocument(nil, bsoncore.AppendDoubleElement(nil, "pi", 3.14159))
				unmarshaler := &testUnmarshaler{}
				vr := bsonrw.NewBSONDocumentReader(want)
				dec, err := NewDecoder(vr)
				noerr(t, err)
				err = dec.Decode(unmarshaler)
				noerr(t, err)
				got := unmarshaler.data
				if !bytes.Equal(got, want) {
					t.Errorf("Did not unmarshal properly. got %v; want %v", got, want)
				}
			})
		})
	})
	t.Run("NewDecoder", func(t *testing.T) {
		t.Run("error", func(t *testing.T) {
			_, got := NewDecoder(nil)
			want := errors.New("cannot create a new Decoder with a nil ValueReader")
			if !cmp.Equal(got, want, cmp.Comparer(compareErrors)) {
				t.Errorf("Was expecting error but got different error. got %v; want %v", got, want)
			}
		})
		t.Run("success", func(t *testing.T) {
			got, err := NewDecoder(bsonrw.NewBSONDocumentReader([]byte{}))
			noerr(t, err)
			if got == nil {
				t.Errorf("Was expecting a non-nil Decoder, but got <nil>")
			}
		})
	})
	t.Run("NewDecoderWithContext", func(t *testing.T) {
		t.Run("errors", func(t *testing.T) {
			dc := bsoncodec.DecodeContext{Registry: DefaultRegistry}
			_, got := NewDecoderWithContext(dc, nil)
			want := errors.New("cannot create a new Decoder with a nil ValueReader")
			if !cmp.Equal(got, want, cmp.Comparer(compareErrors)) {
				t.Errorf("Was expecting error but got different error. got %v; want %v", got, want)
			}
		})
		t.Run("success", func(t *testing.T) {
			got, err := NewDecoderWithContext(bsoncodec.DecodeContext{}, bsonrw.NewBSONDocumentReader([]byte{}))
			noerr(t, err)
			if got == nil {
				t.Errorf("Was expecting a non-nil Decoder, but got <nil>")
			}
			dc := bsoncodec.DecodeContext{Registry: DefaultRegistry}
			got, err = NewDecoderWithContext(dc, bsonrw.NewBSONDocumentReader([]byte{}))
			noerr(t, err)
			if got == nil {
				t.Errorf("Was expecting a non-nil Decoder, but got <nil>")
			}
		})
	})
	t.Run("Decode doesn't zero struct", func(t *testing.T) {
		type foo struct {
			Item  string
			Qty   int
			Bonus int
		}
		var got foo
		got.Item = "apple"
		got.Bonus = 2
		data := docToBytes(D{{"item", "canvas"}, {"qty", 4}})
		vr := bsonrw.NewBSONDocumentReader(data)
		dec, err := NewDecoder(vr)
		noerr(t, err)
		err = dec.Decode(&got)
		noerr(t, err)
		want := foo{Item: "canvas", Qty: 4, Bonus: 2}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Results do not match. got %+v; want %+v", got, want)
		}
	})
	t.Run("Reset", func(t *testing.T) {
		vr1, vr2 := bsonrw.NewBSONDocumentReader([]byte{}), bsonrw.NewBSONDocumentReader([]byte{})
		dc := bsoncodec.DecodeContext{Registry: DefaultRegistry}
		dec, err := NewDecoderWithContext(dc, vr1)
		noerr(t, err)
		if dec.vr != vr1 {
			t.Errorf("Decoder should use the value reader provided. got %v; want %v", dec.vr, vr1)
		}
		err = dec.Reset(vr2)
		noerr(t, err)
		if dec.vr != vr2 {
			t.Errorf("Decoder should use the value reader provided. got %v; want %v", dec.vr, vr2)
		}
	})
	t.Run("SetContext", func(t *testing.T) {
		dc1 := bsoncodec.DecodeContext{Registry: DefaultRegistry}
		dc2 := bsoncodec.DecodeContext{Registry: NewRegistryBuilder().Build()}
		dec, err := NewDecoderWithContext(dc1, bsonrw.NewBSONDocumentReader([]byte{}))
		noerr(t, err)
		if dec.dc != dc1 {
			t.Errorf("Decoder should use the Registry provided. got %v; want %v", dec.dc, dc1)
		}
		err = dec.SetContext(dc2)
		noerr(t, err)
		if dec.dc != dc2 {
			t.Errorf("Decoder should use the Registry provided. got %v; want %v", dec.dc, dc2)
		}
	})
	t.Run("SetRegistry", func(t *testing.T) {
		r1, r2 := DefaultRegistry, NewRegistryBuilder().Build()
		dc1 := bsoncodec.DecodeContext{Registry: r1}
		dc2 := bsoncodec.DecodeContext{Registry: r2}
		dec, err := NewDecoder(bsonrw.NewBSONDocumentReader([]byte{}))
		noerr(t, err)
		if dec.dc != dc1 {
			t.Errorf("Decoder should use the Registry provided. got %v; want %v", dec.dc, dc1)
		}
		err = dec.SetRegistry(r2)
		noerr(t, err)
		if dec.dc != dc2 {
			t.Errorf("Decoder should use the Registry provided. got %v; want %v", dec.dc, dc2)
		}
	})
	t.Run("DecodeToNil", func(t *testing.T) {
		data := docToBytes(D{{"item", "canvas"}, {"qty", 4}})
		vr := bsonrw.NewBSONDocumentReader(data)
		dec, err := NewDecoder(vr)
		noerr(t, err)

		var got *D
		err = dec.Decode(got)
		if err != ErrDecodeToNil {
			t.Fatalf("Decode error mismatch; expected %v, got %v", ErrDecodeToNil, err)
		}
	})
}

type testDecoderCodec struct {
	EncodeValueCalled bool
	DecodeValueCalled bool
}

func (tdc *testDecoderCodec) EncodeValue(bsoncodec.EncodeContext, bsonrw.ValueWriter, interface{}) error {
	tdc.EncodeValueCalled = true
	return nil
}

func (tdc *testDecoderCodec) DecodeValue(bsoncodec.DecodeContext, bsonrw.ValueReader, interface{}) error {
	tdc.DecodeValueCalled = true
	return nil
}

type testUnmarshaler struct {
	invoked bool
	err     error
	data    []byte
}

func (tu *testUnmarshaler) UnmarshalBSON(d []byte) error {
	tu.invoked = true
	tu.data = d
	return tu.err
}
