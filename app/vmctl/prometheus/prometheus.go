package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
)

// Config contains a list of params needed
// for reading Prometheus snapshots
type Config struct {
	// Path to snapshot directory
	Snapshot string

	Filter Filter
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
	filter filter
}

type filter struct {
	min, max   int64
	label      string
	labelValue string
}

func (f filter) inRange(min, max int64) bool {
	fmin, fmax := f.min, f.max
	if min == 0 {
		fmin = min
	}
	if fmax == 0 {
		fmax = max
	}
	return min <= fmax && fmin <= max
}

// NewClient creates and validates new Client
// with given Config
func NewClient(cfg Config) (*Client, error) {
	db, err := tsdb.OpenDBReadOnly(cfg.Snapshot, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot %q: %s", cfg.Snapshot, err)
	}
	c := &Client{DBReadOnly: db}
	min, max, err := parseTime(cfg.Filter.TimeMin, cfg.Filter.TimeMax)
	if err != nil {
		return nil, fmt.Errorf("failed to parse time in filter: %s", err)
	}
	c.filter = filter{
		min:        min,
		max:        max,
		label:      cfg.Filter.Label,
		labelValue: cfg.Filter.LabelValue,
	}
	return c, nil
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
