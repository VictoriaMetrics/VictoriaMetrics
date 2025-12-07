package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/thanos"
)

// Config contains a list of params needed
// for reading Prometheus snapshots
type Config struct {
	// Path to snapshot directory
	Snapshot string

	Filter       Filter
	TemporaryDir string
}

// Filter contains configuration for filtering
// the timeseries
type Filter struct {
	TimeMin    string
	TimeMax    string
	Label      string
	LabelValue string
}

// Client is a wrapper over Prometheus tsdb.DBReader
type Client struct {
	*tsdb.DBReadOnly
	filter       filter
	snapshotPath string
}

type filter struct {
	min, max   int64
	label      string
	labelValue string
}

func (f filter) inRange(minV, maxV int64) bool {
	fmin, fmax := f.min, f.max
	if minV == 0 {
		fmin = minV
	}
	if fmax == 0 {
		fmax = maxV
	}
	return minV <= fmax && fmin <= maxV
}

// NewClient creates and validates new Client
// with given Config
func NewClient(cfg Config) (*Client, error) {
	db, err := tsdb.OpenDBReadOnly(cfg.Snapshot, cfg.TemporaryDir, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot %q: %s", cfg.Snapshot, err)
	}
	c := &Client{
		DBReadOnly:   db,
		snapshotPath: cfg.Snapshot,
	}
	minTime, maxTime, err := parseTime(cfg.Filter.TimeMin, cfg.Filter.TimeMax)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time in filter: %s", err)
	}
	c.filter = filter{
		min:        minTime,
		max:        maxTime,
		label:      cfg.Filter.Label,
		labelValue: cfg.Filter.LabelValue,
	}
	return c, nil
}

// SnapshotPath returns the path to the snapshot directory.
func (c *Client) SnapshotPath() string {
	return c.snapshotPath
}

// Explore fetches all available blocks from a snapshot
// and collects the Meta() data from each block.
// Explore does initial filtering by time-range
// for snapshot blocks but does not take into account
// label filters.
func (c *Client) Explore() ([]tsdb.BlockReader, error) {
	blocks, err := c.Blocks()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blocks: %s", err)
	}
	s := &Stats{
		Filtered: c.filter.min != 0 || c.filter.max != 0 || c.filter.label != "",
		Blocks:   len(blocks),
	}
	var blocksToImport []tsdb.BlockReader
	for _, block := range blocks {
		meta := block.Meta()
		if !c.filter.inRange(meta.MinTime, meta.MaxTime) {
			s.SkippedBlocks++
			continue
		}
		if s.MinTime == 0 || meta.MinTime < s.MinTime {
			s.MinTime = meta.MinTime
		}
		if s.MaxTime == 0 || meta.MaxTime > s.MaxTime {
			s.MaxTime = meta.MaxTime
		}
		s.Samples += meta.Stats.NumSamples
		s.Series += meta.Stats.NumSeries
		blocksToImport = append(blocksToImport, block)
	}
	fmt.Println(s)
	return blocksToImport, nil
}

// Read reads the given BlockReader according to configured
// time and label filters.
func (c *Client) Read(block tsdb.BlockReader) (storage.SeriesSet, error) {
	minTime, maxTime := block.Meta().MinTime, block.Meta().MaxTime
	if c.filter.min != 0 {
		minTime = c.filter.min
	}
	if c.filter.max != 0 {
		maxTime = c.filter.max
	}
	q, err := tsdb.NewBlockQuerier(block, minTime, maxTime)
	if err != nil {
		return nil, err
	}
	ss := q.Select(context.Background(), false, nil, labels.MustNewMatcher(labels.MatchRegexp, c.filter.label, c.filter.labelValue))
	return ss, nil
}

// ReadWithAggrSupport reads the given BlockWithInfo according to configured
// time and label filters. It supports reading Thanos AggrChunk data from downsampled blocks.
// The aggrType parameter is passed for context but not directly used in querying since the
// aggregate type selection is handled by the custom chunk pool configured when the block was opened.
func (c *Client) ReadWithAggrSupport(bi BlockWithInfo, aggrType thanos.AggrType) (storage.SeriesSet, error) {
	_ = aggrType // Reserved for future use
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
	ss := q.Select(context.Background(), false, nil, labels.MustNewMatcher(labels.MatchRegexp, c.filter.label, c.filter.labelValue))
	return ss, nil
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

// BlockWithInfo wraps a BlockReader with resolution information.
type BlockWithInfo struct {
	Block      tsdb.BlockReader
	Resolution thanos.ResolutionLevel
}

// ExploreWithAggrSupport fetches all available blocks from a snapshot
// with support for Thanos AggrChunk (downsampled blocks).
// It opens blocks with custom pool that can decode AggrChunk encoding (0xff).
func (c *Client) ExploreWithAggrSupport(aggrType thanos.AggrType) ([]BlockWithInfo, error) {
	blockInfos, err := thanos.OpenBlocksWithInfo(c.snapshotPath, aggrType)
	if err != nil {
		return nil, fmt.Errorf("failed to open blocks with aggr support: %w", err)
	}

	s := &Stats{
		Filtered: c.filter.min != 0 || c.filter.max != 0 || c.filter.label != "",
		Blocks:   len(blockInfos),
	}

	var blocksToImport []BlockWithInfo
	for _, bi := range blockInfos {
		meta := bi.Block.Meta()

		// Update min/max time from all blocks for statistics
		if s.MinTime == 0 || meta.MinTime < s.MinTime {
			s.MinTime = meta.MinTime
		}
		if s.MaxTime == 0 || meta.MaxTime > s.MaxTime {
			s.MaxTime = meta.MaxTime
		}

		// Filter blocks by time range
		if !c.filter.inRange(meta.MinTime, meta.MaxTime) {
			s.SkippedBlocks++
			continue
		}

		// Count samples and series only for blocks that will be imported
		s.Samples += meta.Stats.NumSamples
		s.Series += meta.Stats.NumSeries
		blocksToImport = append(blocksToImport, BlockWithInfo{
			Block:      bi.Block,
			Resolution: bi.Resolution,
		})
	}
	fmt.Println(s)
	return blocksToImport, nil
}
