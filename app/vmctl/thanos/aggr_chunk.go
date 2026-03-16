package thanos

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

// ChunkEncAggr is the top level encoding byte for the AggrChunk.
// It is defined by Thanos as 0xff to prevent collisions with Prometheus encodings.
const ChunkEncAggr = chunkenc.Encoding(0xff)

// AggrType represents an aggregation type in Thanos downsampled blocks.
type AggrType uint8

// AggrTypeNone indicates raw blocks with no aggregation.
// It is used as a sentinel to distinguish raw block processing from downsampled.
const AggrTypeNone AggrType = 255

// Valid aggregation types matching Thanos definitions.
const (
	AggrCount AggrType = iota
	AggrSum
	AggrMin
	AggrMax
	AggrCounter
)

// AllAggrTypes contains all supported aggregation types.
var AllAggrTypes = []AggrType{AggrCount, AggrSum, AggrMin, AggrMax, AggrCounter}

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
		if l > uint64(len(b[n:])) || l+1 > uint64(len(b[n:])) {
			if l > 0 {
				return nil, errors.New("invalid size: not enough bytes")
			}
		}
		b = b[n:]
		// If length is set to zero explicitly, that means the aggregate is unset.
		if l == 0 {
			if i == t {
				return nil, ErrAggrNotExist
			}
			continue
		}
		chunkLen := int(l) + 1
		x = b[:chunkLen]
		b = b[chunkLen:]
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

// errIterator wraps a nop iterator but reports an error via Err().
// It embeds chunkenc.Iterator to inherit all methods (including Seek)
// which avoids go vet stdmethods warning about Seek signature.
type errIterator struct {
	chunkenc.Iterator
	err error
}

// Err returns the underlying error.
func (it *errIterator) Err() error {
	return it.err
}

// newAggrChunkIterator creates a new iterator for the specified aggregate type.
// If the aggregate is not present in the chunk (ErrAggrNotExist), a nop iterator
// is returned without error — the caller will simply see zero samples.
// Real decoding/corruption errors are reported via the iterator's Err() method.
func newAggrChunkIterator(data []byte, aggrType AggrType) chunkenc.Iterator {
	chunk := AggrChunk(data)
	subChunk, err := chunk.Get(aggrType)
	if err != nil {
		if errors.Is(err, ErrAggrNotExist) {
			return chunkenc.NewNopIterator()
		}
		return &errIterator{
			Iterator: chunkenc.NewNopIterator(),
			err:      err,
		}
	}
	return subChunk.Iterator(nil)
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
	return newAggrChunkIterator(c.data, c.aggrType)
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
