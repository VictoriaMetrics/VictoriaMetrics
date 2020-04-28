// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mgocompat

import (
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
)

// Setter interface: a value implementing the bson.Setter interface will receive the BSON
// value via the SetBSON method during unmarshaling, and the object
// itself will not be changed as usual.
//
// If setting the value works, the method should return nil or alternatively
// mgocompat.ErrSetZero to set the respective field to its zero value (nil for
// pointer types). If SetBSON returns a non-nil error, the unmarshalling
// procedure will stop and error out with the provided value.
//
// This interface is generally useful in pointer receivers, since the method
// will want to change the receiver. A type field that implements the Setter
// interface doesn't have to be a pointer, though.
//
// For example:
//
//     type MyString string
//
//     func (s *MyString) SetBSON(raw bson.RawValue) error {
//         return raw.Unmarshal(s)
//     }
//
type Setter interface {
	SetBSON(raw bson.RawValue) error
}

// Getter interface: a value implementing the bson.Getter interface will have its GetBSON
// method called when the given value has to be marshalled, and the result
// of this method will be marshaled in place of the actual object.
//
// If GetBSON returns return a non-nil error, the marshalling procedure
// will stop and error out with the provided value.
type Getter interface {
	GetBSON() (interface{}, error)
}

// SetterDecodeValue is the ValueDecoderFunc for Setter types.
func SetterDecodeValue(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	if !val.IsValid() || (!val.Type().Implements(tSetter) && !reflect.PtrTo(val.Type()).Implements(tSetter)) {
		return bsoncodec.ValueDecoderError{Name: "SetterDecodeValue", Types: []reflect.Type{tSetter}, Received: val}
	}

	if val.Kind() == reflect.Ptr && val.IsNil() {
		if !val.CanSet() {
			return bsoncodec.ValueDecoderError{Name: "SetterDecodeValue", Types: []reflect.Type{tSetter}, Received: val}
		}
		val.Set(reflect.New(val.Type().Elem()))
	}

	if !val.Type().Implements(tSetter) {
		if !val.CanAddr() {
			return bsoncodec.ValueDecoderError{Name: "ValueUnmarshalerDecodeValue", Types: []reflect.Type{tSetter}, Received: val}
		}
		val = val.Addr() // If they type doesn't implement the interface, a pointer to it must.
	}

	t, src, err := bsonrw.Copier{}.CopyValueToBytes(vr)
	if err != nil {
		return err
	}

	fn := val.Convert(tSetter).MethodByName("SetBSON")

	errVal := fn.Call([]reflect.Value{reflect.ValueOf(bson.RawValue{Type: t, Value: src})})[0]
	if !errVal.IsNil() {
		err = errVal.Interface().(error)
		if err == ErrSetZero {
			val.Set(reflect.Zero(val.Type()))
			return nil
		}
		return err
	}
	return nil
}

// GetterEncodeValue is the ValueEncoderFunc for Getter types.
func GetterEncodeValue(ec bsoncodec.EncodeContext, vw bsonrw.ValueWriter, val reflect.Value) error {
	// Either val or a pointer to val must implement Getter
	switch {
	case !val.IsValid():
		return bsoncodec.ValueEncoderError{Name: "GetterEncodeValue", Types: []reflect.Type{tGetter}, Received: val}
	case val.Type().Implements(tGetter):
		// If Getter is implemented on a concrete type, make sure that val isn't a nil pointer
		if isImplementationNil(val, tGetter) {
			return vw.WriteNull()
		}
	case reflect.PtrTo(val.Type()).Implements(tGetter) && val.CanAddr():
		val = val.Addr()
	default:
		return bsoncodec.ValueEncoderError{Name: "GetterEncodeValue", Types: []reflect.Type{tGetter}, Received: val}
	}

	fn := val.Convert(tGetter).MethodByName("GetBSON")
	returns := fn.Call(nil)
	if !returns[1].IsNil() {
		return returns[1].Interface().(error)
	}
	intermediate := returns[0]
	encoder, err := ec.Registry.LookupEncoder(intermediate.Type())
	if err != nil {
		return err
	}
	return encoder.EncodeValue(ec, vw, intermediate)
}

// isImplementationNil returns if val is a nil pointer and inter is implemented on a concrete type
func isImplementationNil(val reflect.Value, inter reflect.Type) bool {
	vt := val.Type()
	for vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}
	return vt.Implements(inter) && val.Kind() == reflect.Ptr && val.IsNil()
}
