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
	Snapshot     string
	TemporaryDir string
	Filter       Filter
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
	if minV == 0 {
		fmin = minV
	}
	if fmax == 0 {
		fmax = maxV
	}
	return minV <= fmax && fmin <= maxV
}

// BlockWithInfo wraps a BlockReader with resolution information.
type BlockWithInfo struct {
	Block      tsdb.BlockReader
	Resolution ResolutionLevel
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
func (c *Client) Explore(aggrType AggrType) ([]BlockWithInfo, error) {
	blockInfos, err := OpenBlocksWithInfo(c.snapshotPath, aggrType)
	if err != nil {
		return nil, fmt.Errorf("failed to open blocks: %w", err)
	}

	s := &Stats{
		Filtered: c.filter.min != 0 || c.filter.max != 0 || c.filter.label != "",
		Blocks:   len(blockInfos),
	}

	var blocksToImport []BlockWithInfo
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
			continue
		}

		s.Samples += meta.Stats.NumSamples
		s.Series += meta.Stats.NumSeries
		blocksToImport = append(blocksToImport, BlockWithInfo{
			Block:      bi.Block,
			Resolution: bi.Resolution,
		})
	}
	if !c.statsPrinted {
		fmt.Println(s)
		c.statsPrinted = true
	}
	return blocksToImport, nil
}

// Read reads the given BlockWithInfo according to configured time and label filters.
func (c *Client) Read(bi BlockWithInfo) (storage.SeriesSet, error) {
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
