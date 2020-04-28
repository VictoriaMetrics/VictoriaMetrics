// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/internal/testutil/assert"
)

func TestChangeStream(t *testing.T) {
	t.Run("nil cursor", func(t *testing.T) {
		cs := &ChangeStream{}

		id := cs.ID()
		assert.Equal(t, int64(0), id, "expected ID 0, got %v", id)
		assert.False(t, cs.Next(bgCtx), "expected Next to return false, got true")
		err := cs.Decode(nil)
		assert.Equal(t, ErrNilCursor, err, "expected error %v, got %v", ErrNilCursor, err)
		err = cs.Err()
		assert.Nil(t, err, "change stream error: %v", err)
		err = cs.Close(bgCtx)
		assert.Nil(t, err, "Close error: %v", err)
	})
}
