package storage

import (
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// MetricBlock is a time series block for a single metric.
type MetricBlock struct {
	MetricName []byte

	Block *Block
}

// Marshal marshals MetricBlock to dst
func (mb *MetricBlock) Marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, mb.MetricName)
	return MarshalBlock(dst, mb.Block)
}

// MarshalBlock marshals b to dst.
//
// b.MarshalData must be called on b before calling MarshalBlock.
func MarshalBlock(dst []byte, b *Block) []byte {
	dst = b.bh.Marshal(dst)
	dst = encoding.MarshalBytes(dst, b.timestampsData)
	dst = encoding.MarshalBytes(dst, b.valuesData)
	return dst
}

// Unmarshal unmarshals MetricBlock from src
func (mb *MetricBlock) Unmarshal(src []byte) ([]byte, error) {
	if mb.Block == nil {
		logger.Panicf("BUG: MetricBlock.Block must be non-nil when calling Unmarshal!")
	} else {
		mb.Block.Reset()
	}
	tail, mn, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal MetricName: %s", err)
	}
	mb.MetricName = append(mb.MetricName[:0], mn...)
	src = tail

	return UnmarshalBlock(mb.Block, src)
}

// UnmarshalBlock unmarshal Block from src to dst.
//
// dst.UnmarshalData isn't called on the block.
func UnmarshalBlock(dst *Block, src []byte) ([]byte, error) {
	tail, err := dst.bh.Unmarshal(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal blockHeader: %s", err)
	}
	src = tail

	tail, tds, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal timestampsData: %s", err)
	}
	dst.timestampsData = append(dst.timestampsData[:0], tds...)
	src = tail

	tail, vd, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal valuesData: %s", err)
	}
	dst.valuesData = append(dst.valuesData[:0], vd...)
	src = tail

	return src, nil
}

// Search is a search for time series.
type Search struct {
	// MetricBlock is updated with each Search.NextMetricBlock call.
	MetricBlock MetricBlock

	storage *Storage

	ts tableSearch

	err error

	needClosing bool
}

func (s *Search) reset() {
	s.MetricBlock.MetricName = s.MetricBlock.MetricName[:0]
	s.MetricBlock.Block = nil

	s.storage = nil
	s.ts.reset()
	s.err = nil
	s.needClosing = false
}

// Init initializes s from the given storage, tfss and tr.
//
// MustClose must be called when the search is done.
func (s *Search) Init(storage *Storage, tfss []*TagFilters, tr TimeRange, fetchData bool, maxMetrics int) {
	if s.needClosing {
		logger.Panicf("BUG: missing MustClose call before the next call to Init")
	}

	s.reset()
	s.needClosing = true

	tsids, err := storage.searchTSIDs(tfss, tr, maxMetrics)
	if err == nil {
		err = storage.prefetchMetricNames(tsids)
	}
	// It is ok to call Init on error from storage.searchTSIDs.
	// Init must be called before returning because it will fail
	// on Seach.MustClose otherwise.
	s.ts.Init(storage.tb, tsids, tr, fetchData)

	if err != nil {
		s.err = err
		return
	}

	s.storage = storage
}

// MustClose closes the Search.
func (s *Search) MustClose() {
	if !s.needClosing {
		logger.Panicf("BUG: missing Init call before MustClose")
	}
	s.ts.MustClose()
	s.reset()
}

// Error returns the last error from s.
func (s *Search) Error() error {
	if s.err == io.EOF {
		return nil
	}
	return s.err
}

// NextMetricBlock proceeds to the next MetricBlock.
func (s *Search) NextMetricBlock() bool {
	if s.err != nil {
		return false
	}
	for s.ts.NextBlock() {
		tsid := &s.ts.Block.bh.TSID
		var err error
		s.MetricBlock.MetricName, err = s.storage.searchMetricName(s.MetricBlock.MetricName[:0], tsid.MetricID)
		if err != nil {
			if err == io.EOF {
				// Skip missing metricName for tsid.MetricID.
				// It should be automatically fixed. See indexDB.searchMetricName for details.
				continue
			}
			s.err = err
			return false
		}
		s.MetricBlock.Block = s.ts.Block
		return true
	}
	if err := s.ts.Error(); err != nil {
		s.err = err
		return false
	}

	s.err = io.EOF
	return false
}

// SearchQuery is used for sending search queries from vmselect to vmstorage.
type SearchQuery struct {
	MinTimestamp int64
	MaxTimestamp int64
	TagFilterss  [][]TagFilter
}

// TagFilter represents a single tag filter from SearchQuery.
type TagFilter struct {
	Key        []byte
	Value      []byte
	IsNegative bool
	IsRegexp   bool
}

