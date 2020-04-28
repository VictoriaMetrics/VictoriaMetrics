// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServerSession(t *testing.T) {

	t.Run("Expired", func(t *testing.T) {
		sess, err := newServerSession()
		require.Nil(t, err, "Unexpected error")
		if !sess.expired(0) {
			t.Errorf("session should be expired")
		}
		sess.LastUsed = time.Now().Add(-30 * time.Minute)
		if !sess.expired(30) {
			t.Errorf("session should be expired")
		}

	})
}
