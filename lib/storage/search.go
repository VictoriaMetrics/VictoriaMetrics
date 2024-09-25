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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
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

// MetricBlock is a time series block for a single metric.
type MetricBlock struct {
	// MetricName is metric name for the given Block.
	MetricName []byte

	// Block is a block for the given MetricName
	Block Block
}

// Marshal marshals MetricBlock to dst
func (mb *MetricBlock) Marshal(dst []byte) []byte {
	dst = encoding.MarshalBytes(dst, mb.MetricName)
	return MarshalBlock(dst, &mb.Block)
}

// CopyFrom copies src to mb.
func (mb *MetricBlock) CopyFrom(src *MetricBlock) {
	mb.MetricName = append(mb.MetricName[:0], src.MetricName...)
	mb.Block.CopyFrom(&src.Block)
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
	mb.Block.Reset()
	mn, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal MetricName")
	}
	src = src[nSize:]
	mb.MetricName = append(mb.MetricName[:0], mn...)

	return UnmarshalBlock(&mb.Block, src)
}

// UnmarshalBlock unmarshal Block from src to dst.
//
// dst.UnmarshalData isn't called on the block.
func UnmarshalBlock(dst *Block, src []byte) ([]byte, error) {
	tail, err := dst.bh.Unmarshal(src)
	if err != nil {
		return tail, fmt.Errorf("cannot unmarshal blockHeader: %w", err)
	}
	src = tail

	tds, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return tail, fmt.Errorf("cannot unmarshal timestampsData")
	}
	src = src[nSize:]
	dst.timestampsData = append(dst.timestampsData[:0], tds...)

	vd, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return tail, fmt.Errorf("cannot unmarshal valuesData")
	}
	src = src[nSize:]
	dst.valuesData = append(dst.valuesData[:0], vd...)

	return src, nil
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
	if err == nil && len(metricIDs) > 0 && len(tfss) > 0 {
		accountID := tfss[0].accountID
		projectID := tfss[0].projectID
		tsids, err = s.idb.getTSIDsFromMetricIDs(qt, accountID, projectID, metricIDs, deadline)
		if err == nil {
			err = storage.prefetchMetricNames(qt, accountID, projectID, metricIDs, deadline)
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
			s.MetricBlockRef.MetricName, ok = s.idb.searchMetricNameWithCache(s.MetricBlockRef.MetricName[:0], tsid.MetricID, tsid.AccountID, tsid.ProjectID)
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
	AccountID uint32
	ProjectID uint32

	TenantTokens  []TenantToken
	IsMultiTenant bool

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
func NewSearchQuery(accountID, projectID uint32, start, end int64, tagFilterss [][]TagFilter, maxMetrics int) *SearchQuery {
	if start < 0 {
		// This is needed for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
		start = 0
	}
	if maxMetrics <= 0 {
		maxMetrics = 2e9
	}
	return &SearchQuery{
		MinTimestamp: start,
		MaxTimestamp: end,
		TagFilterss:  tagFilterss,
		MaxMetrics:   maxMetrics,
		TenantTokens: []TenantToken{
			{
				AccountID: accountID,
				ProjectID: projectID,
			},
		},
	}
}

// TenantToken represents a tenant (accountID, projectID) pair.
type TenantToken struct {
	AccountID uint32
	ProjectID uint32
}

// String returns string representation of t.
func (t *TenantToken) String() string {
	return fmt.Sprintf("{accountID=%d, projectID=%d}", t.AccountID, t.ProjectID)
}

// Marshal appends marshaled t to dst and returns the result.
func (t *TenantToken) Marshal(dst []byte) []byte {
	dst = encoding.MarshalUint32(dst, t.AccountID)
	dst = encoding.MarshalUint32(dst, t.ProjectID)
	return dst
}

// NewMultiTenantSearchQuery creates new search query for the given args.
func NewMultiTenantSearchQuery(tenants []TenantToken, start, end int64, tagFilterss [][]TagFilter, maxMetrics int) *SearchQuery {
	if start < 0 {
		// This is needed for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5553
		start = 0
	}
	if maxMetrics <= 0 {
		maxMetrics = 2e9
	}
	return &SearchQuery{
		TenantTokens:  tenants,
		MinTimestamp:  start,
		MaxTimestamp:  end,
		TagFilterss:   tagFilterss,
		MaxMetrics:    maxMetrics,
		IsMultiTenant: true,
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
	value := stringsutil.LimitStringLen(string(tf.Value), 60)
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
	k, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal Key")
	}
	src = src[nSize:]
	tf.Key = append(tf.Key[:0], k...)

	v, nSize := encoding.UnmarshalBytes(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal Value")
	}
	src = src[nSize:]
	tf.Value = append(tf.Value[:0], v...)

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
	if !sq.IsMultiTenant {
		return fmt.Sprintf("accountID=%d, projectID=%d, filters=%s, timeRange=[%s..%s]", sq.AccountID, sq.ProjectID, a, start, end)
	}

	tts := make([]string, len(sq.TenantTokens))
	for i, tt := range sq.TenantTokens {
		tts[i] = tt.String()
	}
	return fmt.Sprintf("tenants=[%s], filters=%s, timeRange=[%s..%s]", strings.Join(tts, ","), a, start, end)
}

func tagFiltersToString(tfs []TagFilter) string {
	a := make([]string, len(tfs))
	for i, tf := range tfs {
		a[i] = tf.String()
	}
	return "{" + strings.Join(a, ",") + "}"
}

// MarshaWithoutTenant appends marshaled sq without AccountID/ProjectID to dst and returns the result.
// It is expected that TenantToken is already marshaled to dst.
func (sq *SearchQuery) MarshaWithoutTenant(dst []byte) []byte {
	dst = encoding.MarshalVarInt64(dst, sq.MinTimestamp)
	dst = encoding.MarshalVarInt64(dst, sq.MaxTimestamp)
	dst = encoding.MarshalVarUint64(dst, uint64(len(sq.TagFilterss)))
	for _, tagFilters := range sq.TagFilterss {
		dst = encoding.MarshalVarUint64(dst, uint64(len(tagFilters)))
		for i := range tagFilters {
			dst = tagFilters[i].Marshal(dst)
		}
	}
	dst = encoding.MarshalUint32(dst, uint32(sq.MaxMetrics))
	return dst
}

// Unmarshal unmarshals sq from src and returns the tail.
func (sq *SearchQuery) Unmarshal(src []byte) ([]byte, error) {
	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal AccountID: too short src len: %d; must be at least %d bytes", len(src), 4)
	}
	sq.AccountID = encoding.UnmarshalUint32(src)
	src = src[4:]

	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal ProjectID: too short src len: %d; must be at least %d bytes", len(src), 4)
	}
	sq.ProjectID = encoding.UnmarshalUint32(src)
	src = src[4:]

	minTs, nSize := encoding.UnmarshalVarInt64(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal MinTimestamp from varint")
	}
	src = src[nSize:]
	sq.MinTimestamp = minTs

	maxTs, nSize := encoding.UnmarshalVarInt64(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal MaxTimestamp from varint")
	}
	src = src[nSize:]
	sq.MaxTimestamp = maxTs

	tfssCount, nSize := encoding.UnmarshalVarUint64(src)
	if nSize <= 0 {
		return src, fmt.Errorf("cannot unmarshal the count of TagFilterss from uvarint")
	}
	src = src[nSize:]
	sq.TagFilterss = slicesutil.SetLength(sq.TagFilterss, int(tfssCount))

	for i := 0; i < int(tfssCount); i++ {
		tfsCount, nSize := encoding.UnmarshalVarUint64(src)
		if nSize <= 0 {
			return src, fmt.Errorf("cannot unmarshal the count of TagFilters from uvarint")
		}
		src = src[nSize:]

		tagFilters := sq.TagFilterss[i]
		tagFilters = slicesutil.SetLength(tagFilters, int(tfsCount))
		for j := 0; j < int(tfsCount); j++ {
			tail, err := tagFilters[j].Unmarshal(src)
			if err != nil {
				return tail, fmt.Errorf("cannot unmarshal TagFilter #%d: %w", j, err)
			}
			src = tail
		}
		sq.TagFilterss[i] = tagFilters
	}

	if len(src) < 4 {
		return src, fmt.Errorf("cannot unmarshal MaxMetrics: too short src len: %d; must be at least %d bytes", len(src), 4)
	}
	sq.MaxMetrics = int(encoding.UnmarshalUint32(src))
	src = src[4:]

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
