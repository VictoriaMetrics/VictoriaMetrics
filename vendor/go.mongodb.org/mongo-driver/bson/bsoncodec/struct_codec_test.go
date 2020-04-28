// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package bsoncodec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestZeoerInterfaceUsedByDecoder(t *testing.T) {
	enc := &StructCodec{}

	// cases that are zero, because they are known types or pointers
	var st *nonZeroer
	assert.True(t, enc.isZero(st))
	assert.True(t, enc.isZero(0))
	assert.True(t, enc.isZero(false))

	// cases that shouldn't be zero
	st = &nonZeroer{value: false}
	assert.False(t, enc.isZero(struct{ val bool }{val: true}))
	assert.False(t, enc.isZero(struct{ val bool }{val: false}))
	assert.False(t, enc.isZero(st))
	st.value = true
	assert.False(t, enc.isZero(st))

	// a test to see if the interface impacts the outcome
	z := zeroTest{}
	assert.False(t, enc.isZero(z))

	z.reportZero = true
	assert.True(t, enc.isZero(z))

	// *time.Time with nil should be zero
	var tp *time.Time
	assert.True(t, enc.isZero(tp))

	// actually all zeroer if nil should also be zero
	var zp *zeroTest
	assert.True(t, enc.isZero(zp))
}
