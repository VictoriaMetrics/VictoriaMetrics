package ce

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
	"github.com/axiomhq/hyperloglog"
)

var (
	hllsCreatedTotal = metrics.NewCounter("ce_hlls_created_total")
)

var (
	ERROR_MAX_HLLS_INUSE error = fmt.Errorf("HLL allocation would exceed maxHLLsInUse")
)

// Allocator provides a centralized way to allocate and track HyperLogLog sketches
// with built-in usage tracking and limits.
type Allocator struct {
	max uint64

	inuse   atomic.Uint64
	created atomic.Uint64
}

// NewAllocator creates a new HLL allocator with the specified maximum HLLs in use.
func NewAllocator(max uint64) *Allocator {
	alloc := &Allocator{
		max:     max,
		inuse:   atomic.Uint64{},
		created: atomic.Uint64{},
	}
	return alloc
}

// Allocate creates a new HyperLogLog sketch and tracks it in the allocator.
// Returns nil if the maximum number of HLLs in use would be exceeded.
func (alloc *Allocator) Allocate() (*hyperloglog.Sketch, error) {
	// Use CAS loop to atomically check and reserve a slot to avoid TOCTOU race
	for {
		current := alloc.inuse.Load()
		if current >= alloc.max {
			return nil, ERROR_MAX_HLLS_INUSE
		}
		if alloc.inuse.CompareAndSwap(current, current+1) {
			break
		}
		// CAS failed, another goroutine modified inuse, retry
	}

	// Create a new HLL sketch with precision p=10 and useSparse=true.
	// 1. p determines the number of uint8 registers used in the sketch: m = 2^p.
	// 2. std error is approximately 1.04/sqrt(m). Higher p values increase accuracy but also memory usage and estimation compute time.
	//    Rule of thumb: 4x registers (p += 2) => 2x accuracy (1/2 std error) & 4x estimation compute time.
	// 3. useSparse=true enables the ability for the sketch to automatically switch between sparse and dense representations.
	//    Sparse representation is more memory efficient for small cardinalities, while dense is better for large cardinalities.
	//
	// We choose p=10 as a good trade-off between accuracy and resource usage:
	// - Memory usage: 2^10 = 1024 registers => 1KB in dense mode, and much less in sparse mode for small cardinalities.
	// - Accuracy: ~3.25% standard error, which is acceptable for many applications.
	// - Performance: reasonable estimation compute time. See benchmarks in allocator_test.go
	hll, err := hyperloglog.NewSketch(10, true)
	if err != nil {
		log.Panicf("BUG: failed to create HLL sketch: %v", err)
	}

	alloc.created.Add(1)
	hllsCreatedTotal.Inc()

	return hll, nil
}

// Inuse returns the current number of HLLs in use.
func (alloc *Allocator) Inuse() uint64 {
	return alloc.inuse.Load()
}

// Created returns the total number of HLLs created since allocator creation.
func (alloc *Allocator) Created() uint64 {
	return alloc.created.Load()
}

// Max returns the maximum number of HLLs that can be in use.
func (alloc *Allocator) Max() uint64 {
	return alloc.max
}

// Do not call concurrently.
func (alloc *Allocator) Merge(other *Allocator) {
	alloc.inuse.Store(max(alloc.inuse.Load(), other.inuse.Load()))
	alloc.created.Store(alloc.created.Load() + other.created.Load())
	alloc.max = max(alloc.max, other.max)
}

// GobEncode implements gob.GobEncoder to allow Allocator to be encoded
// without exporting its fields.
func (alloc *Allocator) GobEncode() ([]byte, error) {
	// Create an anonymous struct with exported fields for gob encoding
	anon := struct {
		Max     uint64
		Inuse   uint64
		Created uint64
	}{
		Max:     alloc.max,
		Inuse:   alloc.inuse.Load(),
		Created: alloc.created.Load(),
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(anon); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode implements gob.GobDecoder to allow Allocator to be decoded
// without exporting its fields.
func (alloc *Allocator) GobDecode(data []byte) error {
	// Create an anonymous struct with exported fields for gob decoding
	anon := struct {
		Max     uint64
		Inuse   uint64
		Created uint64
	}{}

	var buf bytes.Buffer
	buf.Write(data)
	if err := gob.NewDecoder(&buf).Decode(&anon); err != nil {
		return err
	}

	// Set the unexported fields
	alloc.max = anon.Max
	alloc.inuse.Store(anon.Inuse)
	alloc.created.Store(anon.Created)

	return nil
}
