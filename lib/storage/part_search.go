package storage

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// partSearch represents blocks stream for the given search args
// passed to Init.
type partSearch struct {
	// BlockRef contains the reference to the found block after NextBlock call.
	BlockRef BlockRef

	// p is the part to search.
	p *part

	// tsids contains sorted tsids to search.
	tsids []TSID

	// tsidIdx points to the currently searched tsid in tsids.
	tsidIdx int

	// tr is a time range to search.
	tr TimeRange

	metaindex []metaindexRow

	ibCache *indexBlockCache

	bhs []blockHeader

	compressedIndexBuf []byte
	indexBuf           []byte

	err error
}

func (ps *partSearch) reset() {
	ps.BlockRef.reset()
	ps.p = nil
	ps.tsids = nil
	ps.tsidIdx = 0
	ps.metaindex = nil
	ps.ibCache = nil
	ps.bhs = nil
	ps.compressedIndexBuf = ps.compressedIndexBuf[:0]
	ps.indexBuf = ps.indexBuf[:0]
	ps.err = nil
}

var isInTest = func() bool {
	return strings.HasSuffix(os.Args[0], ".test")
}()

// Init initializes the ps with the given p, tsids and tr.
//
// tsids must be sorted.
// tsids cannot be modified after the Init call, since it is owned by ps.
func (ps *partSearch) Init(p *part, tsids []TSID, tr TimeRange) {
	ps.reset()
	ps.p = p

	if p.ph.MinTimestamp <= tr.MaxTimestamp && p.ph.MaxTimestamp >= tr.MinTimestamp {
		if isInTest && !sort.SliceIsSorted(tsids, func(i, j int) bool { return tsids[i].Less(&tsids[j]) }) {
			logger.Panicf("BUG: tsids must be sorted; got %+v", tsids)
		}
		// take ownership of of tsids.
		ps.tsids = tsids
	}
	ps.tr = tr
	ps.metaindex = p.metaindex
	ps.ibCache = p.ibCache

	// Advance to the first tsid. There is no need in checking
	// the returned result, since it will be checked in NextBlock.
	ps.nextTSID()
}

// NextBlock advances to the next BlockRef.
//
// Returns true on success.
//
// The blocks are sorted by (TDIS, MinTimestamp). Two subsequent blocks
// for the same TSID may contain overlapped time ranges.
func (ps *partSearch) NextBlock() bool {
	for {
		if ps.err != nil {
			return false
		}
		if len(ps.bhs) == 0 {
			if !ps.nextBHS() {
				return false
			}
		}
		if ps.searchBHS() {
			return true
		}
	}
}

// Error returns the last error.
func (ps *partSearch) Error() error {
	if ps.err == io.EOF {
		return nil
	}
	return ps.err
}

func (ps *partSearch) nextTSID() bool {
	if ps.tsidIdx >= len(ps.tsids) {
		ps.err = io.EOF
		return false
	}
	ps.BlockRef.bh.TSID = ps.tsids[ps.tsidIdx]
	ps.tsidIdx++
	return true
}

func (ps *partSearch) nextBHS() bool {
	for len(ps.metaindex) > 0 {
		// Optimization: skip tsid values smaller than the minimum value
		// from ps.metaindex.
		for ps.BlockRef.bh.TSID.Less(&ps.metaindex[0].TSID) {
			if !ps.nextTSID() {
				return false
			}
		}
		// Invariant: ps.BlockRef.bh.TSID >= ps.metaindex[0].TSID

		ps.metaindex = skipSmallMetaindexRows(ps.metaindex, &ps.BlockRef.bh.TSID)
		// Invariant: len(ps.metaindex) > 0 && ps.BlockRef.bh.TSID >= ps.metaindex[0].TSID

		mr := &ps.metaindex[0]
		ps.metaindex = ps.metaindex[1:]
		if ps.BlockRef.bh.TSID.Less(&mr.TSID) {
			logger.Panicf("BUG: invariant violation: ps.BlockRef.bh.TSID cannot be smaller than mr.TSID; got %+v vs %+v", &ps.BlockRef.bh.TSID, &mr.TSID)
		}

		if mr.MaxTimestamp < ps.tr.MinTimestamp {
			// Skip mr with too small timestamps.
			continue
		}
		if mr.MinTimestamp > ps.tr.MaxTimestamp {
			// Skip mr with too big timestamps.
			continue
		}

		// Found the index block which may contain the required data
		// for the ps.BlockRef.bh.TSID and the given timestamp range.
		indexBlockKey := mr.IndexBlockOffset
		ib := ps.ibCache.Get(indexBlockKey)
		if ib == nil {
			// Slow path - actually read and unpack the index block.
			var err error
			ib, err = ps.readIndexBlock(mr)
			if err != nil {
				ps.err = fmt.Errorf("cannot read index block for part %q at offset %d with size %d: %w",
					&ps.p.ph, mr.IndexBlockOffset, mr.IndexBlockSize, err)
				return false
			}
			ps.ibCache.Put(indexBlockKey, ib)
		}
		ps.bhs = ib.bhs
		return true
	}

	// No more metaindex rows to search.
	ps.err = io.EOF
	return false
}

