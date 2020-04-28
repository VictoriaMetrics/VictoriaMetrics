// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package topology

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
)

// rsrc is a mock resource used in resource pool tests.
// This type should not be used other test files.
type rsrc struct {
	closed bool
}

func initRsrc() interface{} {
	return &rsrc{}
}

func closeRsrc(v interface{}) {
	v.(*rsrc).closed = true
}

func alwaysExpired(_ interface{}) bool {
	return true
}

func neverExpired(_ interface{}) bool {
	return false
}

// expiredCounter is used to implement an expiredFunc that will return true a fixed number of times.
type expiredCounter struct {
	expiredCalled, closeCalled int32 // must be loaded/stored using atomic.*Int32 functions
	total                      int32
	closeChan                  chan struct{}
}

func newExpiredCounter(total int32) expiredCounter {
	return expiredCounter{
		total:     total,
		closeChan: make(chan struct{}, 1),
	}
}

func (ec *expiredCounter) expired(_ interface{}) bool {
	atomic.AddInt32(&ec.expiredCalled, 1)
	return ec.getExpiredCalled() <= ec.total
}

func (ec *expiredCounter) close(_ interface{}) {
	atomic.AddInt32(&ec.closeCalled, 1)
	if ec.getCloseCalled() == ec.total {
		ec.closeChan <- struct{}{}
	}
}

func (ec *expiredCounter) getExpiredCalled() int32 {
	return atomic.LoadInt32(&ec.expiredCalled)
}

func (ec *expiredCounter) getCloseCalled() int32 {
	return atomic.LoadInt32(&ec.closeCalled)
}

func initPool(t *testing.T, minSize uint64, expFn expiredFunc, closeFn closeFunc, initFn initFunc, pruneInterval time.Duration) *resourcePool {
	t.Helper()

	rpc := resourcePoolConfig{
		MinSize:          minSize,
		MaintainInterval: pruneInterval,
		ExpiredFn:        expFn,
		CloseFn:          closeFn,
		InitFn:           initFn,
	}
	rp, err := newResourcePool(rpc)
	assert.Nil(t, err, "error creating new resource pool: %v", err)
	rp.initialize()
	rp.maintainTimer.Reset(rp.maintainInterval)
	return rp
}

func TestResourcePool(t *testing.T) {
	// register a cmp equality function for the rsrc type that will do a pointer comparison
	assert.RegisterOpts(reflect.TypeOf(&rsrc{}), cmp.Comparer(func(r1, r2 *rsrc) bool {
		return r1 == r2
	}))

	t.Run("get", func(t *testing.T) {
		t.Run("remove stale resources", func(t *testing.T) {
			ec := newExpiredCounter(5)
			rp := initPool(t, 1, ec.expired, ec.close, initRsrc, time.Minute)
			rp.maintainTimer.Stop()

			got := rp.Get()
			assert.Nil(t, got, "expected nil, got %v", got)
			assert.Equal(t, uint64(0), rp.size, "expected size 0, got %d", rp.size)

			expiredCalled := ec.getExpiredCalled()
			assert.Equal(t, int32(1), expiredCalled, "expected expire to be called 1 time, got %v", expiredCalled)
			closeCalled := ec.getCloseCalled()
			assert.Equal(t, int32(1), closeCalled, "expected close to be called 1 time, got %v", closeCalled)
		})
		t.Run("recycle resources", func(t *testing.T) {
			rp := initPool(t, 1, neverExpired, closeRsrc, initRsrc, time.Minute)
			rp.maintainTimer.Stop()
			for i := 0; i < 5; i++ {
				got := rp.Get()
				assert.NotNil(t, got, "expected resource, got nil")
				assert.Equal(t, uint64(0), rp.size, "expected size 0, got %v", rp.size)

				rp.Put(got)
				assert.Equal(t, uint64(1), rp.size, "expected size 1, got %v", rp.size)
			}
		})
	})
	t.Run("Put", func(t *testing.T) {
		t.Run("returned resources are returned to front of pool", func(t *testing.T) {
			rp := initPool(t, 0, neverExpired, closeRsrc, initRsrc, time.Minute)
			ret := &rsrc{}
			assert.True(t, rp.Put(ret), "expected Put to return true, got false")
			assert.Equal(t, uint64(1), rp.size, "expected size 1, got %v", rp.size)

			headVal := rp.Get()
			assert.Equal(t, ret, headVal, "expected resource %v at head of pool, got %v", ret, headVal)
		})
		t.Run("stale resource not returned", func(t *testing.T) {
			rp := initPool(t, 1, alwaysExpired, closeRsrc, initRsrc, time.Minute)
			ret := &rsrc{}
			assert.False(t, rp.Put(ret), "expected Put to return false, got true")
		})
	})
	t.Run("Prune", func(t *testing.T) {
		t.Run("removes all stale resources", func(t *testing.T) {
			ec := newExpiredCounter(3)
			rp := initPool(t, 0, ec.expired, ec.close, initRsrc, time.Minute)
			for i := 0; i < 5; i++ {
				ret := &rsrc{}
				_ = rp.Put(ret)
			}

			rp.Maintain()
			assert.Equal(t, uint64(2), rp.size, "expected size 2, got %v", rp.size)

			expiredCalled := ec.getExpiredCalled()
			assert.Equal(t, int32(7), expiredCalled, "expected expire to be called 7 times, got %v", expiredCalled)
			closeCalled := ec.getCloseCalled()
			assert.Equal(t, int32(3), closeCalled, "expected close to be called 3 times, got %v", closeCalled)
		})
	})
	t.Run("Background cleanup", func(t *testing.T) {
		t.Run("runs once every interval", func(t *testing.T) {
			ec := newExpiredCounter(3)
			dur := 100 * time.Millisecond
			rp := initPool(t, 0, neverExpired, ec.close, initRsrc, dur)
			rp.maintainTimer.Stop()

			for i := 0; i < 5; i++ {
				ret := &rsrc{}
				_ = rp.Put(ret)
			}

			rp.expiredFn = ec.expired
			rp.maintainTimer.Reset(dur)

			select {
			case <-ec.closeChan:
			case <-time.After(5 * time.Second):
				t.Fatalf("value was not read on closeChan after 5 seconds")
			}

			expiredCalled := ec.getExpiredCalled()
			assert.Equal(t, int32(5), expiredCalled, "expected expire to be called 5 times, got %v", expiredCalled)
			closeCalled := ec.getCloseCalled()
			assert.Equal(t, int32(3), closeCalled, "expected close to be called 3 times, got %v", closeCalled)
		})
	})
}
