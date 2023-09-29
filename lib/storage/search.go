package storage

import (
	"fmt"
	"io"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
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

// Init initializes br from pr and data
func (br *BlockRef) Init(pr PartRef, data []byte) error {
	br.p = pr.p
	tail, err := br.bh.Unmarshal(data)
	if err != nil {
		return err
	}
	if len(tail) > 0 {
		return fmt.Errorf("unexpected non-empty tail left after unmarshaling blockHeader; len(tail)=%d; tail=%q", len(tail), tail)
	}
	return nil
}

// Marshal marshals br to dst.
func (br *BlockRef) Marshal(dst []byte) []byte {
	return br.bh.Marshal(dst)
}

// RowsCount returns the number of rows in br.
func (br *BlockRef) RowsCount() int {
	return int(br.bh.RowsCount)
}

// PartRef returns PartRef from br.
func (br *BlockRef) PartRef() PartRef {
	return PartRef{
		p: br.p,
	}
}

// PartRef is Part reference.
type PartRef struct {
	p *part
}

// MustReadBlock reads block from br to dst.
func (br *BlockRef) MustReadBlock(dst *Block) {
	dst.Reset()
	dst.bh = br.bh

	dst.timestampsData = bytesutil.ResizeNoCopyMayOverallocate(dst.timestampsData, int(br.bh.TimestampsBlockSize))
	br.p.timestampsFile.MustReadAt(dst.timestampsData, int64(br.bh.TimestampsBlockOffset))

	dst.valuesData = bytesutil.ResizeNoCopyMayOverallocate(dst.valuesData, int(br.bh.ValuesBlockSize))
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

	// idb is used for MetricName lookup for the found data blocks.
	idb *indexDB

	// retentionDeadline is used for filtering out blocks outside the configured retention.
	retentionDeadline int64

	ts tableSearch

	// tr contains time range used in the search.
	tr TimeRange

	// tfss contains tag filters used in the search.
	tfss []*TagFilters

	// deadline in unix timestamp seconds for the current search.
	deadline uint64

	err error

	needClosing bool

	loops int

	prevMetricID uint64
}

func (s *Search) reset() {
	s.MetricBlockRef.MetricName = s.MetricBlockRef.MetricName[:0]
	s.MetricBlockRef.BlockRef = nil

	s.idb = nil
	s.retentionDeadline = 0
	s.ts.reset()
	s.tr = TimeRange{}
	s.tfss = nil
	s.deadline = 0
	s.err = nil
	s.needClosing = false
	s.loops = 0
	s.prevMetricID = 0
}

// Init initializes s from the given storage, tfss and tr.
//
// MustClose must be called when the search is done.
//
// Init returns the upper bound on the number of found time series.
func (s *Search) Init(qt *querytracer.Tracer, storage *Storage, tfss []*TagFilters, tr TimeRange, maxMetrics int, deadline uint64) int {
	qt = qt.NewChild("init series search: filters=%s, timeRange=%s", tfss, &tr)
	defer qt.Done()
	if s.needClosing {
		logger.Panicf("BUG: missing MustClose call before the next call to Init")
	}
	retentionDeadline := int64(fasttime.UnixTimestamp()*1e3) - storage.retentionMsecs

	s.reset()
	s.idb = storage.idb()
	s.retentionDeadline = retentionDeadline
	s.tr = tr
	s.tfss = tfss
	s.deadline = deadline
	s.needClosing = true

	var tsids []TSID
	metricIDs, err := s.idb.searchMetricIDs(qt, tfss, tr, maxMetrics, deadline)
	if err == nil {
		tsids, err = s.idb.getTSIDsFromMetricIDs(qt, metricIDs, deadline)
		if err == nil {
			err = storage.prefetchMetricNames(qt, metricIDs, deadline)
		}
	}
	// It is ok to call Init on non-nil err.
	// Init must be called before returning because it will fail
	// on Search.MustClose otherwise.
	s.ts.Init(storage.tb, tsids, tr)
	qt.Printf("search for parts with data for %d series", len(tsids))
	if err != nil {
		s.err = err
		return 0
	}
	return len(tsids)
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
	if s.err == io.EOF || s.err == nil {
		return nil
	}
	return fmt.Errorf("error when searching for tagFilters=%s on the time range %s: %w", s.tfss, s.tr.String(), s.err)
}

// NextMetricBlock proceeds to the next MetricBlockRef.
func (s *Search) NextMetricBlock() bool {
	if s.err != nil {
		return false
	}
	for s.ts.NextBlock() {
		if s.loops&paceLimiterSlowIterationsMask == 0 {
			if err := checkSearchDeadlineAndPace(s.deadline); err != nil {
				s.err = err
				return false
			}
		}
		s.loops++
		tsid := &s.ts.BlockRef.bh.TSID
		if tsid.MetricID != s.prevMetricID {
			if s.ts.BlockRef.bh.MaxTimestamp < s.retentionDeadline {
				// Skip the block, since it contains only data outside the configured retention.
				continue
			}
			var ok bool
			s.MetricBlockRef.MetricName, ok = s.idb.searchMetricNameWithCache(s.MetricBlockRef.MetricName[:0], tsid.MetricID)
			if !ok {
				// Skip missing metricName for tsid.MetricID.
				// It should be automatically fixed. See indexDB.searchMetricNameWithCache for details.
				continue
			}
			s.prevMetricID = tsid.MetricID
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
	// The time range for searching time series
	MinTimestamp int64
	MaxTimestamp int64

	// Tag filters for the search query
	TagFilterss [][]TagFilter

	// The maximum number of time series the search query can return.
	MaxMetrics int
}

// GetTimeRange returns time range for the given sq.
func (sq *SearchQuery) GetTimeRange() TimeRange {
	return TimeRange{
		MinTimestamp: sq.MinTimestamp,
		MaxTimestamp: sq.MaxTimestamp,
	}
}

// NewSearchQuery creates new search query for the given args.
func NewSearchQuery(start, end int64, tagFilterss [][]TagFilter, maxMetrics int) *SearchQuery {
	if maxMetrics <= 0 {
		maxMetrics = 2e9
	}
	return &SearchQuery{
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
		MaxMetrics:   maxMetrics,
	}
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
	op := tf.getOp()
	value := bytesutil.LimitStringLen(string(tf.Value), 60)
	if len(tf.Key) == 0 {
		return fmt.Sprintf("__name__%s%q", op, value)
	}
	return fmt.Sprintf("%s%s%q", tf.Key, op, value)
}

func (tf *TagFilter) getOp() string {
	if tf.IsNegative {
		if tf.IsRegexp {
			return "!~"
		}
		return "!="
	}
	if tf.IsRegexp {
		return "=~"
	}
	return "="
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
	a := make([]string, len(sq.TagFilterss))
	for i, tfs := range sq.TagFilterss {
		a[i] = tagFiltersToString(tfs)
	}
	start := TimestampToHumanReadableFormat(sq.MinTimestamp)
	end := TimestampToHumanReadableFormat(sq.MaxTimestamp)
	return fmt.Sprintf("filters=%s, timeRange=[%s..%s]", a, start, end)
}

func tagFiltersToString(tfs []TagFilter) string {
	a := make([]string, len(tfs))
	for i, tf := range tfs {
		a[i] = tf.String()
	}
	return "{" + strings.Join(a, ",") + "}"
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

func checkSearchDeadlineAndPace(deadline uint64) error {
	if fasttime.UnixTimestamp() > deadline {
		return ErrDeadlineExceeded
	}
	return nil
}

const (
	paceLimiterFastIterationsMask   = 1<<16 - 1
	paceLimiterMediumIterationsMask = 1<<14 - 1
	paceLimiterSlowIterationsMask   = 1<<12 - 1
)
