// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package session

import (
	"testing"

	"go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/x/mongo/driver/description"
)

func TestSessionPool(t *testing.T) {
	t.Run("TestLifo", func(t *testing.T) {
		descChan := make(chan description.Topology)
		p := NewPool(descChan)
		p.timeout = 30 // Set to some arbitrarily high number greater than 1 minute.

		first, err := p.GetSession()
		testhelpers.RequireNil(t, err, "error getting session %s", err)
		firstID := first.SessionID

		second, err := p.GetSession()
		testhelpers.RequireNil(t, err, "error getting session %s", err)
		secondID := second.SessionID

		p.ReturnSession(first)
		p.ReturnSession(second)

		sess, err := p.GetSession()
		testhelpers.RequireNil(t, err, "error getting session %s", err)
		nextSess, err := p.GetSession()
		testhelpers.RequireNil(t, err, "error getting session %s", err)

		if !sess.SessionID.Equal(secondID) {
			t.Errorf("first sesssion ID mismatch. got %s expected %s", sess.SessionID, secondID)
		}

		if !nextSess.SessionID.Equal(firstID) {
			t.Errorf("second sesssion ID mismatch. got %s expected %s", nextSess.SessionID, firstID)
		}
	})

	t.Run("TestExpiredRemoved", func(t *testing.T) {
		descChan := make(chan description.Topology)
		p := NewPool(descChan)
		// New sessions will always become stale when returned
		p.timeout = 0

		first, err := p.GetSession()
		testhelpers.RequireNil(t, err, "error getting session %s", err)
		firstID := first.SessionID

		second, err := p.GetSession()
		testhelpers.RequireNil(t, err, "error getting session %s", err)
		secondID := second.SessionID

		p.ReturnSession(first)
		p.ReturnSession(second)

		sess, err := p.GetSession()
		testhelpers.RequireNil(t, err, "error getting session %s", err)

		if sess.SessionID.Equal(firstID) || sess.SessionID.Equal(secondID) {
			t.Errorf("Expired sessions not removed!")
		}
	})
}
