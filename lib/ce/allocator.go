package ce

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"sync"

	"github.com/VictoriaMetrics/metrics"
	"github.com/axiomhq/hyperloglog"
)

var (
	hllsCreatedTotal = metrics.NewCounter("vm_ce_hlls_created_total")
)

var (
	ERROR_MAX_HLLS_INUSE error = fmt.Errorf("HLL allocation would exceed maxHLLsInUse")
)

// Allocator provides a centralized way to allocate and track HyperLogLog sketches
// with built-in usage tracking and limits.
type Allocator struct {
	lock sync.Mutex

	max uint64

	inuse   uint64
	created uint64
}

// NewAllocator creates a new HLL allocator with the specified maximum HLLs in use.
func NewAllocator(max uint64) *Allocator {
	alloc := &Allocator{
		max:     max,
		inuse:   0,
		created: 0,
	}
	return alloc
}

// Allocate creates a new HyperLogLog sketch and tracks it in the allocator.
// Returns nil if the maximum number of HLLs in use would be exceeded.
func (alloc *Allocator) Allocate() (*hyperloglog.Sketch, error) {
	alloc.lock.Lock()
	defer alloc.lock.Unlock()

	if alloc.inuse >= alloc.max {
		return nil, ERROR_MAX_HLLS_INUSE
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

	alloc.created++
	alloc.inuse++
	hllsCreatedTotal.Inc()

	return hll, nil
}

// Inuse returns the current number of HLLs in use.
func (alloc *Allocator) Inuse() uint64 {
	alloc.lock.Lock()
	defer alloc.lock.Unlock()

	return alloc.inuse
}

// Created returns the total number of HLLs created since allocator creation.
func (alloc *Allocator) Created() uint64 {
	alloc.lock.Lock()
	defer alloc.lock.Unlock()

	return alloc.created
}

// Max returns the maximum number of HLLs that can be in use.
func (alloc *Allocator) Max() uint64 {
	alloc.lock.Lock()
	defer alloc.lock.Unlock()

	return alloc.max
}

// Can be called concurrently.
func (alloc *Allocator) Merge(other *Allocator) {
	alloc.lock.Lock()
	defer alloc.lock.Unlock()

	alloc.inuse = max(alloc.inuse, other.inuse)
	alloc.created += other.created
	alloc.max = max(alloc.max, other.max)
}

// GobEncode implements gob.GobEncoder to allow Allocator to be encoded
// without exporting its fields.
func (alloc *Allocator) GobEncode() ([]byte, error) {
	alloc.lock.Lock()
	defer alloc.lock.Unlock()

	// Create an anonymous struct with exported fields for gob encoding
	anon := struct {
		Max     uint64
		Inuse   uint64
		Created uint64
	}{
		Max:     alloc.max,
		Inuse:   alloc.inuse,
		Created: alloc.created,
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
	alloc.lock.Lock()
	defer alloc.lock.Unlock()

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
	alloc.inuse = anon.Inuse
	alloc.created = anon.Created

	return nil
}
