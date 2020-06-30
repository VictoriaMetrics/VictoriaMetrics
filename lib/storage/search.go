package storage

import (
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// BlockRef references a Block.
//
// BlockRef is valid only until the corresponding Search is valid,
// i.e. it becomes invalid after Search.MustClose is called.
type BlockRef struct {
	p  *part
	bh blockHeader
}

func (br *BlockRef) reset() {
	br.p = nil
	br.bh = blockHeader{}
}

func (br *BlockRef) init(p *part, bh *blockHeader) {
	br.p = p
	br.bh = *bh
}

// MustReadBlock reads block from br to dst.
//
// if fetchData is false, then only block header is read, otherwise all the data is read.
func (br *BlockRef) MustReadBlock(dst *Block, fetchData bool) {
	dst.Reset()
	dst.bh = br.bh
	if !fetchData {
		return
	}

	dst.timestampsData = bytesutil.Resize(dst.timestampsData[:0], int(br.bh.TimestampsBlockSize))
	br.p.timestampsFile.MustReadAt(dst.timestampsData, int64(br.bh.TimestampsBlockOffset))

	dst.valuesData = bytesutil.Resize(dst.valuesData[:0], int(br.bh.ValuesBlockSize))
	br.p.valuesFile.MustReadAt(dst.valuesData, int64(br.bh.ValuesBlockOffset))
}

// MetricBlockRef contains reference to time series block for a single metric.
type MetricBlockRef struct {
	// The metric name
	MetricName []byte

	// The block reference. Call BlockRef.MustReadBlock in order to obtain the block.
	BlockRef *BlockRef
}

// Search is a search for time series.
type Search struct {
	// MetricBlockRef is updated with each Search.NextMetricBlock call.
	MetricBlockRef MetricBlockRef

	storage *Storage

	ts tableSearch

	err error

	needClosing bool
}

func (s *Search) reset() {
	s.MetricBlockRef.MetricName = s.MetricBlockRef.MetricName[:0]
	s.MetricBlockRef.BlockRef = nil

	s.storage = nil
	s.ts.reset()
	s.err = nil
	s.needClosing = false
}

// Init initializes s from the given storage, tfss and tr.
//
// MustClose must be called when the search is done.
func (s *Search) Init(storage *Storage, tfss []*TagFilters, tr TimeRange, maxMetrics int) {
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
	s.ts.Init(storage.tb, tsids, tr)

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

// NextMetricBlock proceeds to the next MetricBlockRef.
func (s *Search) NextMetricBlock() bool {
	if s.err != nil {
		return false
	}
	for s.ts.NextBlock() {
		tsid := &s.ts.BlockRef.bh.TSID
		var err error
		s.MetricBlockRef.MetricName, err = s.storage.searchMetricName(s.MetricBlockRef.MetricName[:0], tsid.MetricID)
		if err != nil {
			if err == io.EOF {
				// Skip missing metricName for tsid.MetricID.
				// It should be automatically fixed. See indexDB.searchMetricName for details.
				continue
			}
			s.err = err
			return false
		}
		s.MetricBlockRef.BlockRef = s.ts.BlockRef
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
		return tail, fmt.Errorf("cannot unmarshal Key: %w", err)
	}
	tf.Key = append(tf.Key[:0], k...)
	src = tail

	tail, v, err := encoding.UnmarshalBytes(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal Value: %w", err)
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
		return src, fmt.Errorf("cannot unmarshal MinTimestamp: %w", err)
	}
	sq.MinTimestamp = minTs
	src = tail

	tail, maxTs, err := encoding.UnmarshalVarInt64(src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal MaxTimestamp: %w", err)
	}
	sq.MaxTimestamp = maxTs
	src = tail

	tail, tfssCount, err := encoding.UnmarshalVarUint64(src)
	if err != nil {
		return src, fmt.Errorf("cannot unmarshal the count of TagFilterss: %w", err)
	}
	if n := int(tfssCount) - cap(sq.TagFilterss); n > 0 {
		sq.TagFilterss = append(sq.TagFilterss[:cap(sq.TagFilterss)], make([][]TagFilter, n)...)
	}
	sq.TagFilterss = sq.TagFilterss[:tfssCount]
	src = tail

	for i := 0; i < int(tfssCount); i++ {
		tail, tfsCount, err := encoding.UnmarshalVarUint64(src)
		if err != nil {
			return src, fmt.Errorf("cannot unmarshal the count of TagFilters: %w", err)
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
				return tail, fmt.Errorf("cannot unmarshal TagFilter #%d: %w", j, err)
			}
			src = tail
		}
		sq.TagFilterss[i] = tagFilters
	}

	return src, nil
}
