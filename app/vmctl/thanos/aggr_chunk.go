package thanos

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

// ChunkEncAggr is the top level encoding byte for the AggrChunk.
// It is defined by Thanos as 0xff to prevent collisions with Prometheus encodings.
const ChunkEncAggr = chunkenc.Encoding(0xff)

// AggrType represents an aggregation type in Thanos downsampled blocks.
type AggrType uint8

// Valid aggregation types matching Thanos definitions.
const (
	AggrCount AggrType = iota
	AggrSum
	AggrMin
	AggrMax
	AggrCounter
)

func (t AggrType) String() string {
	switch t {
	case AggrCount:
		return "count"
	case AggrSum:
		return "sum"
	case AggrMin:
		return "min"
	case AggrMax:
		return "max"
	case AggrCounter:
		return "counter"
	}
	return "<unknown>"
}

// ParseAggrType parses aggregate type from string.
func ParseAggrType(s string) (AggrType, error) {
	switch s {
	case "count":
		return AggrCount, nil
	case "sum":
		return AggrSum, nil
	case "min":
		return AggrMin, nil
	case "max":
		return AggrMax, nil
	case "counter":
		return AggrCounter, nil
	}
	return 0, fmt.Errorf("unknown aggregate type: %q", s)
}

// ErrAggrNotExist is returned if a requested aggregation is not present in an AggrChunk.
var ErrAggrNotExist = errors.New("aggregate does not exist")

// AggrChunk is a chunk that is composed of a set of aggregates for the same underlying data.
// Not all aggregates must be present.
// This is a read-only implementation for decoding Thanos downsampled blocks.
type AggrChunk []byte

// IsAggrChunk checks if the encoding byte indicates this is an AggrChunk.
func IsAggrChunk(enc chunkenc.Encoding) bool {
	return enc == ChunkEncAggr
}

// Get returns the sub-chunk for the given aggregate type if it exists.
func (c AggrChunk) Get(t AggrType) (chunkenc.Chunk, error) {
	b := c[:]
	var x []byte

	for i := AggrType(0); i <= t; i++ {
		l, n := binary.Uvarint(b)
		if n < 1 {
			return nil, errors.New("invalid size: failed to read uvarint")
		}
		if len(b[n:]) < int(l)+1 && l > 0 {
			return nil, errors.New("invalid size: not enough bytes")
		}
		b = b[n:]
		// If length is set to zero explicitly, that means the aggregate is unset.
		if l == 0 {
			if i == t {
				return nil, ErrAggrNotExist
			}
			continue
		}
		x = b[:int(l)+1]
		b = b[int(l)+1:]
	}
	if len(x) == 0 {
		return nil, ErrAggrNotExist
	}
	return chunkenc.FromData(chunkenc.Encoding(x[0]), x[1:])
}

// Encoding returns the encoding type for AggrChunk.
func (c AggrChunk) Encoding() chunkenc.Encoding {
	return ChunkEncAggr
}

// AggrChunkIterator iterates over samples from a specific aggregate within an AggrChunk.
type AggrChunkIterator struct {
	chunk    AggrChunk
	aggrType AggrType
	it       chunkenc.Iterator
	err      error
}

// NewAggrChunkIterator creates a new iterator for the specified aggregate type.
func NewAggrChunkIterator(data []byte, aggrType AggrType) *AggrChunkIterator {
	aci := &AggrChunkIterator{
		chunk:    AggrChunk(data),
		aggrType: aggrType,
	}

	subChunk, err := aci.chunk.Get(aggrType)
	if err != nil {
		aci.err = err
		return aci
	}

	aci.it = subChunk.Iterator(nil)
	return aci
}

// Next advances the iterator and returns the next value type.
func (it *AggrChunkIterator) Next() chunkenc.ValueType {
	if it.err != nil || it.it == nil {
		return chunkenc.ValNone
	}
	return it.it.Next()
}

// At returns the current timestamp and value.
func (it *AggrChunkIterator) At() (int64, float64) {
	if it.it == nil {
		return 0, 0
	}
	return it.it.At()
}

