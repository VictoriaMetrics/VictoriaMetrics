// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package internal_test

import (
	"context"
	"sync"
	"testing"

	. "go.mongodb.org/mongo-driver/internal"
	"github.com/stretchr/testify/require"
)

func TestSemaphore_Wait(t *testing.T) {
	s := NewSemaphore(3)
	err := s.Wait(context.Background())
	require.NoError(t, err)
	err = s.Wait(context.Background())
	require.NoError(t, err)
	err = s.Wait(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = s.Wait(ctx)
		require.Error(t, err)
		wg.Done()
	}()

	cancel()
	wg.Wait()
}

func TestSemaphore_Release(t *testing.T) {
	s := NewSemaphore(3)
	err := s.Wait(context.Background())
	err = s.Wait(context.Background())
	err = s.Wait(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = s.Wait(context.Background())
		require.NoError(t, err)
		wg.Done()
	}()

	require.NoError(t, s.Release())
	wg.Wait()
}
