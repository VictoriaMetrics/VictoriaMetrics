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
)

func TestDefaultStructTagParser(t *testing.T) {
	testCases := []struct {
		name string
		sf   reflect.StructField
		want StructTags
	}{
		{
			"no bson tag",
			reflect.StructField{Name: "foo", Tag: reflect.StructTag("bar")},
			StructTags{Name: "bar"},
		},
		{
			"empty",
			reflect.StructField{Name: "foo", Tag: reflect.StructTag("")},
			StructTags{Name: "foo"},
		},
		{
			"tag only dash",
			reflect.StructField{Name: "foo", Tag: reflect.StructTag("-")},
			StructTags{Skip: true},
		},
		{
			"bson tag only dash",
			reflect.StructField{Name: "foo", Tag: reflect.StructTag(`bson:"-"`)},
			StructTags{Skip: true},
		},
		{
			"all options",
			reflect.StructField{Name: "foo", Tag: reflect.StructTag(`bar,omitempty,minsize,truncate,inline`)},
			StructTags{Name: "bar", OmitEmpty: true, MinSize: true, Truncate: true, Inline: true},
		},
		{
			"all options default name",
			reflect.StructField{Name: "foo", Tag: reflect.StructTag(`,omitempty,minsize,truncate,inline`)},
			StructTags{Name: "foo", OmitEmpty: true, MinSize: true, Truncate: true, Inline: true},
		},
		{
			"bson tag all options",
			reflect.StructField{Name: "foo", Tag: reflect.StructTag(`bson:"bar,omitempty,minsize,truncate,inline"`)},
			StructTags{Name: "bar", OmitEmpty: true, MinSize: true, Truncate: true, Inline: true},
		},
		{
			"bson tag all options default name",
			reflect.StructField{Name: "foo", Tag: reflect.StructTag(`bson:",omitempty,minsize,truncate,inline"`)},
			StructTags{Name: "foo", OmitEmpty: true, MinSize: true, Truncate: true, Inline: true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DefaultStructTagParser(tc.sf)
			noerr(t, err)
			if !cmp.Equal(got, tc.want) {
				t.Errorf("Returned struct tags do not match. got %#v; want %#v", got, tc.want)
			}
		})
	}
}
