// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/bsonrw"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ExampleValueEncoder() {
	var _ ValueEncoderFunc = func(ec EncodeContext, vw bsonrw.ValueWriter, val reflect.Value) error {
		if val.Kind() != reflect.String {
			return ValueEncoderError{Name: "StringEncodeValue", Kinds: []reflect.Kind{reflect.String}, Received: val}
		}

		return vw.WriteString(val.String())
	}
}

func ExampleValueDecoder() {
	var _ ValueDecoderFunc = func(dc DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
		if !val.CanSet() || val.Kind() != reflect.String {
			return ValueDecoderError{Name: "StringDecodeValue", Kinds: []reflect.Kind{reflect.String}, Received: val}
		}

		if vr.Type() != bsontype.String {
			return fmt.Errorf("cannot decode %v into a string type", vr.Type())
		}

		str, err := vr.ReadString()
		if err != nil {
			return err
		}
		val.SetString(str)
		return nil
	}
}

func noerr(t *testing.T, err error) {
	if err != nil {
		t.Helper()
		t.Errorf("Unexpected error: (%T)%v", err, err)
		t.FailNow()
	}
}

func compareTime(t1, t2 time.Time) bool {
	if t1.Location() != t2.Location() {
		return false
	}
	return t1.Equal(t2)
}

func compareErrors(err1, err2 error) bool {
	if err1 == nil && err2 == nil {
		return true
	}

	if err1 == nil || err2 == nil {
		return false
	}

	if err1.Error() != err2.Error() {
		return false
	}

	return true
}

func compareDecimal128(d1, d2 primitive.Decimal128) bool {
	d1H, d1L := d1.GetBytes()
	d2H, d2L := d2.GetBytes()

	if d1H != d2H {
		return false
	}

	if d1L != d2L {
		return false
	}

	return true
}

func compareStrings(s1, s2 string) bool { return s1 == s2 }

type noPrivateFields struct {
	a string
}

func compareNoPrivateFields(npf1, npf2 noPrivateFields) bool {
	return npf1.a != npf2.a // We don't want these to be equal
}

type zeroTest struct {
	reportZero bool
}

func (z zeroTest) IsZero() bool { return z.reportZero }

func compareZeroTest(_, _ zeroTest) bool { return true }

type nonZeroer struct {
	value bool
}

type llCodec struct {
	t         *testing.T
	decodeval interface{}
	encodeval interface{}
	err       error
}

func (llc *llCodec) EncodeValue(_ EncodeContext, _ bsonrw.ValueWriter, i interface{}) error {
	if llc.err != nil {
		return llc.err
	}

	llc.encodeval = i
	return nil
}

func (llc *llCodec) DecodeValue(_ DecodeContext, _ bsonrw.ValueReader, val reflect.Value) error {
	if llc.err != nil {
		return llc.err
	}

	if !reflect.TypeOf(llc.decodeval).AssignableTo(val.Type()) {
		llc.t.Errorf("decodeval must be assignable to val provided to DecodeValue, but is not. decodeval %T; val %T", llc.decodeval, val)
		return nil
	}

	val.Set(reflect.ValueOf(llc.decodeval))
	return nil
}
