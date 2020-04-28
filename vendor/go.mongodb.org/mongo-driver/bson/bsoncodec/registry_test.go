// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

func TestRegistry(t *testing.T) {
	t.Run("Register", func(t *testing.T) {
		fc1, fc2, fc3, fc4 := new(fakeCodec), new(fakeCodec), new(fakeCodec), new(fakeCodec)
		t.Run("interface", func(t *testing.T) {
			var t1f *testInterface1
			var t2f *testInterface2
			var t4f *testInterface4
			ips := []interfaceValueEncoder{
				{i: reflect.TypeOf(t1f).Elem(), ve: fc1},
				{i: reflect.TypeOf(t2f).Elem(), ve: fc2},
				{i: reflect.TypeOf(t1f).Elem(), ve: fc3},
				{i: reflect.TypeOf(t4f).Elem(), ve: fc4},
			}
			want := []interfaceValueEncoder{
				{i: reflect.TypeOf(t1f).Elem(), ve: fc3},
				{i: reflect.TypeOf(t2f).Elem(), ve: fc2},
				{i: reflect.TypeOf(t4f).Elem(), ve: fc4},
			}
			rb := NewRegistryBuilder()
			for _, ip := range ips {
				rb.RegisterHookEncoder(ip.i, ip.ve)
			}
			got := rb.interfaceEncoders
			if !cmp.Equal(got, want, cmp.AllowUnexported(interfaceValueEncoder{}, fakeCodec{}), cmp.Comparer(typeComparer)) {
				t.Errorf("The registered interfaces are not correct. got %v; want %v", got, want)
			}
		})
		t.Run("type", func(t *testing.T) {
			ft1, ft2, ft4 := fakeType1{}, fakeType2{}, fakeType4{}
			rb := NewRegistryBuilder().
				RegisterTypeEncoder(reflect.TypeOf(ft1), fc1).
				RegisterTypeEncoder(reflect.TypeOf(ft2), fc2).
				RegisterTypeEncoder(reflect.TypeOf(ft1), fc3).
				RegisterTypeEncoder(reflect.TypeOf(ft4), fc4)
			want := []struct {
				t reflect.Type
				c ValueEncoder
			}{
				{reflect.TypeOf(ft1), fc3},
				{reflect.TypeOf(ft2), fc2},
				{reflect.TypeOf(ft4), fc4},
			}
			got := rb.typeEncoders
			for _, s := range want {
				wantT, wantC := s.t, s.c
				gotC, exists := got[wantT]
				if !exists {
					t.Errorf("Did not find type in the type registry: %v", wantT)
				}
				if !cmp.Equal(gotC, wantC, cmp.AllowUnexported(fakeCodec{})) {
					t.Errorf("Codecs did not match. got %#v; want %#v", gotC, wantC)
				}
			}
		})
		t.Run("kind", func(t *testing.T) {
			k1, k2, k4 := reflect.Struct, reflect.Slice, reflect.Map
			rb := NewRegistryBuilder().
				RegisterDefaultEncoder(k1, fc1).
				RegisterDefaultEncoder(k2, fc2).
				RegisterDefaultEncoder(k1, fc3).
				RegisterDefaultEncoder(k4, fc4)
			want := []struct {
				k reflect.Kind
				c ValueEncoder
			}{
				{k1, fc3},
				{k2, fc2},
				{k4, fc4},
			}
			got := rb.kindEncoders
			for _, s := range want {
				wantK, wantC := s.k, s.c
				gotC, exists := got[wantK]
				if !exists {
					t.Errorf("Did not find kind in the kind registry: %v", wantK)
				}
				if !cmp.Equal(gotC, wantC, cmp.AllowUnexported(fakeCodec{})) {
					t.Errorf("Codecs did not match. got %#v; want %#v", gotC, wantC)
				}
			}
		})
		t.Run("RegisterDefault", func(t *testing.T) {
			t.Run("MapCodec", func(t *testing.T) {
				codec := fakeCodec{num: 1}
				codec2 := fakeCodec{num: 2}
				rb := NewRegistryBuilder()
				rb.RegisterDefaultEncoder(reflect.Map, codec)
				if rb.kindEncoders[reflect.Map] != codec {
					t.Errorf("Did not properly set the map codec. got %v; want %v", rb.kindEncoders[reflect.Map], codec)
				}
				rb.RegisterDefaultEncoder(reflect.Map, codec2)
				if rb.kindEncoders[reflect.Map] != codec2 {
					t.Errorf("Did not properly set the map codec. got %v; want %v", rb.kindEncoders[reflect.Map], codec2)
				}
			})
			t.Run("StructCodec", func(t *testing.T) {
				codec := fakeCodec{num: 1}
				codec2 := fakeCodec{num: 2}
				rb := NewRegistryBuilder()
				rb.RegisterDefaultEncoder(reflect.Struct, codec)
				if rb.kindEncoders[reflect.Struct] != codec {
					t.Errorf("Did not properly set the struct codec. got %v; want %v", rb.kindEncoders[reflect.Struct], codec)
				}
				rb.RegisterDefaultEncoder(reflect.Struct, codec2)
				if rb.kindEncoders[reflect.Struct] != codec2 {
					t.Errorf("Did not properly set the struct codec. got %v; want %v", rb.kindEncoders[reflect.Struct], codec2)
				}
			})
			t.Run("SliceCodec", func(t *testing.T) {
				codec := fakeCodec{num: 1}
				codec2 := fakeCodec{num: 2}
				rb := NewRegistryBuilder()
				rb.RegisterDefaultEncoder(reflect.Slice, codec)
				if rb.kindEncoders[reflect.Slice] != codec {
					t.Errorf("Did not properly set the slice codec. got %v; want %v", rb.kindEncoders[reflect.Slice], codec)
				}
				rb.RegisterDefaultEncoder(reflect.Slice, codec2)
				if rb.kindEncoders[reflect.Slice] != codec2 {
					t.Errorf("Did not properly set the slice codec. got %v; want %v", rb.kindEncoders[reflect.Slice], codec2)
				}
			})
			t.Run("ArrayCodec", func(t *testing.T) {
				codec := fakeCodec{num: 1}
				codec2 := fakeCodec{num: 2}
				rb := NewRegistryBuilder()
				rb.RegisterDefaultEncoder(reflect.Array, codec)
				if rb.kindEncoders[reflect.Array] != codec {
					t.Errorf("Did not properly set the slice codec. got %v; want %v", rb.kindEncoders[reflect.Array], codec)
				}
				rb.RegisterDefaultEncoder(reflect.Array, codec2)
				if rb.kindEncoders[reflect.Array] != codec2 {
					t.Errorf("Did not properly set the slice codec. got %v; want %v", rb.kindEncoders[reflect.Array], codec2)
				}
			})
		})
		t.Run("Lookup", func(t *testing.T) {
			type Codec interface {
				ValueEncoder
				ValueDecoder
			}

			var arrinstance [12]int
			arr := reflect.TypeOf(arrinstance)
			slc := reflect.TypeOf(make([]int, 12))
			m := reflect.TypeOf(make(map[string]int))
			strct := reflect.TypeOf(struct{ Foo string }{})
			ft1 := reflect.PtrTo(reflect.TypeOf(fakeType1{}))
			ft2 := reflect.TypeOf(fakeType2{})
			ft3 := reflect.TypeOf(fakeType5(func(string, string) string { return "fakeType5" }))
			ti1 := reflect.TypeOf((*testInterface1)(nil)).Elem()
			ti2 := reflect.TypeOf((*testInterface2)(nil)).Elem()
			ti1Impl := reflect.TypeOf(testInterface1Impl{})
			ti2Impl := reflect.TypeOf(testInterface2Impl{})
			ti3 := reflect.TypeOf((*testInterface3)(nil)).Elem()
			ti3Impl := reflect.TypeOf(testInterface3Impl{})
			ti3ImplPtr := reflect.TypeOf((*testInterface3Impl)(nil))
			fc1, fc2 := fakeCodec{num: 1}, fakeCodec{num: 2}
			fsc, fslcc, fmc := new(fakeStructCodec), new(fakeSliceCodec), new(fakeMapCodec)
			pc := NewPointerCodec()

			reg := NewRegistryBuilder().
				RegisterTypeEncoder(ft1, fc1).
				RegisterTypeEncoder(ft2, fc2).
				RegisterTypeEncoder(ti1, fc1).
				RegisterDefaultEncoder(reflect.Struct, fsc).
				RegisterDefaultEncoder(reflect.Slice, fslcc).
				RegisterDefaultEncoder(reflect.Array, fslcc).
				RegisterDefaultEncoder(reflect.Map, fmc).
				RegisterDefaultEncoder(reflect.Ptr, pc).
				RegisterTypeDecoder(ft1, fc1).
				RegisterTypeDecoder(ft2, fc2).
				RegisterTypeDecoder(ti1, fc1). // values whose exact type is testInterface1 will use fc1 encoder
				RegisterDefaultDecoder(reflect.Struct, fsc).
				RegisterDefaultDecoder(reflect.Slice, fslcc).
				RegisterDefaultDecoder(reflect.Array, fslcc).
				RegisterDefaultDecoder(reflect.Map, fmc).
				RegisterDefaultDecoder(reflect.Ptr, pc).
				RegisterHookEncoder(ti2, fc2).
				RegisterHookDecoder(ti2, fc2).
				RegisterHookEncoder(ti3, fc3).
				RegisterHookDecoder(ti3, fc3).
				Build()

			testCases := []struct {
				name      string
				t         reflect.Type
				wantcodec Codec
				wanterr   error
				testcache bool
			}{
				{
					"type registry (pointer)",
					ft1,
					fc1,
					nil,
					false,
				},
				{
					"type registry (non-pointer)",
					ft2,
					fc2,
					nil,
					false,
				},
				{
					// lookup an interface type and expect that the registered encoder is returned
					"interface with type encoder",
					ti1,
					fc1,
					nil,
					true,
				},
				{
					// lookup a type that implements an interface and expect that the default struct codec is returned
					"interface implementation with type encoder",
					ti1Impl,
					fsc,
					nil,
					false,
				},
				{
					// lookup an interface type and expect that the registered hook is returned
					"interface with hook",
					ti2,
					fc2,
					nil,
					false,
				},
				{
					// lookup a type that implements an interface and expect that the registered hook is returned
					"interface implementation with hook",
					ti2Impl,
					fc2,
					nil,
					false,
				},
				{
					// lookup a type whose pointer implements an interface and expect that the registered hook is
					// returned
					"interface implementation with hook (pointer)",
					ti3Impl,
					fc3,
					nil,
					false,
				},
				{
					// lookup a pointer to a type where the pointer implements an interface and expect that the
					// registered hook is returned
					"interface pointer to implementation with hook (pointer)",
					ti3ImplPtr,
					fc3,
					nil,
					false,
				},
				{
					"default struct codec (pointer)",
					reflect.PtrTo(strct),
					pc,
					nil,
					false,
				},
				{
					"default struct codec (non-pointer)",
					strct,
					fsc,
					nil,
					false,
				},
				{
					"default array codec",
					arr,
					fslcc,
					nil,
					false,
				},
				{
					"default slice codec",
					slc,
					fslcc,
					nil,
					false,
				},
				{
					"default map",
					m,
					fmc,
					nil,
					false,
				},
				{
					"map non-string key",
					reflect.TypeOf(map[int]int{}),
					fmc,
					nil,
					false,
				},
				{
					"No Codec Registered",
					ft3,
					nil,
					ErrNoEncoder{Type: ft3},
					false,
				},
			}

			allowunexported := cmp.AllowUnexported(fakeCodec{}, fakeStructCodec{}, fakeSliceCodec{}, fakeMapCodec{})
			comparepc := func(pc1, pc2 *PointerCodec) bool { return true }
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					t.Run("Encoder", func(t *testing.T) {
						gotcodec, goterr := reg.LookupEncoder(tc.t)
						if !cmp.Equal(goterr, tc.wanterr, cmp.Comparer(compareErrors)) {
							t.Errorf("Errors did not match. got %v; want %v", goterr, tc.wanterr)
						}
						if !cmp.Equal(gotcodec, tc.wantcodec, allowunexported, cmp.Comparer(comparepc)) {
							t.Errorf("Codecs did not match. got %v; want %v", gotcodec, tc.wantcodec)
						}
					})
					t.Run("Decoder", func(t *testing.T) {
						var wanterr error
						if ene, ok := tc.wanterr.(ErrNoEncoder); ok {
							wanterr = ErrNoDecoder{Type: ene.Type}
						} else {
							wanterr = tc.wanterr
						}
						gotcodec, goterr := reg.LookupDecoder(tc.t)
						if !cmp.Equal(goterr, wanterr, cmp.Comparer(compareErrors)) {
							t.Errorf("Errors did not match. got %v; want %v", goterr, wanterr)
						}
						if !cmp.Equal(gotcodec, tc.wantcodec, allowunexported, cmp.Comparer(comparepc)) {
							t.Errorf("Codecs did not match. got %v; want %v", gotcodec, tc.wantcodec)
							t.Errorf("Codecs did not match. got %T; want %T", gotcodec, tc.wantcodec)
						}
					})
				})
			}
		})
	})
	t.Run("Type Map", func(t *testing.T) {
		reg := NewRegistryBuilder().
			RegisterTypeMapEntry(bsontype.String, reflect.TypeOf(string(""))).
			RegisterTypeMapEntry(bsontype.Int32, reflect.TypeOf(int(0))).
			Build()

		var got, want reflect.Type

		want = reflect.TypeOf(string(""))
		got, err := reg.LookupTypeMapEntry(bsontype.String)
		noerr(t, err)
		if got != want {
			t.Errorf("Did not get expected type. got %v; want %v", got, want)
		}

		want = reflect.TypeOf(int(0))
		got, err = reg.LookupTypeMapEntry(bsontype.Int32)
		noerr(t, err)
		if got != want {
			t.Errorf("Did not get expected type. got %v; want %v", got, want)
		}

		want = nil
		wanterr := ErrNoTypeMapEntry{Type: bsontype.ObjectID}
		got, err = reg.LookupTypeMapEntry(bsontype.ObjectID)
		if err != wanterr {
			t.Errorf("Did not get expected error. got %v; want %v", err, wanterr)
		}
		if got != want {
			t.Errorf("Did not get expected type. got %v; want %v", got, want)
		}
	})
}