func skipSmallMetaindexRows(metaindex []metaindexRow, tsid *TSID) []metaindexRow {
	// Invariant: len(metaindex) > 0 && tsid >= metaindex[0].TSID.
	if tsid.Less(&metaindex[0].TSID) {
		logger.Panicf("BUG: invariant violation: tsid cannot be smaller than metaindex[0]; got %+v vs %+v", tsid, &metaindex[0].TSID)
	}

	if tsid.MetricID == metaindex[0].TSID.MetricID {
		return metaindex
	}

	// Invariant: tsid > metaindex[0].TSID, so sort.Search cannot return 0.
	n := sort.Search(len(metaindex), func(i int) bool {
		return !metaindex[i].TSID.Less(tsid)
	})
	if n == 0 {
		logger.Panicf("BUG: invariant violation: sort.Search returned 0 for tsid > metaindex[0].TSID; tsid=%+v; metaindex[0].TSID=%+v",
			tsid, &metaindex[0].TSID)
	}

	// The given tsid may be located in the previous metaindex row,
	// so go to the previous row.
	// Suppose the following metaindex rows exist [tsid10, tsid20, tsid30].
	// The following table contains the corresponding rows to start search for
	// for the given tsid values greater than tsid10:
	//
	//   * tsid11 -> tsid10
	//   * tsid20 -> tsid10, since tsid20 items may present in the index block [tsid10...tsid20]
	//   * tsid21 -> tsid20
	//   * tsid30 -> tsid20
	//   * tsid99 -> tsid30, since tsid99 items may be present in the index block [tsid30...tsidInf]
	return metaindex[n-1:]
}

func (ps *partSearch) readIndexBlock(mr *metaindexRow) (*indexBlock, error) {
	ps.compressedIndexBuf = bytesutil.Resize(ps.compressedIndexBuf[:0], int(mr.IndexBlockSize))
	ps.p.indexFile.MustReadAt(ps.compressedIndexBuf, int64(mr.IndexBlockOffset))

	var err error
	ps.indexBuf, err = encoding.DecompressZSTD(ps.indexBuf[:0], ps.compressedIndexBuf)
	if err != nil {
		return nil, fmt.Errorf("cannot decompress index block: %w", err)
	}
	ib := getIndexBlock()
	ib.bhs, err = unmarshalBlockHeaders(ib.bhs[:0], ps.indexBuf, int(mr.BlockHeadersCount))
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal index block: %w", err)
	}
	return ib, nil
}

func (ps *partSearch) searchBHS() bool {
	for i := range ps.bhs {
		bh := &ps.bhs[i]

	nextTSID:
		if bh.TSID.Less(&ps.BlockRef.bh.TSID) {
			// Skip blocks with small tsid values.
			continue
		}

		// Invariant: ps.BlockRef.bh.TSID <= bh.TSID

		if bh.TSID.MetricID != ps.BlockRef.bh.TSID.MetricID {
			// ps.BlockRef.bh.TSID < bh.TSID: no more blocks with the given tsid.
			// Proceed to the next (bigger) tsid.
			if !ps.nextTSID() {
				return false
			}
			goto nextTSID
		}

		// Found the block with the given tsid. Verify timestamp range.
		// While blocks for the same TSID are sorted by MinTimestamp,
		// the may contain overlapped time ranges.
		// So use linear search instead of binary search.
		if bh.MaxTimestamp < ps.tr.MinTimestamp {
			// Skip the block with too small timestamps.
			continue
		}
		if bh.MinTimestamp > ps.tr.MaxTimestamp {
			// Proceed to the next tsid, since the remaining blocks
			// for the current tsid contain too big timestamps.
			if !ps.nextTSID() {
				return false
			}
			continue
		}

		// Found the tsid block with the matching timestamp range.
		// Read it.
		ps.BlockRef.init(ps.p, bh)

		ps.bhs = ps.bhs[i+1:]
		return true
	}

	ps.bhs = nil
	return false
}