// Err returns any error encountered during iteration.
func (it *AggrChunkIterator) Err() error {
	if it.err != nil {
		return it.err
	}
	if it.it != nil {
		return it.it.Err()
	}
	return nil
}

// Seek advances the iterator to the first sample with timestamp >= t.
func (it *AggrChunkIterator) Seek(t int64) chunkenc.ValueType {
	if it.err != nil || it.it == nil {
		return chunkenc.ValNone
	}
	return it.it.Seek(t)
}

// AtHistogram returns histogram value (not supported for AggrChunk).
func (it *AggrChunkIterator) AtHistogram(h *histogram.Histogram) (int64, *histogram.Histogram) {
	return 0, nil
}

// AtFloatHistogram returns float histogram value (not supported for AggrChunk).
func (it *AggrChunkIterator) AtFloatHistogram(fh *histogram.FloatHistogram) (int64, *histogram.FloatHistogram) {
	return 0, nil
}

// AtT returns only the timestamp of the current sample.
func (it *AggrChunkIterator) AtT() int64 {
	if it.it == nil {
		return 0
	}
	return it.it.AtT()
}

// AggrChunkWrapper wraps AggrChunk to implement chunkenc.Chunk interface.
// It delegates iteration to a specific aggregate type.
type AggrChunkWrapper struct {
	data     []byte
	aggrType AggrType
}

// NewAggrChunkWrapper creates a new AggrChunk wrapper for the specified aggregate type.
func NewAggrChunkWrapper(data []byte, aggrType AggrType) *AggrChunkWrapper {
	return &AggrChunkWrapper{
		data:     data,
		aggrType: aggrType,
	}
}

// Bytes returns the underlying byte slice.
func (c *AggrChunkWrapper) Bytes() []byte {
	return c.data
}

// Encoding returns the AggrChunk encoding.
func (c *AggrChunkWrapper) Encoding() chunkenc.Encoding {
	return ChunkEncAggr
}

// Appender returns an error since AggrChunk is read-only.
func (c *AggrChunkWrapper) Appender() (chunkenc.Appender, error) {
	return nil, errors.New("AggrChunk is read-only")
}

// Iterator returns an iterator for the specified aggregate type.
func (c *AggrChunkWrapper) Iterator(it chunkenc.Iterator) chunkenc.Iterator {
	return NewAggrChunkIterator(c.data, c.aggrType)
}

// NumSamples returns the number of samples in the aggregate.
func (c *AggrChunkWrapper) NumSamples() int {
	chunk := AggrChunk(c.data)
	subChunk, err := chunk.Get(c.aggrType)
	if err != nil {
		return 0
	}
	return subChunk.NumSamples()
}

// Compact is a no-op for read-only AggrChunk.
func (c *AggrChunkWrapper) Compact() {}

// Reset resets the chunk with new data.
func (c *AggrChunkWrapper) Reset(stream []byte) {
	c.data = stream
}

// AggrChunkPool is a custom Pool that understands AggrChunk encoding (0xff).
// It delegates standard encodings to the default pool and handles AggrChunk specially.
type AggrChunkPool struct {
	defaultPool chunkenc.Pool
	aggrType    AggrType
}

// NewAggrChunkPool creates a new pool that handles AggrChunk encoding.
func NewAggrChunkPool(aggrType AggrType) *AggrChunkPool {
	return &AggrChunkPool{
		defaultPool: chunkenc.NewPool(),
		aggrType:    aggrType,
	}
}

// Get returns a chunk for the given encoding and data.
func (p *AggrChunkPool) Get(e chunkenc.Encoding, b []byte) (chunkenc.Chunk, error) {
	if e == ChunkEncAggr {
		return NewAggrChunkWrapper(b, p.aggrType), nil
	}
	return p.defaultPool.Get(e, b)
}

// Put returns a chunk to the pool.
func (p *AggrChunkPool) Put(c chunkenc.Chunk) error {
	if c.Encoding() == ChunkEncAggr {
		// AggrChunk wrappers are not pooled
		return nil
	}
	return p.defaultPool.Put(c)
}