type fakeType1 struct{ b bool }
type fakeType2 struct{ b bool }
type fakeType3 struct{ b bool }
type fakeType4 struct{ b bool }
type fakeType5 func(string, string) string
type fakeStructCodec struct{ fakeCodec }
type fakeSliceCodec struct{ fakeCodec }
type fakeMapCodec struct{ fakeCodec }

type fakeCodec struct{ num int }

func (fc fakeCodec) EncodeValue(EncodeContext, bsonrw.ValueWriter, reflect.Value) error {
	return nil
}
func (fc fakeCodec) DecodeValue(DecodeContext, bsonrw.ValueReader, reflect.Value) error {
	return nil
}

type testInterface1 interface{ test1() }
type testInterface2 interface{ test2() }
type testInterface3 interface{ test3() }
type testInterface4 interface{ test4() }

type testInterface1Impl struct{}

var _ testInterface1 = testInterface1Impl{}

func (testInterface1Impl) test1() {}

type testInterface2Impl struct{}

var _ testInterface2 = testInterface2Impl{}

func (testInterface2Impl) test2() {}

type testInterface3Impl struct{}

var _ testInterface3 = (*testInterface3Impl)(nil)

func (*testInterface3Impl) test3() {}

func typeComparer(i1, i2 reflect.Type) bool { return i1 == i2 }
