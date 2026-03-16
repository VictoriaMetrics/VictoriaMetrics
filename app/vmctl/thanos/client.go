package thanos

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
)

// Config contains parameters for reading Thanos snapshots.
type Config struct {
	Snapshot string
	Filter   Filter
}

// Filter contains configuration for filtering the timeseries.
type Filter struct {
	TimeMin    string
	TimeMax    string
	Label      string
	LabelValue string
}

// Client reads Thanos snapshot blocks, including downsampled blocks with AggrChunk encoding.
type Client struct {
	snapshotPath string
	filter       filter
	statsPrinted bool
}

type filter struct {
	min, max   int64
	label      string
	labelValue string
}

func (f filter) inRange(minV, maxV int64) bool {
	fmin, fmax := f.min, f.max
	if fmin == 0 {
		fmin = minV
	}
	if fmax == 0 {
		fmax = maxV
	}
	return minV <= fmax && fmin <= maxV
}

// NewClient creates a new Thanos snapshot client.
func NewClient(cfg Config) (*Client, error) {
	minTime, maxTime, err := parseTime(cfg.Filter.TimeMin, cfg.Filter.TimeMax)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time in filter: %s", err)
	}
	return &Client{
		snapshotPath: cfg.Snapshot,
		filter: filter{
			min:        minTime,
			max:        maxTime,
			label:      cfg.Filter.Label,
			labelValue: cfg.Filter.LabelValue,
		},
	}, nil
}

// Explore fetches all available blocks from the snapshot with support for
// Thanos AggrChunk (downsampled blocks). It opens blocks with a custom pool
// that can decode AggrChunk encoding (0xff).
func (c *Client) Explore(aggrType AggrType) ([]BlockInfo, error) {
	blockInfos, err := OpenBlocksWithInfo(c.snapshotPath, aggrType)
	if err != nil {
		return nil, fmt.Errorf("failed to open blocks: %w", err)
	}

	s := &Stats{
		Filtered: c.filter.min != 0 || c.filter.max != 0 || c.filter.label != "",
		Blocks:   len(blockInfos),
	}

	var blocksToImport []BlockInfo
	for _, bi := range blockInfos {
		meta := bi.Block.Meta()

		if s.MinTime == 0 || meta.MinTime < s.MinTime {
			s.MinTime = meta.MinTime
		}
		if s.MaxTime == 0 || meta.MaxTime > s.MaxTime {
			s.MaxTime = meta.MaxTime
		}

		if !c.filter.inRange(meta.MinTime, meta.MaxTime) {
			s.SkippedBlocks++
			if bi.Closer != nil {
				_ = bi.Closer.Close()
			}
			continue
		}

		s.Samples += meta.Stats.NumSamples
		s.Series += meta.Stats.NumSeries
		blocksToImport = append(blocksToImport, bi)
	}
	if !c.statsPrinted {
		fmt.Println(s)
		c.statsPrinted = true
	}
	return blocksToImport, nil
}

// querierSeriesSet wraps a SeriesSet and its underlying Querier, ensuring
// the querier is closed once the SeriesSet has been fully consumed.
// This releases the querier's read reference on the block, which is required
// for Block.Close() to complete without hanging.
type querierSeriesSet struct {
	storage.SeriesSet
	q      storage.Querier
	closed bool
}

// Next advances the iterator. When the underlying SeriesSet is exhausted,
// it closes the querier to release resources.
func (s *querierSeriesSet) Next() bool {
	if s.SeriesSet.Next() {
		return true
	}
	if !s.closed {
		_ = s.q.Close()
		s.closed = true
	}
	return false
}

// Close explicitly closes the underlying querier.
// This must be called if iteration is stopped early (before Next returns false)
// to release block read references and prevent Block.Close() from hanging.
func (s *querierSeriesSet) Close() {
	if !s.closed {
		_ = s.q.Close()
		s.closed = true
	}
}

// ClosableSeriesSet extends storage.SeriesSet with a Close method for explicit cleanup.
type ClosableSeriesSet interface {
	storage.SeriesSet
	Close()
}

// Read reads the given BlockInfo according to configured time and label filters.
// The returned ClosableSeriesSet automatically closes the underlying querier when fully consumed,
// but Close() should be called explicitly (e.g., via defer) to handle early returns.
func (c *Client) Read(bi BlockInfo) (ClosableSeriesSet, error) {
	minTime, maxTime := bi.Block.Meta().MinTime, bi.Block.Meta().MaxTime
	if c.filter.min != 0 {
		minTime = c.filter.min
	}
	if c.filter.max != 0 {
		maxTime = c.filter.max
	}
	q, err := tsdb.NewBlockQuerier(bi.Block, minTime, maxTime)
	if err != nil {
		return nil, err
	}
	ss := q.Select(
		context.Background(),
		false,
		nil,
		labels.MustNewMatcher(labels.MatchRegexp, c.filter.label, c.filter.labelValue),
	)
	return &querierSeriesSet{
		SeriesSet: ss,
		q:         q,
	}, nil
}

func parseTime(start, end string) (int64, int64, error) {
	var s, e int64
	if start == "" && end == "" {
		return 0, 0, nil
	}
	if start != "" {
		v, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse %q: %s", start, err)
		}
		s = v.UnixNano() / int64(time.Millisecond)
	}
	if end != "" {
		v, err := time.Parse(time.RFC3339, end)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to parse %q: %s", end, err)
		}
		e = v.UnixNano() / int64(time.Millisecond)
	}
	return s, e, nil
}