// String returns string representation of tf.
func (tf *TagFilter) String() string {
	var bb bytesutil.ByteBuffer
	fmt.Fprintf(&bb, "{Key=%q, Value=%q, IsNegative: %v, IsRegexp: %v}", tf.Key, tf.Value, tf.IsNegative, tf.IsRegexp)
	return string(bb.B)
}

// Marshal appends marshaled tf to dst and returns the result.
func (tf *TagFilter) Marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, tf.Key)
	dst = encoding.MarshalBytes(dst, tf.Value)

	x := 0
	if tf.IsNegative {
		x = 2
	}
	if tf.IsRegexp {
		x |= 1
	}
	dst = append(dst, byte(x))

	return dst
}

// Unmarshal unmarshals tf from src and returns the tail.
func (tf *TagFilter) Unmarshal(src []byte) ([]byte, error) {
	tail, k, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal Key: %s", err)
	}
	tf.Key = append(tf.Key[:0], k...)
	src = tail

	tail, v, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal Value: %s", err)
	}
	tf.Value = append(tf.Value[:0], v...)
	src = tail

	if len(src) < 1 {
		return src, fmt.Errorf("cannot unmarshal IsNegative+IsRegexp from empty src")
	}
	x := src[0]
	switch x {
	case 0:
		tf.IsNegative = false
		tf.IsRegexp = false
	case 1:
		tf.IsNegative = false
		tf.IsRegexp = true
	case 2:
		tf.IsNegative = true
		tf.IsRegexp = false
	case 3:
		tf.IsNegative = true
		tf.IsRegexp = true
	default:
		return src, fmt.Errorf("unexpected value for IsNegative+IsRegexp: %d; must be in the range [0..3]", x)
	}
	src = src[1:]

	return src, nil
}

// String returns string representation of the search query.
func (sq *SearchQuery) String() string {
	var bb bytesutil.ByteBuffer
	fmt.Fprintf(&bb, "MinTimestamp=%s, MaxTimestamp=%s, TagFilters=[\n",
		timestampToTime(sq.MinTimestamp), timestampToTime(sq.MaxTimestamp))
	for _, tagFilters := range sq.TagFilterss {
		for _, tf := range tagFilters {
			fmt.Fprintf(&bb, "%s", tf.String())
		}
		fmt.Fprintf(&bb, "\n")
	}
	fmt.Fprintf(&bb, "]")
	return string(bb.B)
}

// Marshal appends marshaled sq to dst and returns the result.
func (sq *SearchQuery) Marshal(dst []byte) []byte {
	dst = encoding.MarshalVarInt64(dst, sq.MinTimestamp)
	dst = encoding.MarshalVarInt64(dst, sq.MaxTimestamp)
	dst = encoding.MarshalVarUint64(dst, uint64(len(sq.TagFilterss)))
	for _, tagFilters := range sq.TagFilterss {
		dst = encoding.MarshalVarUint64(dst, uint64(len(tagFilters)))
		for i := range tagFilters {
			dst = tagFilters[i].Marshal(dst)
		}
	}
	return dst
}

// Unmarshal unmarshals sq from src and returns the tail.
func (sq *SearchQuery) Unmarshal(src []byte) ([]byte, error) {
	tail, minTs, err := encoding.UnmarshalVarInt64(src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal MinTimestamp: %s", err)
	}
	sq.MinTimestamp = minTs
	src = tail

	tail, maxTs, err := encoding.UnmarshalVarInt64(src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal MaxTimestamp: %s", err)
	}
	sq.MaxTimestamp = maxTs
	src = tail

	tail, tfssCount, err := encoding.UnmarshalVarUint64(src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal the count of TagFilterss: %s", err)
	}
	if n := int(tfssCount) - cap(sq.TagFilterss); n > 0 {
		sq.TagFilterss = append(sq.TagFilterss[:cap(sq.TagFilterss)], make([][]TagFilter, n)...)
	}
	sq.TagFilterss = sq.TagFilterss[:tfssCount]
	src = tail

	for i := 0; i < int(tfssCount); i++ {
		tail, tfsCount, err := encoding.UnmarshalVarUint64(src)
		if err != nil {
			return src, fmt.Errorf("cannot unmarshal the count of TagFilters: %s", err)
		}
		src = tail

		tagFilters := sq.TagFilterss[i]
		if n := int(tfsCount) - cap(tagFilters); n > 0 {
			tagFilters = append(tagFilters[:cap(tagFilters)], make([]TagFilter, n)...)
		}
		tagFilters = tagFilters[:tfsCount]
		for j := 0; j < int(tfsCount); j++ {
			tail, err := tagFilters[j].Unmarshal(src)
			if err != nil {
				return tail, fmt.Errorf("cannot unmarshal TagFilter #%d: %s", j, err)
			}
			src = tail
		}
		sq.TagFilterss[i] = tagFilters
	}

	return src, nil
}
