// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package writeconcern_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

func TestWriteConcernWithOptions(t *testing.T) {
	t.Run("on nil WriteConcern", func(t *testing.T) {
		var wc *writeconcern.WriteConcern

		wc = wc.WithOptions(writeconcern.WMajority())
		require.Equal(t, wc.GetW().(string), "majority")
	})
	t.Run("on existing WriteConcern", func(t *testing.T) {
		wc := writeconcern.New(writeconcern.W(1), writeconcern.J(true))
		require.Equal(t, wc.GetW().(int), 1)
		require.Equal(t, wc.GetJ(), true)

		wc = wc.WithOptions(writeconcern.WMajority())
		require.Equal(t, wc.GetW().(string), "majority")
		require.Equal(t, wc.GetJ(), true)
	})
	t.Run("with multiple options", func(t *testing.T) {
		wc := writeconcern.New(writeconcern.W(1), writeconcern.J(true))
		require.Equal(t, wc.GetW().(int), 1)
		require.Equal(t, wc.GetJ(), true)
		require.Equal(t, wc.GetWTimeout(), time.Duration(0))

		wc = wc.WithOptions(writeconcern.WMajority(), writeconcern.WTimeout(time.Second))
		require.Equal(t, wc.GetW().(string), "majority")
		require.Equal(t, wc.GetJ(), true)
		require.Equal(t, wc.GetWTimeout(), time.Second)
	})
}
